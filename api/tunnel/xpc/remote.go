package xpc

import (
	"bytes"
	"github.com/larryhou/j3idevice/api/tunnel/h2c"
	"io"
	"net"
)


func NewRemoteXpc(addr *net.TCPAddr) (*RemoteXpcConnection, error) {
	conn, err := net.Dial(`tcp`, addr.String())
	if err != nil {return nil, err}

	c, err := h2c.NewClient(conn)
	if err != nil {return nil, err}

	r := &RemoteXpcConnection{
		TCPAddr: addr,
		conn: &h2Conn{
			Connection: c,
		},
	}

	return r, r.connect()
}

type h2Conn struct {
	*h2c.Connection
}

func (x *h2Conn) OpenStream(discard bool) (Stream, error) {
	return x.Connection.NewStream(discard)
}

func (x *h2Conn) Close() error {
	return x.Connection.Close()
}

type Stream interface {
	io.Reader
	io.Writer
}

type Connection interface {
	OpenStream(discard bool) (Stream, error)
	io.Closer
}

type RemoteXpcConnection struct {
	*net.TCPAddr
	conn Connection
	main Stream
	assi Stream
	sn   int64
}

func (x *RemoteXpcConnection) connect() error {
	return x.handshake()
}

func (x *RemoteXpcConnection) handshake() (err error) {
	buf := &bytes.Buffer{}

	x.main, err = x.conn.OpenStream(false)
	if err != nil {return err}

	Encode(buf, &Message{Payload: &Payload{Data: map[string]any{}}})
	_, err = x.main.Write(buf.Bytes())

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: 0x0201})
		_, err = x.main.Write(buf.Bytes())
	}

	if err == nil {
		x.assi, err = x.conn.OpenStream(true)
		if err != nil {return err}
	}

	buf.Reset()
	Encode(buf, &Message{Flag: FlagInitHandshake})
	_, err = x.assi.Write(buf.Bytes())

	for i := 0; i < 2 && err == nil; i++ {
		_, err = x.Recv()
	}

	return err
}

func (x *RemoteXpcConnection) Send(msg any) error {
	buf := &bytes.Buffer{}
	err := Encode(buf, &Message{
		Id:      x.sn,
		Flag:    FlagDataPresent,
		Payload: &Payload{Data: msg},
	})

	if err == nil {
		_, err = x.main.Write(buf.Bytes())
	}

	return err
}

func (x *RemoteXpcConnection) Recv() (any, error) {
	msg := &Message{}
	err := Decode(x.main, msg)
	if err == nil {
		x.sn = msg.Id + 1
		if msg.Payload != nil {return msg.Data, nil}
		return nil, nil
	}
	return nil, err
}

func (x *RemoteXpcConnection) Close() error {
	return x.conn.Close()
}