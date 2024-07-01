package xpc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/larryhou/j3idevice/api/tunnel/h2c"
	"io"
	"net"
)

type Network string

const (
	NetworkTCP  Network = `tcp`
)

type Context struct {
	Network
	*net.TCPAddr
}

func (x *Context) String() string {
	return fmt.Sprintf(`%s://%s`, x.Network, x.TCPAddr)
}

func NewRemoteXpc(ctx *Context) (*RemoteXpcConnection, error) {
	var xpcConn Connection

	switch ctx.Network {
	case NetworkTCP,``:
		conn, err := net.Dial(`tcp`, ctx.TCPAddr.String())
		if err != nil {return nil, err}

		c, err := h2c.NewClient(conn)
		if err != nil {return nil, err}
		xpcConn = &h2Conn{
			Connection: c,
		}

	default:
		return nil, errors.New(`BAD NETWORK: ` + string(ctx.Network))
	}

	r := &RemoteXpcConnection{
		Context: ctx,
		conn:    xpcConn,
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
	*Context
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
	if err == nil {
		Encode(buf, &Message{Payload: &Payload{Data: map[string]any{}}})
		_, err = x.main.Write(buf.Bytes())
	}

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: 0x0201})
		_, err = x.main.Write(buf.Bytes())
	}

	x.assi, err = x.conn.OpenStream(true)
	if err != nil {return err}

	if err == nil {
		buf.Reset()
		Encode(buf, &Message{Flag: FlagInitHandshake})
		_, err = x.assi.Write(buf.Bytes())
	}

	_, err = x.Recv()
	if err == nil {
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