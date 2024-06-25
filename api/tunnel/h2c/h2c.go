package h2c

import (
	"bytes"
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
	ID uint32

	w io.Writer
	r io.Reader

	c *Connection
	f flow
	b [1 << 14]byte
}

func (x *Stream) abort() {
	x.c.endStream(x)
}

func (x *Stream) control(n int) (int, error) {
	x.c.mu.Lock()
	defer x.c.mu.Unlock()
	for {
		if x.c == nil {
			return 0, errors.New(`stream closed`)
		}

		if a := x.f.available(); a > 0 {
			take := min(n, int(a), int(x.c.maxFrameSize))
			x.f.take(int32(take))
			return take, nil
		}

		x.c.cd.Wait()
	}
}

func (x *Stream) Write(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		m := min(math.MaxInt32, n-t)
		p, err := x.control(m)
		if err != nil {return 0, err}

		x.c.wm.Lock()
		err = x.c.fr.WriteData(x.ID, false, b[t:t+p])
		x.c.wm.Unlock()
		t += p
	}

	return n, nil
}

func (x *Stream) Read(b []byte) (int, error) {
	return x.r.Read(b)
}

func (x *Stream) Send(r io.Reader) error {
	for {
		n := 0
		for ; n < len(x.b); {
			m, err := r.Read(x.b[n:])
			n += m
			if err != nil {
				if err == io.EOF { break }
				return err
			}
		}
		_, err := x.Write(x.b[:n])
		if err != nil || n < len(x.b) {return err}
	}
}

func (x *Stream) Recv(w io.Writer) error {
	_, err := io.Copy(w, x.r)
	return err
}

func (x *Stream) Close() error {
	if w, ok := x.w.(io.Closer); ok {
		return w.Close()
	}

	return nil
}

func NewClient(c net.Conn) (*Connection, error) {
	cc := &Connection{
		nc:                   c,
		nextStreamID:         1,
		maxFrameSize:         16 << 10,
		initialWindowSize:    65535,
		maxConcurrentStreams: 100,
		streams:              map[uint32]*Stream{},
		done:                 make(chan struct{}),
	}

	cc.cd = sync.NewCond(&cc.mu)
	cc.fl.add(int32(cc.initialWindowSize))

	cc.fr = http2.NewFramer(cc.nc, cc.nc)

	settings := []http2.Setting{
		{ID: http2.SettingMaxConcurrentStreams, Val: cc.maxConcurrentStreams},
		{ID: http2.SettingInitialWindowSize, Val: 1 << 20},
	}

	cc.wm.Lock()
	cc.nc.Write([]byte(http2.ClientPreface))
	cc.fr.WriteSettings(settings...)
	err := cc.fr.WriteWindowUpdate(0, (1 << 20) - cc.initialWindowSize)
	cc.wm.Unlock()

	go func() {
		if err := cc.runloop(); err != nil {
			cc.Close()
		}
	}()

	return cc, err
}

type Connection struct {
	nc net.Conn
	fr *http2.Framer
	wm sync.Mutex
	mu sync.RWMutex
	cd *sync.Cond
	fl flow

	nextStreamID         uint32
	maxFrameSize         uint32
	maxConcurrentStreams uint32
	initialWindowSize    uint32

	streams map[uint32]*Stream
	done    chan struct{}
}

func (x *Connection) runloop() error {
	for x.fr != nil {
		f, err := x.fr.ReadFrame()
		if err != nil {
			return err
		}

		switch f := f.(type) {
		case *http2.HeadersFrame:
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
			log.Printf("H2C: unhandled frame %#v", f)
		}

		if err != nil {return err}
	}

	return nil
}

func (x *Connection) NewStream(discard bool) (*Stream, error) {
	x.mu.Lock()
	defer x.mu.Unlock()

	cs := &Stream{
		ID: x.nextStreamID,
		c:  x,
	}

	if discard {
		cs.w = io.Discard
	} else {
		cs.r, cs.w = io.Pipe()
	}

	cs.f.setConnFlow(&x.fl)
	cs.f.add(int32(x.initialWindowSize))

	x.nextStreamID += 2
	x.streams[cs.ID] = cs

	return cs, x.fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:   cs.ID,
		EndHeaders: true,
	})
}

func (x *Connection) processSettings(f *http2.SettingsFrame) error {
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
				for _, cs := range x.streams { cs.f.add(num) }
				x.initialWindowSize = setting.Val
				x.cd.Broadcast()
			}
		}
		return nil
	})

	if !f.IsAck() {
		x.fr.WriteSettingsAck()
	}

	return nil
}

func (x *Connection) endStream(cs *Stream) {
	delete(x.streams, cs.ID)
	cs.c = nil
}

func (x *Connection) streamByID(id uint32) *Stream {
	if len(x.streams) == 0 {return nil}
	return x.streams[id]
}

func (x *Connection) processData(f *http2.DataFrame) (err error) {
	data := f.Data()

	//x.wm.Lock()
	//x.fr.WriteWindowUpdate(0, f.Length)
	//x.fr.WriteWindowUpdate(f.StreamID, f.Length)
	//x.wm.Unlock()

	cs := x.streamByID(f.StreamID)
	if cs != nil {
		_, err = io.Copy(cs.w, bytes.NewReader(data))
		if f.StreamEnded() {
			x.endStream(cs)
		}
	}

	return
}

func (x *Connection) processGoAway(f *http2.GoAwayFrame) error {
	log.Printf(`GOAWAY %s`, f.ErrCode)
	return http2.ConnectionError(f.ErrCode)
}

func (x *Connection) processResetStream(f *http2.RSTStreamFrame) error {
	log.Printf(`RESET %s`, f.ErrCode)
	return http2.ConnectionError(f.ErrCode)
}

func (x *Connection) processWindowUpdate(f *http2.WindowUpdateFrame) error {
	cs := x.streamByID(f.StreamID)
	if cs == nil && f.StreamID != 0 {
		return nil
	}

	x.mu.Lock()
	defer x.mu.Unlock()

	fl := &x.fl
	if cs != nil {
		fl = &cs.f
	}

	if !fl.add(int32(f.Increment)) {
		return http2.ConnectionError(http2.ErrCodeFlowControl)
	}

	x.cd.Broadcast()
	return nil
}

func (x *Connection) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()

	for _, cs := range x.streams {
		if cs.w != nil {
			cs.Close()
			cs.w = nil
		}
	}

	if x.fr != nil {
		close(x.done)
	}

	x.streams = nil
	x.fr = nil

	return x.nc.Close()
}

func (x *Connection) Done() <-chan struct{} {
	return x.done
}