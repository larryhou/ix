package remote

import (
	"bytes"
	"errors"
	"github.com/larryhou/iconsole/ns"
	"io"
	"net"
	"sync"
	"unsafe"
)

const (
	flagInstrumentsMessageType = 2
	flagExpectsReplyMask       = 0x1000
)

const (
	BroadcastChannel = 0
)

type dxtChannel struct {
	code    int32
	data    chan []byte
	pending bytes.Buffer
}

func (x *dxtChannel) Write(b []byte) (int, error) {
	n, err := x.pending.Write(b)
	return n, err
}

func (x *dxtChannel) Bytes() <-chan []byte{
	return x.data
}

func (x *dxtChannel) Flush() {
	data := make([]byte, x.pending.Len())
	copy(data, x.pending.Bytes())
	x.pending.Reset()
	x.data <- data
}

func New(conn net.Conn) (*Service, error) {
	s := &Service{
		Conn: conn,
	}

	return s, s.connect()
}

type Service struct {
	net.Conn

	ch map[int32]*dxtChannel
	cm sync.RWMutex

	sn uint32
}

func (x *Service) connect() error {
	x.ch = make(map[int32]*dxtChannel)
	panic(``)
}

func (x *Service) handshake() error {
	aux := &MessageAux{}
	aux.AddObj(map[string]any{
		`com.apple.private.DTXBlockCompression`: 0,
		`com.apple.private.DTXConnection`: 1,
	})

	err := x.send(BroadcastChannel, `_notifyOfPublishedCapabilities:`, aux, false)
	if err != nil {return err}
	go x.runloop()
	return nil
}

func (x *Service) getChannel(code int32) *dxtChannel {
	x.cm.RLock()
	ch, ok := x.ch[code]
	x.cm.RUnlock()
	if !ok {
		ch = &dxtChannel{
			code: code,
			data: make(chan []byte, 1),
		}
		x.cm.Lock()
		x.ch[code] = ch
		x.cm.Unlock()
	}

	return ch
}

func (x *Service) runloop() (err error) {
	buf := make([]byte, unsafe.Sizeof(DXTMessageHeader{}))
	hdr := (*DXTMessageHeader)(unsafe.Pointer(&buf[0]))
	for err == nil {
		_, err = io.ReadFull(x.Conn, buf)
		code := hdr.ChannelCode
		ch := x.getChannel(code)

		if hdr.SessionIndex == 0 {
			if hdr.Identifier > x.sn {
				x.sn = hdr.Identifier
			}
		}

		if hdr.FragmentCount > 1 && hdr.FragmentId == 0 {
			continue
		}

		_, err = io.Copy(ch, io.LimitReader(x.Conn, int64(hdr.Length)))
		if hdr.FragmentCount == hdr.FragmentId + 1 {
			ch.Flush()
		}
	}

	return
}

func (x *Service) recv(channel int32) (*MessageAux, []byte, error) {
	ch := x.getChannel(channel)
	buf := <-ch.Bytes()
	hdr := (*DXTPayloadHeader)(unsafe.Pointer(&buf[0]))
	if hdr.Flags & 0xFF000 > 0 {
		return nil, nil, errors.New(`compressed`)
	}

	var aux *MessageAux
	if hdr.AuxiliaryLength > 0 {
		panic(``)
	}

	obj := buf[hdr.AuxiliaryLength:]
	return aux, obj, nil
}

func (x *Service) recvObject(channel int32) (any, error) {
	_, data, err := x.recv(channel)
	if err != nil {return nil, err}
	nka := ns.NewNSKeyedArchiver()
	return nka.Unmarshal(data)
}

func (x *Service) send(channel int32, selector string, args *MessageAux, reply bool) error {
	akn := ns.NewNSKeyedArchiver()
	sel, err := akn.Marshal(selector)
	if err != nil {return err}

	aux, err := args.Bytes()
	if err != nil {return err}

	payHeader := &DXTPayloadHeader{
		Flags:           flagInstrumentsMessageType,
		AuxiliaryLength: uint32(len(aux)),
		TotalLength:     uint64(len(aux) + len(sel)),
	}

	if reply {
		payHeader.Flags |= flagExpectsReplyMask
	}

	x.sn++
	msgHeader := &DXTMessageHeader{
		Magic:         magicDXT,
		FragmentId:    0,
		FragmentCount: 1,
		Identifier:    x.sn,
		SessionIndex:  0,
		ChannelCode:   channel,
		ExpectReply:   uint32(*(*byte)(unsafe.Pointer(&reply))),
	}

	msgHeader.Cb = uint32(unsafe.Sizeof(msgHeader))
	msgHeader.Length = uint32(unsafe.Sizeof(payHeader)) + uint32(payHeader.TotalLength)

	buf := &bytes.Buffer{}
	buf.Write(unsafe.Slice((*byte)(unsafe.Pointer(msgHeader)), unsafe.Sizeof(msgHeader)))
	buf.Write(unsafe.Slice((*byte)(unsafe.Pointer(payHeader)), unsafe.Sizeof(payHeader)))
	buf.Write(aux)
	buf.Write(sel)
	_, err = io.Copy(x.Conn, buf)
	return err
}
