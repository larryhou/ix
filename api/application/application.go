package application

import (
	"encoding/binary"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
)




func New(mux *usbmux.USBMux, device *usbmux.DeviceDescriptor, port int) (*Service, error) {
	s := &usbmux.Service{
		USBMux:           mux,
		DeviceDescriptor: device,
		ByteOrder:        binary.BigEndian,
		PortNumber:       port,
	}

	return &Service{Service: s}, s.Connect()
}

type Service struct {
	*usbmux.Service
}

func (x *Service) List() (any, error) {
	req := &ListRequest{
		Command: CommandLookup,
		ClientOptions: &ClientOptions{
			ApplicationType: TypeAny,
		},
	}

	rsp := &ListResponse{}
	return rsp, x.Get(req, rsp)
}