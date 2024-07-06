package base

import (
	"errors"
	"log"
	"net"
)

type Connection struct {
	net.Conn
}

func (x *Connection) Spawn() (*Connection, error) {
	if x.Conn == nil {
		return nil, errors.New(`BAD CONNECTION`)
	}

	addr := x.Conn.RemoteAddr()
	conn, err := net.Dial(addr.Network(), addr.String())
	if err != nil {return nil, err}
	log.Printf(`CONNECT %s => %s`, conn.LocalAddr(), conn.RemoteAddr())

	return &Connection{
		Conn: conn,
	}, nil
}
