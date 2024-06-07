package application

import (
	"encoding/binary"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
)


type ClientOptions struct {
	ApplicationType string `plist:"ApplicationType"`
}

type Request struct {
	*ClientOptions `plist:"ClientOptions"`
	Command        string `plist:"Command"`
}

func New(mux *usbmux.USBMux, device *usbmux.DeviceDescriptor, port int) (*Service, error) {
	u, err := mux.Spawn()
	if err != nil {
		return nil, err
	}

	s := &usbmux.Service{
		USBMux:           u,
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
	req := &Request{
		Command: CommandLookup,
		ClientOptions: &ClientOptions{
			ApplicationType: TypeAny,
		},
	}

	var rsp any
	return rsp, x.USBMux.Get(req, rsp)
}