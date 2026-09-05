package device

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/larryhou/ix/api/afc"
	"github.com/larryhou/ix/api/dvt"
	"github.com/larryhou/ix/api/dvt/deviceinfo"
	"github.com/larryhou/ix/api/dvt/processctrl"
	"github.com/larryhou/ix/api/dvt/remotesvr"
	"github.com/larryhou/ix/api/heartbeat"
	"github.com/larryhou/ix/api/housearrest"
	"github.com/larryhou/ix/api/installationproxy"
	"github.com/larryhou/ix/api/j3"
	"github.com/larryhou/ix/api/j3/usbmux"
	"github.com/larryhou/ix/api/lockdown"
	"github.com/larryhou/ix/api/syslog"
	"github.com/larryhou/ix/api/tunnel"
	"github.com/larryhou/ix/api/tunnel/rsd"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

const (
	Any = ``
)

//goland:noinspection GoSnakeCaseUsage,GoUnusedGlobalVariable
var (
	VERSION_17_0_0 = NewVersion(`17.0.0`)
	VERSION_17_3_1 = NewVersion(`17.3.1`)
	VERSION_17_4_0 = NewVersion(`17.4.0`)
)

type Version [8]byte

func (x Version) Compare(v Version) int {
	v1 := binary.BigEndian.Uint64(x[:])
	v2 := binary.BigEndian.Uint64(v[:])
	switch {
	case v1 < v2:
		return -1
	case v1 > v2:
		return +1
	default:
		return 0
	}
}

func (x Version) Major() int {
	return int(binary.BigEndian.Uint16(x[0:2]))
}

func (x Version) Minor() int {
	return int(binary.BigEndian.Uint16(x[2:4]))
}

func NewVersion(vers string) Version {
	i := 0
	var v Version
	for _, s := range strings.Split(vers, `.`) {
		n, _ := strconv.Atoi(s)
		binary.BigEndian.PutUint16(v[i*2:], uint16(n))
		i++
	}

	return v
}

func NewFromTunnelD(udid string) (*Service, error) {
	r, err := rsd.NewFromTunnelD(udid)
	if err != nil {
		return nil, err
	}

	dev := &Service{
		handle:   &j3.Handle{UDID: udid},
		lockdown: r,
	}

	return dev, nil
}

func New(udid string) (*Service, error) {
	mux, err := usbmux.New()
	if err != nil {
		return nil, err
	}
	rsp, err := mux.List()
	if err != nil {
		return nil, err
	}
	if len(rsp.DeviceList) == 0 {
		if udid == Any {
			return nil, errors.New(`NO CONNECTED DEVICES`)

		}
		return NewFromTunnelD(udid)
	}

	var device *j3.Device
	if udid == Any {
		device = rsp.DeviceList[0]
	} else {
		for _, it := range rsp.DeviceList {
			if it.Properties.SerialNumber == udid {
				device = it
				break
			}
		}
		if device == nil {
			return nil, fmt.Errorf(`NO DEVICE WITH %s`, udid)
		}
	}

	dev := &Service{
		handle: &j3.Handle{
			UDID: device.Properties.SerialNumber,
			DVID: device.DeviceID,
		},
	}

	lockd, err := lockdown.New(mux, dev.handle)
	if err != nil {
		return nil, err
	}

	if NewVersion(lockd.ProductVersion).Compare(VERSION_17_0_0) >= 0 {
		_ = lockd.Close()
		return NewFromTunnelD(device.Properties.SerialNumber)
	}

	dev.lockdown = lockd
	return dev, nil
}

type Service struct {
	handle *j3.Handle

	lockdown     lockdown.ServiceProvider
	installation *installationproxy.Service
	afc          *afc.Service
	houseArrest  *housearrest.Service
	dvt          *dvt.Service
	cdtunnel     *tunnel.Service
}

func (x *Service) StartCoreDeviceTunnelService() (*tunnel.Service, error) {
	if x.cdtunnel == nil {
		service, err := x.lockdown.StartService(rsd.ComAppleInternalDevicecomputeCoreDeviceProxy)
		if err == nil {
			x.cdtunnel, err = tunnel.New(service, tunnel.MtuTcp, context.Background())
		}

		if err == nil {
			err = x.cdtunnel.Start(service)
		}

		return x.cdtunnel, err
	}

	return x.cdtunnel, nil
}

func (x *Service) pick(name, rsdname string) string {
	switch x.lockdown.(type) {
	case *rsd.Service:
		return rsdname
	default:
		return name
	}
}

func (x *Service) dvtService() (*dvt.Service, error) {
	if x.dvt == nil {
		name := x.pick(remotesvr.ServiceName, rsd.ComAppleInstrumentsDtservicehub)
		svr, err := remotesvr.New(x.lockdown, name)
		if err != nil {
			return nil, err
		}
		x.dvt, err = dvt.New(svr)
		if err != nil {
			return nil, err
		}
	}

	return x.dvt, nil
}

func (x *Service) Lockdown() lockdown.ServiceProvider {
	return x.lockdown
}

func (x *Service) InstallationProxyService() (*installationproxy.Service, error) {
	if x.installation == nil {
		name := x.pick(installationproxy.ServiceName, rsd.ComAppleMobileInstallationProxyShimRemote)
		if service, err := x.lockdown.StartService(name); err == nil {
			x.installation = installationproxy.New(service)
		} else {
			return nil, err
		}
	}

	return x.installation, nil
}

func (x *Service) AfcService() (*afc.Service, error) {
	if x.afc == nil {
		name := x.pick(afc.ServiceName, rsd.ComAppleAfcShimRemote)
		if service, err := x.lockdown.StartService(name); err == nil {
			x.afc = afc.New(service)
		} else {
			return nil, err
		}
	}

	return x.afc, nil
}

func (x *Service) HouseArrestService() (*housearrest.Service, error) {
	if x.houseArrest == nil {
		name := x.pick(housearrest.ServiceName, rsd.ComAppleMobileHouseArrestShimRemote)
		if service, err := x.lockdown.StartService(name); err == nil {
			x.houseArrest = housearrest.New(service)
		} else {
			return nil, err
		}
	}

	return x.houseArrest, nil
}

func (x *Service) Install(ipa string) error {
	afcSvc, err := x.AfcService()
	if err != nil {
		return err
	}
	proxy, err := x.InstallationProxyService()
	if err != nil {
		return err
	}
	return proxy.Install(ipa, afcSvc)
}

func (x *Service) Uninstall(identifer string) error {

	proxy, err := x.InstallationProxyService()
	if err != nil {
		return err
	}
	return proxy.Uninstall(identifer)
}

func (x *Service) ListApplications() (map[string]*installationproxy.Application, error) {
	proxy, err := x.InstallationProxyService()
	if err != nil {
		return nil, err
	}
	rsp, err := proxy.List()
	if err != nil {
		return nil, err
	}
	return rsp.LookupResult, nil
}

func (x *Service) ListProcesses() ([]*deviceinfo.Process, error) {
	svc, err := x.dvtService()
	if err != nil {
		return nil, err
	}
	di, err := svc.DeviceInfo()
	if err != nil {
		return nil, err
	}
	return di.ListProcesses()
}

func (x *Service) ReadDir(name string) ([]string, error) {
	svc, err := x.dvtService()
	if err != nil {
		return nil, err
	}
	di, err := svc.DeviceInfo()
	if err != nil {
		return nil, err
	}
	return di.ReadDir(name)
}

func (x *Service) Launch(identifer string, ctx processctrl.LaunchContext) (int, error) {
	svc, err := x.dvtService()
	if err != nil {
		return 0, err
	}
	pc, err := svc.ProcessCtrl()
	if err != nil {
		return 0, err
	}
	return pc.Launch(identifer, ctx)
}

func (x *Service) Kill(pid int) error {
	svc, err := x.dvtService()
	if err != nil {
		return err
	}
	pc, err := svc.ProcessCtrl()
	if err != nil {
		return err
	}
	return pc.Kill(pid)
}

func (x *Service) ScreenShot() ([]byte, error) {
	svc, err := x.dvtService()
	if err != nil {
		return nil, err
	}
	ss, err := svc.ScreenShot()
	if err != nil {
		return nil, err
	}
	return ss.Capture()
}

func (x *Service) SreenShotAndSave(name string, s *afc.Service) error {
	buf, err := x.ScreenShot()
	if err != nil {
		return err
	}
	return x.Save(name, bytes.NewReader(buf), int64(len(buf)), s)
}

func (x *Service) Save(name string, r io.Reader, n int64, s *afc.Service) error {
	h, err := s.Open(name, `w`)
	if err != nil {
		return err
	}
	defer h.Close()

	w, err := h.FileWriter(n)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

func (x *Service) Forward(localPort, devicePort int) error {
	create := func() (net.Conn, error) {
		return x.lockdown.StartService(x.lockdown.UserServiceName(devicePort))
	}

	proxy, err := net.Listen(`tcp`, fmt.Sprintf(`:%d`, localPort))
	if err != nil {
		return err
	}

	pipe := func(w io.WriteCloser, r io.Reader) {
		if _, err := io.Copy(w, r); err != nil {
			w.Close()
		}
	}

	for {
		if conn, err := proxy.Accept(); err == nil {
			if remote, err := create(); err == nil {
				go pipe(conn, remote)
				go pipe(remote, conn)
			} else {
				log.Printf(`Connect/%d %v CLOSE %s`, devicePort, err, conn.RemoteAddr())
				conn.Close()
			}
		} else {
			return err
		}
	}
}

func (x *Service) Logcat(w io.Writer) error {
	name := x.pick(syslog.ServiceName, rsd.ComAppleSyslogRelayShimRemote)
	svc, err := x.lockdown.StartService(name)
	if err != nil {
		return err
	}
	defer svc.Close()
	return syslog.New(svc).Streaming(w)
}

func (x *Service) Heartbeat() error {
	name := x.pick(heartbeat.ServiceName, rsd.ComAppleMobileHeartbeatShimRemote)
	svc, err := x.lockdown.StartService(name)
	if err != nil {
		return err
	}
	defer svc.Close()
	return heartbeat.New(svc).Run()
}
