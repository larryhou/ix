package device

import (
	"context"
	"fmt"
	"github.com/larryhou/j3idevice/api/afc"
	"github.com/larryhou/j3idevice/api/application"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/base/plist"
	"github.com/larryhou/j3idevice/api/base/usbmux"
	"github.com/larryhou/j3idevice/api/housearrest"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/tunnel"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"io"
	"log"
	"net"
)

func New(mux *usbmux.UsbMux, descriptor *base.DeviceDescriptor) (*Device, error) {
	dev := &Device{
		usbmux:     mux,
		descriptor: descriptor,
		handle: &base.Handle{
			UDID: descriptor.Properties.SerialNumber,
			DVID: descriptor.DeviceID,
		},
	}

	ld, err := lockdown.New(dev.usbmux, dev.handle)
	if err != nil {return nil, err}

	dev.lockdown = ld
	return dev, nil
}

type Device struct {
	descriptor *base.DeviceDescriptor
	usbmux *usbmux.UsbMux
	handle *base.Handle

	lockdown    *lockdown.Service
	application *application.Service
	afc         *afc.Service
	houseArrest *housearrest.Service
	tunnel      *tunnel.Service
}

func (x *Device) TunnelService() (*tunnel.Service, error) {
	if x.tunnel == nil {
		service, err := x.lockdown.StartService(rsd.ComAppleInternalDevicecomputeCoreDeviceProxy)
		if err == nil {
			x.tunnel, err = tunnel.New(service, tunnel.MtuTcp, context.Background())
		}

		if err == nil {
			err = x.tunnel.Start(service)
		}

		return x.tunnel, err
	}

	return x.tunnel, nil
}

func (x *Device) LockdownService() *lockdown.Service { return x.lockdown }

func (x *Device) ApplicationService() (*application.Service, error) {
	if x.application == nil {
		if service, err := x.lockdown.StartService(application.ServiceName); err == nil {
			x.application = application.New(service)
		} else {return nil, err}
	}

	return x.application, nil
}

func (x *Device) AfcService() (*afc.Service, error) {
	if x.application == nil {
		if service, err := x.lockdown.StartService(afc.ServiceName); err == nil {
			x.afc = afc.New(service)
		} else {return nil, err}
	}

	return x.afc, nil
}

func (x *Device) HouseArrestService() (*housearrest.Service, error) {
	if x.application == nil {
		if service, err := x.lockdown.StartService(housearrest.ServiceName); err == nil {
			x.houseArrest = housearrest.New(service)
		} else {return nil, err}
	}

	return x.houseArrest, nil
}

func (x *Device) Forward(localPort, devicePort int) error {
	create := func() (net.Conn, error) {
		mux, err := x.usbmux.Spawn()
		if err != nil {return nil, err}

		s := &plist.Service{
			Connection: plist.NewConnection(mux),
			PortNumber: devicePort,
			Handle:     x.handle,
		}

		return s, s.Connect()
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


