package rsd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/base/plist"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"github.com/larryhou/j3idevice/api/util"
	"github.com/mitchellh/mapstructure"
	"log"
	"net"
	"net/http"
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
	Missing = errors.New(`SERVICE MISSING`)
)


func BrowseRSD() (*Service, error) {
	addr, err := bonjour.TCPAddr(bonjour.RemotePairingServiceName)
	if err != nil {return nil, err}
	return New(addr)
}

func New(addr *net.TCPAddr) (*Service, error) {
	addr.Port = RsdPort
	svc := &Service{TCPAddr: addr}
	return util.Return(svc, hijack(svc.connect))
}

func NewFromTunnelD(udid string) (*Service, error) {
	rsp, err := http.Get(fmt.Sprintf(`http://localhost:%d/rsd/%s`, SvrPort, udid))
	if err != nil {return nil, err}
	var data map[string]any
	err = json.NewDecoder(rsp.Body).Decode(&data)
	if err != nil {return nil, err}
	if ret, ok := data[`Ret`]; !ok || ret.(float64) != 0 {
		return nil, errors.New(`no tunnel found for ` + udid)
	}

	addr, err := net.ResolveTCPAddr(`tcp`, data[`Data`].(map[string]any)[`RSD`].(string))
	if err != nil {return nil, err}

	return NewFromTunnel(addr)
}

func NewFromTunnel(addr *net.TCPAddr) (*Service, error) {
	s := &Service{TCPAddr: addr}
	return util.Return(s, s.connect())
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

func (x *Service) LockdownService() (lockd *lockdown.Service, err error) {
	var svc *plist.Service
	if _, ok := x.Services[ComAppleMobileLockdownRemoteTrusted]; ok {
		svc, err = x.StartService(ComAppleMobileLockdownRemoteTrusted)
	} else {
		svc, err = x.StartService(ComAppleMobileLockdownRemoteUntrusted)
	}

	if err == nil {
		lockd = &lockdown.Service{Service: svc}
		_, err = lockd.GetDescriptor()
	}

	return
}

func (x *Service) GetServiceAddr(name string, useXpc bool) (*net.TCPAddr, error) {
	s, ok := x.Services[name]
	if !ok {return nil, Missing
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

func (x *Service) StartXpcService(name string) (*xpc.RemoteXpcConnection, error) {
	addr, err := x.GetServiceAddr(name, true)
	if err != nil {return nil, err}
	return xpc.NewRemoteXpc(addr)
}

func (x *Service) StartService(name string) (*plist.Service, error) {
	addr, err := x.GetServiceAddr(name, false)
	if err != nil {return nil, err}
	log.Printf(`RSD StartService %s`, addr)

	conn, err := base.NewConnection(addr)
	if err != nil {return nil, err}

	svc := &plist.Service{
		Connection: plist.NewConnection(conn.Conn),
	}

	rsp := make(map[string]any)
	err = svc.Get(map[string]any{
		`Label`:           base.ProgramName,
		`ProtocolVersion`: `2`,
		`Request`:         `RSDCheckin`,
	}, &rsp)

	if err == nil {
		if rsp[`Request`] != `RSDCheckin` {
			err = fmt.Errorf(`CHECKIN: %+v`, rsp)
		}
	}

	if err == nil {
		rsp = make(map[string]any)
		err = svc.Recv(&rsp)
		if err == nil {
			if rsp[`Request`] != `StartService` {
				err = fmt.Errorf(`SERVICE: %+v`, rsp)
			}
		}
	}

	return svc, err
}

var (
	guard sync.Mutex
)

func hijack(f func()error) error {
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