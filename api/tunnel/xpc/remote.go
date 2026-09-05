package xpc

import (
	"bytes"
	"github.com/larryhou/j3idevice/api/tunnel/h2c"
	"io"
	"net"
	"sync/atomic"
)

func NewRemoteXpc(addr *net.TCPAddr) (*RemoteXpcConnection, error) {
	conn, err := net.Dial(`tcp`, addr.String())
	if err != nil {
		return nil, err
	}

	c, err := h2c.NewClient(conn)
	if err != nil {
		return nil, err
	}

	r := &RemoteXpcConnection{
		TCPAddr: addr,
		conn:    &h2Conn{Connection: c},
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
	sn   atomic.Int64 // sequence number — safe for concurrent Send/Recv
}

func (x *RemoteXpcConnection) connect() error {
	return x.handshake()
}

func (x *RemoteXpcConnection) handshake() (err error) {
	x.main, err = x.conn.OpenStream(false)
	if err != nil {
		return err
	}

	// First handshake message: empty dict payload.
	buf := &bytes.Buffer{}
	if err = Encode(buf, &Message{Payload: &Payload{Data: map[string]any{}}}); err != nil {
		return err
	}
	if _, err = x.main.Write(buf.Bytes()); err != nil {
		return err
	}

	// Second handshake message: init flags, no payload.
	buf.Reset()
	if err = Encode(buf, &Message{Flag: 0x0201}); err != nil {
		return err
	}
	if _, err = x.main.Write(buf.Bytes()); err != nil {
		return err
	}

	x.assi, err = x.conn.OpenStream(true)
	if err != nil {
		return err
	}

	// Third handshake message on assistant stream.
	buf.Reset()
	if err = Encode(buf, &Message{Flag: FlagInitHandshake}); err != nil {
		return err
	}
	if _, err = x.assi.Write(buf.Bytes()); err != nil {
		return err
	}

	// Consume two reply messages.
	for i := 0; i < 2 && err == nil; i++ {
		_, err = x.Recv()
	}

	return err
}

func (x *RemoteXpcConnection) Send(msg any) error {
	buf := &bytes.Buffer{}
	err := Encode(buf, &Message{
		Id:      x.sn.Load(),
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
	if err := Decode(x.main, msg); err != nil {
		return nil, err
	}
	x.sn.Store(msg.Id + 1)
	if msg.Payload != nil {
		return msg.Data, nil
	}
	return nil, nil
}

func (x *RemoteXpcConnection) Close() error {
	return x.conn.Close()
}
