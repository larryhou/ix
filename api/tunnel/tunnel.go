package tunnel

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/ginuerzh/gost"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/quic-go/quic-go"
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
	Magic = `CDTunnel`
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
		conn:   conn,
		endian: binary.BigEndian,
		mtu:    mtu,
	}

	s.contex, s.cancel = context.WithCancel(ctx)
	return s, s.handshake()
}

type Service struct {
	*Descriptor
	RSD *rsd.Service

	conn   Conn
	endian binary.ByteOrder
	mtu    int
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
		x.endian.PutUint16(buf.Bytes()[k:], uint16(buf.Len()-k-2))
		_, err = io.Copy(x.conn, buf)
	}

	return err
}

func (x *Service) recv(msg any) error {
	rsv := make([]byte, len(Magic))
	_, err := x.conn.Read(rsv)
	if err == nil {
		_, err = x.conn.Read(rsv[:2])
	}

	buf := &bytes.Buffer{}
	if err == nil {
		num := x.endian.Uint16(rsv)
		_, err = io.Copy(buf, io.LimitReader(x.conn, int64(num)))
	}

	if err == nil {
		err = json.NewDecoder(buf).Decode(msg)
	}

	return err
}

func (x *Service) handshake() error {
	err := x.send(map[string]any{
		`type`: `clientHandshakeRequest`,
		`mtu`:  x.mtu,
	})

	rsp := &Descriptor{}
	if err == nil {
		err = x.recv(rsp)
	}

	if err == nil {
		x.Descriptor = rsp
	}

	return err
}

func (x *Service) Start(conn any) error {
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
	if err != nil {return err}
	defer tun.Close()

	ifce := *(**water.Interface)(unsafe.Pointer(reflect.ValueOf(tun).Pointer()))
	addr := &net.TCPAddr{
		IP:   net.ParseIP(x.ServerAddress),
		Port: x.ServerRSDPort,
		Zone: ifce.Name(),
	}

	go func() {
		rs, err := rsd.NewFromTunnel(addr)
		if err == nil {
			log.Printf(`TUNNEL RSD %s`, addr)
			x.RSD = rs
		}
	}()

	log.Printf(`TUNNEL STARTED %s`, addr)

	switch conn := conn.(type) {
	case quic.Connection:
		err = x.startQuicTunnel(tun, conn)
	case net.Conn:
		err = x.startTcpTunnel(tun, conn)
	default:
		return errors.New(`BAD CONN INSTANCE`)
	}

	return err
}

func (x *Service) startQuicTunnel(tun net.Conn, conn quic.Connection) (err error) {
	defer conn.CloseWithError(0, `CLOSE`)
	go func() error {
		err := error(nil)
		mtu := make([]byte, x.ClientParameters.Mtu)
		for err == nil {
			_, err = tun.Read(mtu)
			if err == nil {
				num := binary.BigEndian.Uint16(mtu[4:])
				err = conn.SendDatagram(mtu[:num+40])
				if err != nil {
					if err, ok := err.(*quic.DatagramTooLargeError); ok {
						log.Printf(`QUIC SEND #%d > %d`, num+40, err.MaxDatagramPayloadSize)
					}
				}
			}
		}

		return err
	}()

	for mtu := []byte(nil); err == nil; {
		mtu, err = conn.ReceiveDatagram(x.contex)
		if err == nil {
			_, err = tun.Write(mtu)
		}
	}

	return err
}

func (x *Service) startTcpTunnel(tun, conn net.Conn) (err error) {
	defer conn.Close()
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
				_, err = io.Copy(conn, bytes.NewReader(mtu[:num+40]))
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

		_, err = io.ReadFull(conn, mtu[:40])
		num := binary.BigEndian.Uint16(mtu[4:])
		if err == nil {
			_, err = io.ReadFull(conn, mtu[40:40+num])
		}

		if err == nil {
			_, err = tun.Write(mtu)
		}
	}

	return
}

func (x *Service) Stop() {
	if x.cancel != nil {
		x.cancel()
		x.cancel = nil
	}
}
