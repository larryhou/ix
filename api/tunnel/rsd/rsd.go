package rsd

import (
	"encoding/binary"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"github.com/shirou/gopsutil/process"
	"log"
	"net"
	"strconv"
	"syscall"
)

const (
	Port = 58783
)

const (
	FusionService                 = `com.apple.fusion.remote.service`
	GpuToolsAgent                 = `com.apple.gputools.remote.agent`
	TunnelService                 = `com.apple.internal.dt.coredevice.untrusted.tunnelservice`
	InsecureNotificationProxy     = `com.apple.mobile.insecure_notification_proxy.remote`
	InsecureNotificationProxyShim = `com.apple.mobile.insecure_notification_proxy.shim.remote`
	UntrustedLockdown             = `com.apple.mobile.lockdown.remote.untrusted`
	LogTransfer                   = `com.apple.osanalytics.logTransfer`
)

func New() (*Service, error) {
	s := &Service{}
	return s, s.connect()
}

type Service struct {
	*Handshake

	tcpAddr *net.TCPAddr
}

func (x *Service) connect() error {
	addr, err := bonjour.TCPAddr(bonjour.RemotedServiceName)
	if err != nil {return err}
	x.tcpAddr = addr

	return Hijack(func() error {
		addr := *x.tcpAddr
		addr.Port = Port
		r, err := xpc.NewRemoteXpc(&addr)
		if err != nil {return err}
		defer r.Close()

		msg, err := r.Recv()
		if err == nil {
			err = x.handshake(msg)
		}

		return err
	})
}

func (x *Service) handshake(msg any) (err error) {
	if data, ok := msg.(map[string]any); ok {
		if data[`MessageType`] == `Handshake` {
			hs := &Handshake{}
			err = mapstructure.Decode(data, hs)
			if err == nil { x.Handshake = hs }
		} else {
			err = fmt.Errorf(`expect handshake: %+v`, msg)
		}
	}

	return
}

func (x *Service) StartLockdownService() (*lockdown.Service, error) {
	addr, err := x.getServiceAddr(UntrustedLockdown, false)
	if err != nil {return nil, err}

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

func (x *Service) getServiceAddr(name string, useXpc bool) (*net.TCPAddr, error) {
	s, ok := x.Services[name]
	if !ok {return nil, fmt.Errorf(`invalid service: %s`, name)}
	if s.Properties.UsesRemoteXPC != useXpc {
		return nil, fmt.Errorf(`%s UsesRemoteXPC=%v`, name, s.Properties.UsesRemoteXPC)
	}

	port, err := strconv.Atoi(s.Port)
	if err != nil {return nil, err}

	addr := *x.tcpAddr
	addr.Port = port
	return &addr, nil
}

func (x *Service) StartRemoteService(name string) (*xpc.RemoteXpcConnection, error) {
	addr, err := x.getServiceAddr(name, true)
	if err != nil {return nil, err}
	return xpc.NewRemoteXpc(addr)
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
		err = syscall.Kill(pid, syscall.SIGSTOP)
		log.Printf(`HIJACK STOP %d %v`, pid, err)
		defer func(err error) {
			if err == nil {
				err = syscall.Kill(pid, syscall.SIGCONT)
				log.Printf(`HIJACK CONT %d %v`, pid, err)
			}
		}(err)
		err = f()
	} else {
		err = f()
	}

	return err
}