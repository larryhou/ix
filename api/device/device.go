package device

import (
	"bytes"
	"context"
	"fmt"
	"github.com/larryhou/j3idevice/api/afc"
	"github.com/larryhou/j3idevice/api/dvt"
	"github.com/larryhou/j3idevice/api/dvt/deviceinfo"
	"github.com/larryhou/j3idevice/api/dvt/processctrl"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/heartbeat"
	"github.com/larryhou/j3idevice/api/housearrest"
	"github.com/larryhou/j3idevice/api/installationproxy"
	"github.com/larryhou/j3idevice/api/j3"
	"github.com/larryhou/j3idevice/api/j3/usbmux"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/syslog"
	"github.com/larryhou/j3idevice/api/tunnel"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"io"
	"log"
	"net"
)

func NewFromTunnelD(udid string) (*Service, error) {
	r, err := rsd.NewFromTunnelD(udid)
	if err != nil {return nil, err}

	dev := &Service{
		handle:   &j3.Handle{UDID: udid},
		lockdown: r,
	}

	return dev, nil
}

func New(mux *usbmux.UsbMux, device *j3.Device) (*Service, error) {
	dev := &Service{
		handle: &j3.Handle{
			UDID: device.Properties.SerialNumber,
			DVID: device.DeviceID,
		},
	}

	lockd, err := lockdown.New(mux, dev.handle)
	if err != nil {return nil, err}

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
	case *rsd.Service: return rsdname
	default: return name
	}
}

func (x *Service) getdvt() (*dvt.Service, error) {
	if x.dvt == nil {
		name := x.pick(remotesvr.ServiceName, rsd.ComAppleInstrumentsDtservicehub)
		svr, err := remotesvr.New(x.lockdown, name)
		if err != nil {return nil, err}
		x.dvt, err = dvt.New(svr)
		if err != nil {return nil, err}
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
		} else {return nil, err}
	}

	return x.installation, nil
}

func (x *Service) AfcService() (*afc.Service, error) {
	if x.afc == nil {
		name := x.pick(afc.ServiceName, rsd.ComAppleAfcShimRemote)
		if service, err := x.lockdown.StartService(name); err == nil {
			x.afc = afc.New(service)
		} else {return nil, err}
	}

	return x.afc, nil
}

func (x *Service) HouseArrestService() (*housearrest.Service, error) {
	if x.houseArrest == nil {
		name := x.pick(housearrest.ServiceName, rsd.ComAppleMobileHouseArrestShimRemote)
		if service, err := x.lockdown.StartService(name); err == nil {
			x.houseArrest = housearrest.New(service)
		} else {return nil, err}
	}

	return x.houseArrest, nil
}

func (x *Service) Install(ipa string) error {
	afcSvc, err := x.AfcService()
	if err != nil {return err}
	proxy, err := x.InstallationProxyService()
	if err != nil {return err}
	return proxy.Install(ipa, afcSvc)
}

func (x *Service) ListApplications() (map[string]*installationproxy.Application, error) {
	proxy, err := x.InstallationProxyService()
	if err != nil {return nil, err}
	rsp, err := proxy.List()
	if err != nil {return nil, err}
	return rsp.LookupResult, nil
}

func (x *Service) ListProcesses() ([]*deviceinfo.Process, error) {
	svc, err := x.getdvt()
	if err != nil {return nil, err}
	di, err := svc.DeviceInfo()
	if err != nil {return nil, err}
	return di.ListProcesses()
}

func (x *Service) ReadDir(name string) ([]string, error) {
	svc, err := x.getdvt()
	if err != nil {return nil, err}
	di, err := svc.DeviceInfo()
	if err != nil {return nil, err}
	return di.ReadDir(name)
}

func (x *Service) Launch(identifer string, ctx processctrl.LaunchContext) error {
	svc, err := x.getdvt()
	if err != nil {return err}
	pc, err := svc.ProcessCtrl()
	if err != nil {return err}
	_, err = pc.Launch(identifer, ctx)
	return err
}

func (x *Service) Kill(pid int) error {
	svc, err := x.getdvt()
	if err != nil {return err}
	pc, err := svc.ProcessCtrl()
	if err != nil {return err}
	return pc.Kill(pid)
}

func (x *Service) ScreenShot() ([]byte, error) {
	svc, err := x.getdvt()
	if err != nil {return nil, err}
	ss, err := svc.ScreenShot()
	if err != nil {return nil, err}
	return ss.Capture()
}

func (x *Service) SreenShotAndSave(name string, s *afc.Service) error {
	buf, err := x.ScreenShot()
	if err != nil {return err}
	return x.Save(name, bytes.NewReader(buf), int64(len(buf)), s)
}

func (x *Service) Save(name string, r io.Reader, n int64, s *afc.Service) error {
	h, err := s.Open(name, `w`)
	if err != nil {return err}
	defer h.Close()

	w, err := h.FileWriter(n)
	if err != nil {return err}
	_, err = io.Copy(w, r)
	return err
}

func (x *Service) Forward(localPort, devicePort int) error {
	create := func() (net.Conn, error) {
		return x.lockdown.StartService(x.lockdown.UserServiceName(devicePort))
	}

	proxy, err := net.Listen(`tcp`, fmt.Sprintf(`:%d`, localPort))
	if err != nil {return err}

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
	if err != nil {return err}
	defer svc.Close()
	return syslog.New(svc).Streaming(w)
}

func (x *Service) Heartbeat() error {
	name := x.pick(heartbeat.ServiceName, rsd.ComAppleMobileHeartbeatShimRemote)
	svc, err := x.lockdown.StartService(name)
	if err != nil {return err}
	defer svc.Close()
	return heartbeat.New(svc).Run()
}
