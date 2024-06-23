package rsd

import (
	"bytes"
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"github.com/larryhou/gomobiledevice3/api/tunnel/h2c"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"github.com/shirou/gopsutil/process"
	"io"
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

	return hijack(func() error {
		conn, err := net.Dial(`tcp`, addr.String())
		if err != nil { return err }
		log.Printf(`REMOTED %s => %s`, conn.LocalAddr(), conn.RemoteAddr())
		return x.handshake(conn)
	})
}

func (x *Service) monitor(r io.Reader) (bool, error) {
	msg := &xpc.Message{}
	err := xpc.Decode(r, msg)
	if err == nil && msg.Payload != nil {
		if data, ok := msg.Data.(map[string]any); ok {
			if data[`MessageType`] == `Handshake` {
				hs := &Handshake{}
				err = mapstructure.Decode(data, hs)
				if err == nil {
					x.Handshake = hs
					return true, nil
				}

				log.Printf(`REMOTED HANDSHAKE %v`, err)
			}
		}

		log.Printf(`REMOTED RECV %+v %v`, msg, msg.Data)
	}

	return false, err
}

func (x *Service) handshake(conn net.Conn) error {
	r, w := io.Pipe()
	defer w.Close()

	hc, err := h2c.NewClient(conn)
	if err != nil {return err}

	buf := &bytes.Buffer{}

	s1, err := hc.NewStream(w)
	if err != nil {return err}
	if err == nil {
		xpc.Encode(buf, &xpc.Message{Payload: &xpc.Payload{Data: map[string]any{}}})
		err = s1.Send(buf, int64(buf.Len()))
	}

	if err == nil {
		buf.Reset()
		xpc.Encode(buf, &xpc.Message{Flag: 0x0201})
		err = s1.Send(buf, int64(buf.Len()))
	}

	s3, err := hc.NewStream(w)
	if err != nil {return err}

	if err == nil {
		buf.Reset()
		xpc.Encode(buf, &xpc.Message{Flag: xpc.FlagInitHandshake})
		err = s3.Send(buf, int64(buf.Len()))
	}

	for f := false; err == nil && !f; f, err = x.monitor(r) {}
	return err
}

func hijack(f func()error) error {
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
		err = syscall.Kill(pid, syscall.SIGSTOP)
		log.Printf(`HIJACK STOP %d %v`, pid, err)
		defer func() {
			if err == nil {
				err = syscall.Kill(pid, syscall.SIGCONT)
				log.Printf(`HIJACK CONT %d %v`, pid, err)
			}
		}()
		err = f()
	} else {
		err = f()
	}

	return err
}