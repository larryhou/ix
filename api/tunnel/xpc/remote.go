package xpc

import (
	"bytes"
	"github.com/larryhou/j3idevice/api/tunnel/h2c"
	"log"
	"net"
)

func NewRemoteXpc(addr *net.TCPAddr) (*RemoteXpcConnection, error) {
	r := &RemoteXpcConnection{
		TCPAddr: addr,
	}
	return r, r.connect()
}

type RemoteXpcConnection struct {
	*net.TCPAddr
	*h2c.Connection
	Main *h2c.Stream
	Assi *h2c.Stream

	n int64
}

func (x *RemoteXpcConnection) connect() error {
	conn, err := net.Dial(`tcp`, x.TCPAddr.String())
	if err != nil { return err }
	log.Printf(`RemoteXpc %s => %s`, conn.LocalAddr(), conn.RemoteAddr())
	return x.handshake(conn)
}

func (x *RemoteXpcConnection) handshake(conn net.Conn) error {
	hc, err := h2c.NewClient(conn)
	if err != nil {return err}
	x.Connection = hc

	buf := &bytes.Buffer{}

	x.Main, err = hc.NewStream(false)
	if err != nil {return err}
	if err == nil {
		Encode(buf, &Message{Payload: &Payload{Data: map[string]any{}}})
		err = x.Main.Send(buf)
	}

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: 0x0201})
		err = x.Main.Send(buf)
	}

	x.Assi, err = hc.NewStream(true)
	if err != nil {return err}

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: FlagInitHandshake})
		err = x.Assi.Send(buf)
	}

	_, err = x.Recv()
	if err == nil {
		_, err = x.Recv()
	}

	return err
}

func (x *RemoteXpcConnection) Send(msg any) error {
	x.n++
	buf := &bytes.Buffer{}
	err := Encode(buf, &Message{
		Id:      x.n,
		Flag:    FlagDataPresent,
		Payload: &Payload{Data: msg},
	})

	if err == nil {
		err = x.Main.Send(buf)
	}

	return err
}

func (x *RemoteXpcConnection) Recv() (any, error) {
	msg := &Message{}
	err := Decode(x.Main, msg)
	if err == nil {
		if msg.Payload != nil {return msg.Data, nil}
		return nil, nil
	}
	return nil, err
}