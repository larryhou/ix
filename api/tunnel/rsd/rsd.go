package rsd

import (
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"net"
)

const (
	Port = 58783
)

type Service struct {
	net.Conn
	*Handshake
}

func (x *Service) Connect() error {
	addr, err := bonjour.TCPAddr(bonjour.RemotedServiceName)
	addr.Port = Port

	conn, err := net.Dial(`tcp`, addr.String())
	if err != nil { return err }

	x.Conn = conn

	return nil
}

func (x *Service) Read(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		k, err := x.Conn.Read(b[t:])
		if err != nil {return 0, err}
		t += k
	}
	return n, nil
}

func (x *Service) Write(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		k, err := x.Conn.Write(b[t:])
		if err != nil {return 0, err}
		t += k
	}
	return n, nil
}