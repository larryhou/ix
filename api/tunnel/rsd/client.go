package rsd

import (
	"bufio"
	"errors"
	"golang.org/x/net/http2"
	"io"
	"log"
	"math"
	"net"
	"sync"
)

type flow struct {
	n int32

	conn *flow
}

func (f *flow) setConnFlow(cf *flow) { f.conn = cf }

func (f *flow) available() int32 {
	n := f.n
	if f.conn != nil && f.conn.n < n {
		n = f.conn.n
	}
	return n
}

func (f *flow) take(n int32) {
	if n > f.available() {
		panic("internal error: took too much")
	}
	f.n -= n
	if f.conn != nil {
		f.conn.n -= n
	}
}

func (f *flow) add(n int32) bool {
	sum := f.n + n
	if (sum > n) == (f.n > 0) {
		f.n = sum
		return true
	}
	return false
}

type Stream struct {
	ID   uint32
	Recv func(b []byte, ended bool) error

	cc *Client
	fl flow
	bf [1 << 14]byte
}

func (x *Stream) abort(err error) {
	x.cc.endStream(x)
}

func (x *Stream) control(n int) (int, error) {
	x.cc.mu.Lock()
	defer x.cc.mu.Unlock()
	for {
		if x.cc == nil {
			return 0, errors.New(`stream closed`)
		}

		if a := x.fl.available(); a > 0 {
			take := min(n, int(a), int(x.cc.maxFrameSize))
			x.fl.take(int32(take))
			return take, nil
		}

		x.cc.cd.Wait()
	}
}

func (x *Stream) Send(r io.Reader, n int64) error {
	for n > 0 {
		b := min(math.MaxInt32, n)
		p, err := x.control(int(b))
		if err != nil {return err}

		for k := 0; k < p; {
			m, err := r.Read(x.bf[k:])
			if err != nil {return err}
			k += m
		}

		x.cc.wm.Lock()
		err = x.cc.fr.WriteData(x.ID, n == int64(p), x.bf[:p])
		if err == nil {
			err = x.cc.wb.Flush()
		}

		x.cc.wm.Unlock()

		if err != nil {return err}
		n -= int64(p)
	}

	return nil
}

func NewClient(c net.Conn) (*Client, error) {
	cc := &Client{
		nc:                   c,
		nextStreamID:         1,
		maxFrameSize:         16 << 10,
		initialWindowSize:    65535,
		maxConcurrentStreams: 100,
		streams:              map[uint32]*Stream{},
	}

	cc.cd = sync.NewCond(&cc.mu)
	cc.fl.add(int32(cc.initialWindowSize))

	cc.wb = bufio.NewWriter(c)
	cc.rb = bufio.NewReader(c)
	cc.fr = http2.NewFramer(cc.wb, cc.rb)

	settings := []http2.Setting{
		{ID: http2.SettingEnablePush, Val: 0},
		{ID: http2.SettingInitialWindowSize, Val: 1 << 20},
		{ID: http2.SettingMaxConcurrentStreams, Val: cc.maxConcurrentStreams},
	}

	cc.wb.Write([]byte(http2.ClientPreface))
	cc.wb.Flush()
	cc.fr.WriteSettings(settings...)
	cc.fr.WriteWindowUpdate(0, (1 << 20) - cc.initialWindowSize)
	err := cc.wb.Flush()
	go func() {
		defer cc.nc.Close()
		err := cc.runloop()
		if ce, ok := err.(http2.ConnectionError); ok {
			cc.wm.Lock()
			cc.fr.WriteGoAway(0, http2.ErrCode(ce), nil)
			cc.wm.Unlock()
		}
	}()

	return cc, err
}

type Client struct {
	Notify func(cs *Stream)

	nc net.Conn
	fr *http2.Framer
	rb *bufio.Reader
	wb *bufio.Writer
	wm sync.Mutex
	mu sync.Mutex
	cd *sync.Cond
	fl flow

	nextStreamID         uint32
	maxFrameSize         uint32
	maxConcurrentStreams uint32
	initialWindowSize    uint32

	streams map[uint32]*Stream
}

func (x *Client) runloop() error {
	for x.fr != nil {
		f, err := x.fr.ReadFrame()
		if err != nil {
			return err
		}

		switch f := f.(type) {
		case *http2.MetaHeadersFrame:
			err = x.processHeaders(f)
		case *http2.DataFrame:
			err = x.processData(f)
		case *http2.GoAwayFrame:
			err = x.processGoAway(f)
		case *http2.RSTStreamFrame:
			err = x.processResetStream(f)
		case *http2.SettingsFrame:
			err = x.processSettings(f)
		case *http2.WindowUpdateFrame:
			err = x.processWindowUpdate(f)
		default:
			log.Printf("Transport: unhandled response frame type %#v", f)
		}

		if err != nil {return err}
	}

	return nil
}

func (x *Client) NewStream() (*Stream, error) {
	x.mu.Lock()
	defer x.mu.Unlock()

	cs := &Stream{
		ID: x.nextStreamID,
		cc: x,
	}

	cs.fl.setConnFlow(&x.fl)
	cs.fl.add(int32(x.initialWindowSize))

	x.nextStreamID += 2
	x.streams[cs.ID] = cs

	defer x.wb.Flush()
	return cs, x.fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:   cs.ID,
		EndHeaders: true,
	})
}

func (x *Client) processSettings(f *http2.SettingsFrame) error {
	x.wm.Lock()
	defer x.wm.Unlock()

	f.ForeachSetting(func(setting http2.Setting) error {
		switch setting.ID {
		case http2.SettingMaxFrameSize:
			x.maxFrameSize = setting.Val
		case http2.SettingMaxConcurrentStreams:
			x.maxConcurrentStreams = setting.Val
		case http2.SettingMaxHeaderListSize:
		case http2.SettingInitialWindowSize:
			if setting.Val < math.MaxInt32 {
				num := int32(setting.Val) - int32(x.initialWindowSize)
				for _, cs := range x.streams { cs.fl.add(num) }
				x.initialWindowSize = setting.Val
				x.cd.Broadcast()
			}
		}
		return nil
	})

	if !f.IsAck() {
		x.fr.WriteSettingsAck()
		return x.wb.Flush()
	}

	return nil
}

func (x *Client) endStream(cs *Stream) {
	delete(x.streams, cs.ID)
	cs.cc = nil
}

func (x *Client) streamByID(id uint32) *Stream {
	if len(x.streams) == 0 {return nil}
	return x.streams[id]
}

func (x *Client) processHeaders(f *http2.MetaHeadersFrame) error {
	x.mu.Lock()
	defer x.mu.Unlock()

	cs := &Stream{
		ID: f.StreamID,
		cc: x,
	}

	cs.fl.setConnFlow(&x.fl)
	cs.fl.add(int32(x.initialWindowSize))
	x.streams[cs.ID] = cs
	if x.Notify != nil {
		x.Notify(cs)
	}
	return nil
}

func (x *Client) processData(f *http2.DataFrame) error {
	data := f.Data()

	x.wm.Lock()
	x.fr.WriteWindowUpdate(0, f.Length)
	x.fr.WriteWindowUpdate(f.StreamID, f.Length)
	x.wb.Flush()
	x.wm.Unlock()

	cs := x.streamByID(f.StreamID)
	if cs != nil {
		if cs.Recv != nil {
			err := cs.Recv(data, f.StreamEnded())
			if err != nil {return err}
		}

		if f.StreamEnded() {
			x.endStream(cs)
		}
	}

	return nil
}

func (x *Client) processGoAway(_ *http2.GoAwayFrame) error {
	return x.Close()
}

func (x *Client) processResetStream(_ *http2.RSTStreamFrame) error {
	return x.Close()
}

func (x *Client) processWindowUpdate(f *http2.WindowUpdateFrame) error {
	cs := x.streamByID(f.StreamID)
	if cs == nil && f.StreamID != 0 {
		return nil
	}

	x.mu.Lock()
	defer x.mu.Unlock()

	fl := &x.fl
	if cs != nil {
		fl = &cs.fl
	}

	if !fl.add(int32(f.Increment)) {
		return http2.ConnectionError(http2.ErrCodeFlowControl)
	}

	x.cd.Broadcast()
	return nil
}

func (x *Client) Close() error {
	x.streams = nil
	x.fr = nil

	return x.nc.Close()
}