package rsd

import (
	"errors"
	"fmt"
	"github.com/grandcat/zeroconf"
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"golang.org/x/net/http2"
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
	var ent *zeroconf.ServiceEntry
	err := bonjour.Browse(bonjour.RemotedServiceName, func(v *zeroconf.ServiceEntry) bool {
		ent = v
		return false
	})

	if err != nil {return err}

	var address *net.IP
	if len(ent.AddrIPv6) > 0 {
		address = &ent.AddrIPv6[0]
	}

	if len(ent.AddrIPv4) > 0 && address == nil{
		address = &ent.AddrIPv4[0]
	}

	if address == nil {
		return errors.New(`no bonjour device`)
	}

	conn, err := net.Dial(`tcp`, fmt.Sprintf(`%s:%d`, address, Port))
	if err != nil { return err }

	x.Conn = conn

	t2 := http2.Transport{

	}



	t2.AllowHTTP = true

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