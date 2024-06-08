package application

import (
	"encoding/binary"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
)


func New(mux *usbmux.UsbMux, device *usbmux.DeviceDescriptor, port int) (*Service, error) {
	s := &usbmux.Service{
		UsbMux:           mux,
		DeviceDescriptor: device,
		ByteOrder:        binary.BigEndian,
		PortNumber:       port,
	}

	return &Service{Service: s}, s.Connect()
}

type Service struct {
	*usbmux.Service
}

func (x *Service) List(opaque bool) (any, error) {
	req := &ListRequest{
		Command: CommandLookup,
		ClientOptions: &ClientOptions{
			ApplicationType: TypeAny,
		},
	}

	if !opaque {
		rsp := &ListResponse{}
		return rsp, x.Get(req, rsp)
	} else {
		var rsp any
		return rsp, x.Get(req, &rsp)
	}
}