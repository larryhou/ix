package device

import (
	"github.com/larryhou/gomobiledevice3/api/application"
	"github.com/larryhou/gomobiledevice3/api/lockdown"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
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

func (x *Device) ApplicationService() (*application.Service, error) {
	if x.applicationService != nil {
		return x.applicationService, nil
	}

	if rsp, err := x.lockdownService.StartService(application.Name); err == nil {
		log.Printf(`StartService %+v`, rsp)
		app, err := application.New(x.usbmux, x.descriptor, rsp.Port)
		if err == nil {
			x.applicationService = app
		}
		return app, err
	} else {
		return nil, err
	}
}
