package xpc

import (
	"bytes"
	"errors"
	"github.com/larryhou/gomobiledevice3/api/tunnel/h2c"
	"io"
	"log"
	"net"
)

var (
	DONE = errors.New(`ExitRunloop`)
)

type Handle func(msg *Message) error

type multiHandle []Handle

func (x multiHandle) Handle(msg *Message) error {
	for _, h := range x {
		err := h(msg)
		if err != nil {return err}
	}

	return nil
}

func NewRemoteXpc(addr *net.TCPAddr, h Handle) (*RemoteXpcConnection, error) {
	r := &RemoteXpcConnection{
		TCPAddr: addr,
	}
	return r, r.connect(h)
}

type RemoteXpcConnection struct {
	*net.TCPAddr
	*h2c.Connection
	Main *h2c.Stream
	Assi *h2c.Stream

	h chan struct{}
	n int64
}

func (x *RemoteXpcConnection) connect(h Handle) error {
	conn, err := net.Dial(`tcp`, x.TCPAddr.String())
	if err != nil { return err }
	log.Printf(`RemoteXpc %s => %s`, conn.LocalAddr(), conn.RemoteAddr())
	return x.handshake(conn, h)
}

func (x *RemoteXpcConnection) handshake(conn net.Conn, h Handle) error {
	hc, err := h2c.NewClient(conn)
	if err != nil {return err}
	x.Connection = hc

	r, w := io.Pipe()
	buf := &bytes.Buffer{}

	x.Main, err = hc.NewStream(w)
	if err != nil {return err}
	if err == nil {
		Encode(buf, &Message{Payload: &Payload{Data: map[string]any{}}})
		err = x.Main.Send(buf, int64(buf.Len()))
	}

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: 0x0201})
		err = x.Main.Send(buf, int64(buf.Len()))
	}

	x.Assi, err = hc.NewStream(w)
	if err != nil {return err}

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: FlagInitHandshake})
		err = x.Assi.Send(buf, int64(buf.Len()))
	}

	x.h = make(chan struct{})
	go func() {
		defer x.Connection.Close()
		defer w.Close()

		if err := x.runloop(r, h); err != nil {
			log.Printf(`RemoteXpc RUNLOOP %v`, err)
		}
	}()

	<-x.h
	log.Printf(`HANDSHAKE DONE`)
	return nil
}

func (x *RemoteXpcConnection) SendRequest(msg any) error {
	x.n++
	buf := &bytes.Buffer{}
	err := Encode(buf, &Message{
		Id:      x.n,
		Flag:    FlagDataPresent,
		Payload: &Payload{Data: msg},
	})

	if err == nil {
		err = x.Main.Send(buf, int64(buf.Len()))
	}

	return err
}

func (x *RemoteXpcConnection) runloop(r io.Reader, h Handle) (err error) {
	for err == nil {
		msg := &Message{}
		err = Decode(r, msg)
		if err == nil {
			if h != nil {
				if err = h(msg); err == DONE {
					err = nil
					return
				}
			}

			if msg.Flag & FlagInitHandshake != 0 {
				close(x.h)
			}
		}
	}

	return
}