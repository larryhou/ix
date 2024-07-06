package device

import (
	"context"
	"fmt"
	"github.com/larryhou/j3idevice/api/afc"
	"github.com/larryhou/j3idevice/api/application"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/base/usbmux"
	"github.com/larryhou/j3idevice/api/dvt"
	"github.com/larryhou/j3idevice/api/dvt/applicationlisting"
	"github.com/larryhou/j3idevice/api/dvt/deviceinfo"
	"github.com/larryhou/j3idevice/api/dvt/processctrl"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/housearrest"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/tunnel"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"io"
	"log"
	"net"
)

func NewFromRSD(udid string) (*Service, error) {
	r, err := rsd.NewFromTunnelD(udid)
	if err != nil {return nil, err}

	dev := &Service{
		handle:   &base.Handle{UDID: udid},
		lockdown: r,
	}

	return dev, nil
}

func New(mux *usbmux.UsbMux, descriptor *base.DeviceDescriptor) (*Service, error) {
	dev := &Service{
		descriptor: descriptor,
		handle: &base.Handle{
			UDID: descriptor.Properties.SerialNumber,
			DVID: descriptor.DeviceID,
		},
	}

	lockd, err := lockdown.New(mux, dev.handle)
	if err != nil {return nil, err}

	dev.lockdown = lockd
	return dev, nil
}

type Service struct {
	descriptor *base.DeviceDescriptor
	handle     *base.Handle

	lockdown    lockdown.ServiceProvider
	application *application.Service
	afc         *afc.Service
	houseArrest *housearrest.Service
	dvt         *dvt.Service

	cdTunnel *tunnel.Service
}

func (x *Service) StartCoreDeviceTunnelService() (*tunnel.Service, error) {
	if x.cdTunnel == nil {
		service, err := x.lockdown.StartService(rsd.ComAppleInternalDevicecomputeCoreDeviceProxy)
		if err == nil {
			x.cdTunnel, err = tunnel.New(service, tunnel.MtuTcp, context.Background())
		}

		if err == nil {
			err = x.cdTunnel.Start(service)
		}

		return x.cdTunnel, err
	}

	return x.cdTunnel, nil
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

func (x *Service) ApplicationService() (*application.Service, error) {
	if x.application == nil {
		name := x.pick(application.ServiceName, rsd.ComAppleMobileInstallationProxyShimRemote)
		if service, err := x.lockdown.StartService(name); err == nil {
			x.application = application.New(service)
		} else {return nil, err}
	}

	return x.application, nil
}

func (x *Service) AfcService() (*afc.Service, error) {
	if x.application == nil {
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

func (x *Service) ListApplications() ([]*applicationlisting.Application, error) {
	svc, err := x.getdvt()
	if err != nil {return nil, err}
	al, err := svc.ApplicationListing()
	if err != nil {return nil, err}
	return al.List()
}

func (x *Service) ListProcesses() ([]*deviceinfo.Process, error) {
	svc, err := x.getdvt()
	if err != nil {return nil, err}
	di, err := svc.DeviceInfo()
	if err != nil {return nil, err}
	return di.ListProcesses()
}

func (x *Service) ReadDir(name string) ([]any, error) {
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


