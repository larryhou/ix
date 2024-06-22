package rsd

import (
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"github.com/shirou/gopsutil/process"
	"log"
	"net"
	"syscall"
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