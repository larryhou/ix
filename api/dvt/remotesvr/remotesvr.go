package remotesvr

import (
	"bytes"
	"errors"
	"github.com/larryhou/iconsole/ns"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
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

type DTXChannel struct {
	Id int32
	ch chan []byte
	cm sync.Mutex

	svc     *Service
	pending bytes.Buffer
}

func (x *DTXChannel) Send(selector string, args *ArgumentAux, reply bool) error {
	return x.svc.Send(x.Id, selector, args, reply)
}

func (x *DTXChannel) Recv(aux **ArgumentAux) (any, error) {
	rsp, err := x.svc.RecvObject(x.Id, aux)
	if err == nil {
		if nserr, ok := rsp.(error); ok {
			return nil, nserr
		}
	}

	return rsp, err
}

func (x *DTXChannel) RecvBytes(aux **ArgumentAux) ([]byte, error) {
	return x.svc.Recv(x.Id, aux)
}

func (x *DTXChannel) Bytes() <-chan []byte {
	x.cm.Lock()
	defer x.cm.Unlock()
	return x.ch
}

func (x *DTXChannel) flush() {
	x.cm.Lock()
	ch := x.ch
	x.cm.Unlock()

	if ch == nil || x.pending.Len() == 0 {return}
	data := make([]byte, x.pending.Len())
	copy(data, x.pending.Bytes())
	x.pending.Reset()
	ch <- data
}

func (x *DTXChannel) Close() error {
	x.cm.Lock()
	defer x.cm.Unlock()
	if x.ch != nil {
		close(x.ch)
		x.ch = nil
	}

	return nil
}

func New(r *rsd.Service) (*Service, error) {
	return NewByName(r, rsd.ComAppleInstrumentsDtservicehub)
}

func NewByName(r *rsd.Service, name string) (*Service, error) {
	conn, err := r.StartService(name)
	if err != nil {return nil, err}

	s := &Service{
		Conn: conn,
	}

	return s, s.connect()
}

type Service struct {
	net.Conn

	ch map[int32]*DTXChannel
	cm sync.RWMutex

	sn uint32
	cn int32
}

func (x *Service) connect() error {
	x.ch = make(map[int32]*DTXChannel)
	go x.runloop()

	return x.handshake()
}

func (x *Service) handshake() error {
	args := &ArgumentAux{}
	args.Obj(map[string]any{
		`com.apple.private.DTXBlockCompression`: 0,
		`com.apple.private.DTXConnection`:       1,
	})

	sel := `_notifyOfPublishedCapabilities:`
	err := x.Send(BroadcastChannel, sel, args, false)
	if err != nil {return err}

	var aux *ArgumentAux
	rsp, err := x.RecvObject(BroadcastChannel, &aux)
	if err != nil {return err}

	if rsp != sel {
		return errors.New(`bad handshake`)
	}

	if len(aux.Values) == 0 {
		return errors.New(`bad handshake len(aux)==0`)
	}

	//log.Printf(`HANDSHAKE %+v %+v`, aux.Values[0], rsp)
	return nil
}

func (x *Service) OpenChannel(identifier string) (int32, error) {
	x.cn++
	args := new(ArgumentAux).U32(*(*uint32)(unsafe.Pointer(&x.cn))).Obj(identifier)
	err := x.Send(BroadcastChannel, `_requestChannelWithCode:identifier:`, args, true)
	if err != nil {return 0, err}

	var aux *ArgumentAux
	rsp, err := x.RecvObject(BroadcastChannel, &aux)
	if rsp != nil {return 0, errors.New(`CREATE CHANNEL FAIL`)}
	return x.cn, err
}

func (x *Service) GetChannel(id int32) *DTXChannel {
	x.cm.RLock()
	ch, ok := x.ch[id]
	x.cm.RUnlock()
	if !ok {
		ch = &DTXChannel{
			Id:  id,
			ch:  make(chan []byte, 1),
			svc: x,
		}
		x.cm.Lock()
		x.ch[id] = ch
		x.cm.Unlock()
	}

	return ch
}

func (x *Service) runloop() (err error) {
	defer x.Close()
	buf := make([]byte, unsafe.Sizeof(DTXMessageHeader{}))
	hdr := (*DTXMessageHeader)(unsafe.Pointer(&buf[0]))
	for err == nil {
		_, err = io.ReadFull(x.Conn, buf)
		if err != nil {continue}
		id := hdr.ChannelCode
		ch := x.GetChannel(id)

		if hdr.SessionIndex == 0 {
			if hdr.Identifier > x.sn {
				x.sn = hdr.Identifier
			}
		}
		if hdr.FragmentCount > 1 && hdr.FragmentId == 0 {
			continue
		}

		if hdr.Length > 0 {
			_, err = io.Copy(&ch.pending, io.LimitReader(x.Conn, int64(hdr.Length)))
		}

		if hdr.FragmentCount == hdr.FragmentId + 1 {
			if ch.pending.Len() > 0 { ch.flush() }
		}
	}

	return
}

func (x *Service) Recv(channel int32, aux **ArgumentAux) ([]byte, error) {
	ch := x.GetChannel(channel)
	if ch.Bytes() == nil {return nil, errors.New(`channel closed`)}
	buf := <-ch.Bytes()
	hdr := (*DTXPayloadHeader)(unsafe.Pointer(&buf[0]))
	if hdr.Flags & 0xFF000 > 0 {
		return nil, errors.New(`compressed`)
	}

	buf = buf[HeaderSizePayload:]
	if hdr.AuxiliaryLength > 0 {
		aUx := &ArgumentAux{}
		err := aUx.Decode(buf[:hdr.AuxiliaryLength])
		if err != nil {return nil, err}
		if aux != nil {
			*aux = aUx
		}
	}

	obj := buf[hdr.AuxiliaryLength:]
	return obj, nil
}

func (x *Service) RecvObject(channel int32, aux **ArgumentAux) (any, error) {
	data, err := x.Recv(channel, aux)
	if err != nil {return nil, err}
	if len(data) != 0 {
		nka := ns.NewNSKeyedArchiver()
		return nka.Unmarshal(data)
	}

	return nil, nil
}

const (
	HeaderSizeMessage = 0x20
	HeaderSizePayload = 0x10
)

func (x *Service) Send(channel int32, selector string, args *ArgumentAux, reply bool) error {
	akn := ns.NewNSKeyedArchiver()
	sel, err := akn.Marshal(selector)
	if err != nil {return err}

	aux, err := args.Encode()
	if err != nil {return err}

	payHeader := &DTXPayloadHeader{
		Flags:           flagInstrumentsMessageType,
		AuxiliaryLength: uint32(len(aux)),
		TotalLength:     uint64(len(aux) + len(sel)),
	}

	if reply {
		payHeader.Flags |= flagExpectsReplyMask
	}

	x.sn++
	msgHeader := &DTXMessageHeader{
		Magic:         magicDTX,
		Cb:            HeaderSizeMessage,
		FragmentId:    0,
		FragmentCount: 1,
		Length:        uint32(HeaderSizePayload + payHeader.TotalLength),
		Identifier:    x.sn,
		SessionIndex:  0,
		ChannelCode:   channel,
		ExpectReply:   uint32(*(*byte)(unsafe.Pointer(&reply))),
	}

	buf := &bytes.Buffer{}
	buf.Write(unsafe.Slice((*byte)(unsafe.Pointer(msgHeader)), HeaderSizeMessage))
	buf.Write(unsafe.Slice((*byte)(unsafe.Pointer(payHeader)), HeaderSizePayload))
	buf.Write(aux)
	buf.Write(sel)
	_, err = io.Copy(x.Conn, buf)
	return err
}

func (x *Service) Close() error {
	args := new(ArgumentAux)
	for _, ch := range x.ch {
		if ch.Id > 0 {
			args.U32(*(*uint32)(unsafe.Pointer(&ch.Id)))
		}
	}

	x.Send(BroadcastChannel, `_channelCanceled:`, args, false)

	x.cm.Lock()
	defer x.cm.Unlock()
	for _, ch := range x.ch { ch.Close() }
	return x.Conn.Close()
}
