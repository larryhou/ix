package xpc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/larryhou/j3idevice/api/tunnel/h2c"
	"github.com/quic-go/quic-go"
	"io"
	"math/big"
	"net"
)

type Network string

const (
	NetworkTCP  Network = `tcp`
	NetworkQUIC Network = `quic`
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

	case NetworkQUIC:
		key, err := rsa.GenerateKey(rand.Reader, 1024)
		if err != nil {return nil, err}
		tpl := x509.Certificate{SerialNumber: big.NewInt(1)}
		crt, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &key.PublicKey, key)
		if err != nil {return nil, err}

		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crt})

		tlsCert, err := tls.X509KeyPair(crtPEM, keyPEM)
		if err != nil {return nil, err}

		conn, err := quic.DialAddr(context.Background(), ctx.TCPAddr.String(), &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}, nil)

		if err != nil {return nil, err}
		xpcConn = &quicConn{
			Connection: conn,
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

type quicConn struct {
	quic.Connection
}

func (x *quicConn) OpenStream(discard bool) (Stream, error) {
	stream, err := x.Connection.OpenStream()
	if discard {
		go io.Copy(io.Discard, stream)
	}
	return stream, err
}

func (x *quicConn) Close() error {
	return x.Connection.CloseWithError(0, `CLOSE`)
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