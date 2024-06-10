package device

import (
	"encoding/binary"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/afc"
	"github.com/larryhou/gomobiledevice3/api/application"
	"github.com/larryhou/gomobiledevice3/api/housearrest"
	"github.com/larryhou/gomobiledevice3/api/lockdown"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"io"
	"log"
	"net"
)

func New(mux *usbmux.UsbMux, descriptor *usbmux.DeviceDescriptor) (*Device, error) {
	dev := &Device{
		descriptor: descriptor,
		usbmux:     mux,
	}

	ld, err := lockdown.New(dev.usbmux, dev.descriptor)
	if err != nil {return nil, err}

	dev.lockdown = ld
	return dev, nil
}

type Device struct {
	descriptor *usbmux.DeviceDescriptor
	usbmux     *usbmux.UsbMux

	lockdown    *lockdown.Service
	application *application.Service
	afc         *afc.Service
	houseArrest *housearrest.Service
}

func (x *Device) LockdownService() *lockdown.Service { return x.lockdown }

func (x *Device) ApplicationService() (*application.Service, error) {
	if x.application == nil {
		if service, err := x.lockdown.StartService(application.ServiceName); err == nil {
			x.application = application.New(service)
		}
	}

	return x.application, nil
}

func (x *Device) AfcService() (*afc.Service, error) {
	if x.application == nil {
		if service, err := x.lockdown.StartService(afc.ServiceName); err == nil {
			x.afc = afc.New(service)
		}
	}

	return x.afc, nil
}

func (x *Device) HouseArrestService() (*housearrest.Service, error) {
	if x.application == nil {
		if service, err := x.lockdown.StartService(housearrest.ServiceName); err == nil {
			x.houseArrest = housearrest.New(service)
		}
	}

	return x.houseArrest, nil
}

func (x *Device) Forward(localPort, devicePort int) error {
	create := func() (net.Conn, error) {
		mux, err := x.usbmux.Spawn()
		if err != nil {return nil, err}

		s := &usbmux.Service{
			UsbMux:           mux,
			DeviceDescriptor: x.descriptor,
			ByteOrder:        binary.BigEndian,
			PortNumber:       devicePort,
		}

		return s, s.Connect()
	}

	proxy, err := net.Listen(`tcp`, fmt.Sprintf(`:%d`, localPort))
	if err != nil {return err}

	pipe := func(w io.WriteCloser, r io.Reader) {
		if _, err := io.Copy(w, r); err != nil {
			if err == io.EOF {
				w.Close()
			}
		}
	}

	for {
		if conn, err := proxy.Accept(); err == nil {
			if remote, err := create(); err == nil {
				go pipe(conn, remote)
				go pipe(remote, conn)
			} else {
				log.Printf(`Connect/%d %v`, devicePort, err)
				conn.Close()
			}
		} else {
			return err
		}
	}
}


