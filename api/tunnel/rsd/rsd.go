package rsd

import (
	"context"
	"errors"
	"fmt"
	"github.com/grandcat/zeroconf"
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
	ent, err := x.bonjour()
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

//func (x *Service) handshake() error {
//
//}

func (x *Service) bonjour() (*zeroconf.ServiceEntry, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil { return nil, err }

	result := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go resolver.Browse(ctx, "_remoted._tcp", "local.", result)
	for entry := range result {
		return entry, nil
	}

	return nil, nil
}