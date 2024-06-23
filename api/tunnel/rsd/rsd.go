package rsd

import (
	"bytes"
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"github.com/larryhou/gomobiledevice3/api/tunnel/h2c"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"github.com/shirou/gopsutil/process"
	"log"
	"net"
	"syscall"
)

const (
	Port = 58783
)

func New() (*Service, error) {
	s := &Service{}
	return s, s.connect()
}

type Service struct {
	*Handshake
}

func (x *Service) connect() error {
	addr, err := bonjour.TCPAddr(bonjour.RemotedServiceName)
	if err != nil {return err}
	addr.Port = Port

	conn, err := net.Dial(`tcp`, addr.String())
	if err != nil { return err }
	recv := make(chan []byte)
	defer close(recv)

	return Hijack(func() error {
		err := x.handshake(conn, recv)
		if err == nil {
			for b := range recv {
				if x.monitor(b) {
					err = conn.Close()
					break
				}
			}
		}
		return err
	})
}

func (x *Service) monitor(b []byte) bool {
	msg := &xpc.Message{}
	err := xpc.Decode(bytes.NewReader(b), msg)
	if err == nil && msg.Payload != nil {
		if data, ok := msg.Data.(map[string]any); ok {
			if data[`MessageType`] == `Handshake` {
				hs := &Handshake{}
				err = mapstructure.Decode(data, hs)
				if err == nil {
					x.Handshake = hs
					return true
				}

				log.Printf(`REMOTED HANDSHAKE: %v`, err)
			}
		}

		log.Printf(`REMOTED RECV: %+v %v`, msg, msg.Data)
	}

	return false
}

func (x *Service) handshake(conn net.Conn, recv chan<-[]byte) error {
	hc, err := h2c.NewConnection(conn)
	if err != nil {return err}

	buf := &bytes.Buffer{}

	s1, err := hc.NewStream()
	if err != nil {return err}
	s1.Recv = recv
	if err == nil {
		xpc.Encode(buf, &xpc.Message{
			Payload: &xpc.Payload{
				Data: map[string]any{},
			},
		})

		err = s1.Send(buf, int64(buf.Len()))
	}

	if err == nil {
		buf.Reset()
		xpc.Encode(buf, &xpc.Message{
			Flag: 0x0201,
		})

		err = s1.Send(buf, int64(buf.Len()))
	}

	s3, err := hc.NewStream()
	if err != nil {return err}
	s3.Recv = recv

	if err == nil {
		buf.Reset()
		xpc.Encode(buf, &xpc.Message{
			Flag: xpc.FlagInitHandshake,
		})

		err = s3.Send(buf, int64(buf.Len()))
	}

	return err
}

func Hijack(f func()error) error {
	pid := -1
	processes, err := process.Processes()
	for _, proc := range processes {
		name, _ := proc.Exe()
		if name == `/usr/libexec/remoted` {
			pid = int(proc.Pid)
			break
		}
	}

	if pid > 0 {
		syscall.Kill(pid, syscall.SIGSTOP)
		log.Printf(`HIJACK STOP %d`, pid)
		defer func() {
			syscall.Kill(pid, syscall.SIGCONT)
			log.Printf(`HIJACK CONT %d`, pid)
		}()
		err = f()
	} else {
		err = f()
	}

	return err
}