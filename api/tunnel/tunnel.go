package tunnel

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/ginuerzh/gost"
	"github.com/songgao/water"
	"io"
	"log"
	"net"
	"reflect"
	"strconv"
	"unsafe"
)

const (
	MtuTcp = 16000
	MtuUdp = 1420
)

const (
	CoreDeviceProxyName = `com.apple.internal.devicecompute.CoreDeviceProxy`
	Magic               = `CDTunnel`
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

type Conn interface {
	io.Reader
	io.Writer
	io.Closer
}

func New(conn Conn, mtu int, ctx context.Context) (*Service, error) {
	s := &Service{
		Conn:      conn,
		ByteOrder: binary.BigEndian,
		mtu:       mtu,
	}

	s.contex, s.cancel = context.WithCancel(ctx)

	err := s.handshake()
	if err == nil {
		log.Printf(`TUNNEL %+v`, s.Descriptor)
		err = s.start()
	}

	return s, err
}

type Service struct {
	Conn
	binary.ByteOrder
	*Descriptor

	mtu    int
	ifce   *water.Interface
	contex context.Context
	cancel func()
}

func (x *Service) send(msg any) error {
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

func (x *Service) recv(msg any) error {
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
	err := x.send(map[string]any{
		`type`: TypeClientHandshakeRequest,
		`mtu`:  x.mtu,
	})

	rsp := &Descriptor{}
	if err == nil {
		err = x.recv(rsp)
	}

	if err == nil {
		x.Descriptor = rsp
		log.Printf(`%+v`, rsp)
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

	listener, err := gost.TunListener(gost.TunConfig{
		Addr: x.Descriptor.ClientParameters.Address + `/` + strconv.Itoa(n),
		MTU:  x.Descriptor.ClientParameters.Mtu,
		Peer: x.Descriptor.ServerAddress,
	})
	if err != nil {return err}
	tun, err := listener.Accept()
	if err != nil {
		return err
	}

	x.ifce = *(**water.Interface)(unsafe.Pointer(reflect.ValueOf(tun).Pointer()))
	log.Printf(`TUNNEL STARTED [%s%%%s]:%d`, x.ServerAddress, x.ifce.Name(), x.ServerRSDPort)

	go func() error {
		err := error(nil)
		mtu := make([]byte, x.ClientParameters.Mtu)
		for err == nil {
			select {
			case <-x.contex.Done(): return x.contex.Err()
			default:
			}

			_, err = tun.Read(mtu)
			if err == nil {
				num := binary.BigEndian.Uint16(mtu[4:])
				_, err = io.Copy(x.Conn, bytes.NewReader(mtu[:num+40]))
			}
		}

		return err
	}()

	mtu := make([]byte, x.ClientParameters.Mtu)
	for err == nil {
		select {
		case <-x.contex.Done():
			return x.contex.Err()
		default:
		}

		_, err = io.ReadFull(x.Conn, mtu[:40])
		num := binary.BigEndian.Uint16(mtu[4:])
		if err == nil {
			_, err = io.ReadFull(x.Conn, mtu[40:40+num])
		}

		if err == nil {
			_, err = tun.Write(mtu)
		}
	}

	return err
}

func (x *Service) Stop() {
	if x.Conn != nil {
		x.Conn.Close()
		x.Conn = nil
	}

	if x.ifce != nil {
		x.ifce.Close()
		x.ifce = nil
	}

	if x.cancel != nil {
		x.cancel()
		x.cancel = nil
	}
}

