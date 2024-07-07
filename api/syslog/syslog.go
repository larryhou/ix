package syslog

import (
	"bufio"
	"io"
	"net"
)

const (
	ServiceName = `com.apple.syslog_relay`
)

func New(conn net.Conn) *Service {
	s := &Service{Conn: conn}
	return s
}

type Service struct {
	net.Conn
}

func (x *Service) Streaming(w io.Writer) error {
	s := bufio.NewScanner(x.Conn)
	for s.Scan() {
		w.Write(s.Bytes()[1:])
		w.Write([]byte{'\n'})
	}

	return s.Err()
}