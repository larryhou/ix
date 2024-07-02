package rsd

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

//goland:noinspection GoNameStartsWithPackageName
const (
	RsdPort = 58783
	SvrPort = 33333
)

var (
	BadServiceName = errors.New(`BAD SERVICE NAME`)
)

func BrowseRSD() (*Service, error) {
	addr, err := bonjour.TCPAddr(bonjour.RemotedServiceName)
	if err != nil {return nil, err}
	return New(addr)
}

func Return[T any](v *T, err error) (*T, error) {
	if err != nil {return nil, err}
	return v, nil
}

func New(addr *net.TCPAddr) (*Service, error) {
	addr.Port = RsdPort
	svc := &Service{TCPAddr: addr}
	return Return(svc, Hijack(svc.connect))
}

func NewFromTunnel(addr *net.TCPAddr) (*Service, error) {
	s := &Service{TCPAddr: addr}
	return Return(s, s.connect())
}

type Service struct {
	*Descriptor
	*net.TCPAddr
}

func (x *Service) connect() error {
	r, err := xpc.NewRemoteXpc(x.TCPAddr)
	if err != nil {return err}
	defer r.Close()

	msg, err := r.Recv()
	if err == nil {
		err = x.handshake(msg)
	}

	return err
}

func (x *Service) handshake(msg any) (err error) {
	if data, ok := msg.(map[string]any); ok {
		if data[`MessageType`] == `Handshake` {
			hs := &Descriptor{}
			err = mapstructure.Decode(data, hs)
			if err == nil { x.Descriptor = hs }
		} else {
			err = fmt.Errorf(`expect handshake: %+v`, msg)
		}
	}

	return
}

func (x *Service) LockdownService() (*lockdown.Service, error) {
	addr, err := x.GetServiceAddr(ComAppleMobileLockdownRemoteTrusted, false)
	if err != nil {
		if err == BadServiceName {
			addr, err = x.GetServiceAddr(ComAppleMobileLockdownRemoteUntrusted, false)
		}

		if err != nil {return nil, err}
	}

	con := &base.Connection{ByteOrder: binary.BigEndian}
	err = con.Connect(addr.String())

	svc := &base.Service{
		Connection: con,
		ByteOrder:  binary.BigEndian,
	}

	rsp := make(map[string]any)
	err = svc.Get(map[string]any{
		`Label`:           base.ProgramName,
		`ProtocolVersion`: `2`,
		`Request`:         `RSDCheckin`,
	}, &rsp)

	if err == nil {
		if rsp[`Request`] != `RSDCheckin` {
			return nil, fmt.Errorf(`unexpected: %+v`, rsp)
		}
	}

	rsp = make(map[string]any)
	err = svc.Recv(&rsp)
	if err == nil {
		if rsp[`Request`] != `StartService` {
			return nil, fmt.Errorf(`unexpected: %+v`, rsp)
		}
	}

	lds := &lockdown.Service{Service: svc}
	_, err = lds.GetDescriptor()
	return lds, err
}

func (x *Service) GetServiceAddr(name string, useXpc bool) (*net.TCPAddr, error) {
	s, ok := x.Services[name]
	if !ok {return nil, BadServiceName
	}
	if s.Properties.UsesRemoteXPC != useXpc {
		return nil, fmt.Errorf(`%s UsesRemoteXPC=%v`, name, s.Properties.UsesRemoteXPC)
	}

	port, err := strconv.Atoi(s.Port)
	if err != nil {return nil, err}

	addr := *x.TCPAddr
	addr.Port = port
	return &addr, nil
}

func (x *Service) StartService(name string) (*xpc.RemoteXpcConnection, error) {
	addr, err := x.GetServiceAddr(name, true)
	if err != nil {return nil, err}
	return xpc.NewRemoteXpc(addr)
}

var (
	guard sync.Mutex
)

func Hijack(f func()error) error {
	guard.Lock()
	defer guard.Unlock()

	if runtime.GOOS != `darwin` {
		return f()
	}

	buf := &bytes.Buffer{}
	cmd := exec.Command(`ps`, `-ax`, `-opid,comm`)
	cmd.Stdout = buf
	cmd.Run()

	pid := 0
	for k := bufio.NewScanner(buf); k.Scan(); {
		if proc := k.Text(); strings.HasSuffix(proc, `/usr/libexec/remoted`) {
			if i := strings.IndexByte(proc, ' '); i > 0 {
				pid, _ = strconv.Atoi(proc[:i])
				break
			}
		}
	}

	if pid == 0 {
		return f()
	}

	err := syscall.Kill(pid, syscall.SIGSTOP)
	defer func(err error) {
		if err == nil {
			syscall.Kill(pid, syscall.SIGCONT)
		}
	}(err)
	return f()
}