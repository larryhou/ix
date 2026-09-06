package plist

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/larryhou/ix/api/mux"
	"github.com/larryhou/ix/api/mux/usb"
	"howett.net/plist"
	"io"
	"net"
)

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		ByteOrder: binary.BigEndian,
		Connection: &mux.Connection{
			Conn: conn,
		},
	}
}

type Connection struct {
	*mux.Connection
	binary.ByteOrder
}

func (x *Connection) Send(msg any) error {
	buf := &bytes.Buffer{}
	buf.Write([]byte{1, 2, 3, 4})
	err := plist.NewEncoder(buf).Encode(msg)
	if err == nil {
		x.ByteOrder.PutUint32(buf.Bytes(), uint32(buf.Len()-4))
		_, err = io.Copy(x.Conn, buf)
	}

	return err
}

func (x *Connection) RecvRaw() ([]byte, error) {
	num := make([]byte, 4)
	if _, err := io.ReadFull(x.Conn, num); err != nil {
		return nil, err
	}
	buf := make([]byte, x.ByteOrder.Uint32(num))
	if _, err := io.ReadFull(x.Conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (x *Connection) Recv(msg any) error {
	num := make([]byte, 4)
	if _, err := io.ReadFull(x.Conn, num); err != nil {
		return err
	}

	buf := make([]byte, x.ByteOrder.Uint32(num))
	if _, err := io.ReadFull(x.Conn, buf); err != nil {
		return err
	}

	err := plist.NewDecoder(bytes.NewReader(buf)).Decode(msg)
	if err == nil {
		if r, ok := msg.(usb.Retcode); ok {
			err = r.Verify()
		} else {
			switch msg := msg.(type) {
			case *map[string]any:
				if errStr, ok := (*msg)[`Error`]; ok {
					err = errors.New(errStr.(string))
				}
			}
		}
	}
	return err
}

func (x *Connection) Get(req, rsp any) error {
	if err := x.Send(req); err == nil {
		return x.Recv(rsp)
	} else {
		return err
	}
}
