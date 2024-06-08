package device

import (
	"github.com/larryhou/gomobiledevice3/api/application"
	"github.com/larryhou/gomobiledevice3/api/lockdown"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
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
