package device

import (
	"crypto/tls"
	"github.com/larryhou/gomobiledevice3/api/application"
	"github.com/larryhou/gomobiledevice3/api/lockdown"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
)

func New(mux *usbmux.USBMux, descriptor *usbmux.DeviceDescriptor) (*Device, error) {
	dev := &Device{
		descriptor: descriptor,
		usbmux:     mux,
	}

	ld, err := lockdown.New(dev.usbmux, dev.descriptor)
	if err != nil {return nil, err}

	dev.lockdownService = ld
	return dev, nil
}

type Device struct {
	descriptor         *usbmux.DeviceDescriptor
	usbmux             *usbmux.USBMux
	lockdownService    *lockdown.Service
	applicationService *application.Service
}

func (x *Device) hton(port int) int {
	return (port & 0xFF) << 8 | (port & 0xFF00) >> 8
}

func (x *Device) SpawnUsbMux(ssl bool) (*usbmux.USBMux, error) {
	mux, err := x.usbmux.Spawn()
	if err != nil {return nil, err}

	if ssl {
		tlsConfig, err := x.lockdownService.TLSConfig()
		if err != nil {return nil, err}

		tlsConn := tls.Client(mux.Conn, tlsConfig)
		if err = tlsConn.Handshake(); err != nil {return nil, err}
		mux.Conn = tlsConn
	}

	return mux, nil
}

func (x *Device) ApplicationService() (*application.Service, error) {
	if x.applicationService != nil {
		return x.applicationService, nil
	}

	if rsp, err := x.lockdownService.StartService(application.Name); err == nil {
		mux, err := x.SpawnUsbMux(rsp.EnableServiceSSL)
		if err != nil {return nil, err}
		app, err := application.New(mux, x.descriptor, x.hton(rsp.Port))
		if err != nil {return nil, err}
		x.applicationService = app
		return app, nil
	} else {
		return nil, err
	}
}
