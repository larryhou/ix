package tunnel

import (
	"bytes"
	"encoding/json"
	"github.com/ginuerzh/gost"
	"github.com/larryhou/j3idevice/api/base"
	"io"
	"log"
	"net"
	"strconv"
)

const (
	Mtu = 16000
)

const (
	ServiceName = `com.apple.internal.devicecompute.CoreDeviceProxy`
	Magic       = `CDTunnel`
)

const (
	TypeClientHandshakeRequest = `clientHandshakeRequest`
)

type Descriptor struct {
	ServerRSDPort    int    `json:"serverRSDPort"`
	ServerAddress    string `json:"serverAddress"`
	Type             string `json:"type"`
	ClientParameters struct {
		Mtu     int    `json:"mtu"`
		Address string `json:"address"`
		Netmask string `json:"netmask"`
	} `json:"clientParameters"`
}

func New(service *base.Service) (*Service, error) {
	s := &Service{Service: service}
	err := s.handshake()
	if err == nil {
		log.Printf(`TUNNEL %+v`, s.Descriptor)
		err = s.start()
	}

	return s, err
}

type Service struct {
	*base.Service
	*Descriptor
}

func (x *Service) Send(msg any) error {
	num := make([]byte, 2)
	buf := &bytes.Buffer{}
	buf.WriteString(Magic)
	buf.Write(num)
	err := json.NewEncoder(buf).Encode(msg)
	if err == nil {
		k := len(Magic)
		x.ByteOrder.PutUint16(buf.Bytes()[k:], uint16(buf.Len()-k-2))
		_, err = io.Copy(x.Conn, buf)
	}

	return err
}

func (x *Service) Recv(msg any) error {
	rsv := make([]byte, len(Magic))
	_, err := x.Read(rsv)
	if err == nil {
		_, err = x.Read(rsv[:2])
	}

	buf := &bytes.Buffer{}
	if err == nil {
		num := x.ByteOrder.Uint16(rsv)
		_, err = io.Copy(buf, io.LimitReader(x.Conn, int64(num)))
	}

	if err == nil {
		err = json.NewDecoder(buf).Decode(msg)
	}

	return err
}

func (x *Service) handshake() error {
	err := x.Send(map[string]any{
		`type`: TypeClientHandshakeRequest,
		`mtu`:  Mtu,
	})

	rsp := &Descriptor{}
	if err == nil {
		err = x.Recv(rsp)
	}

	if err == nil {
		x.Descriptor = rsp
	}

	return err
}

func (x *Service) start() error {
	n := 0
	z:for _, c := range net.ParseIP(x.Descriptor.ClientParameters.Netmask) {
		for j, k := 0, byte(7); j < 8; j,k = j+1,k-1 {
			if c&(1<<k) == 0 { break z }
			n++
		}
	}

	gost.SetLogger(&gost.LogLogger{})
	ln, err := gost.TunListener(gost.TunConfig{
		Addr: x.Descriptor.ClientParameters.Address + `/` + strconv.Itoa(n),
		MTU:  x.Descriptor.ClientParameters.Mtu,
		Peer: x.Descriptor.ServerAddress,
	})

	if err == nil {
		log.Printf(`TUNNEL STARTED [%s]:%d`, x.ServerAddress, x.ServerRSDPort)
		nc, err := ln.Accept()
		if err != nil {
			return err
		}

		go io.Copy(x.Conn, nc)
	}

	return err
}

