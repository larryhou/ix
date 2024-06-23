package remotepair

import (
	"encoding/binary"
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"github.com/larryhou/gomobiledevice3/api/lockdown"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
)

func New(uuid string) (*Service, error) {
	s := &Service{

	}

	return s, s.Connect()
}

type Service struct {
	*usbmux.UsbMux
	UUID string

	n int
}

func (x *Service) Connect() error {
	addr, err := bonjour.TCPAddr(bonjour.Mobdev2ServiceName)
	if err != nil {return err}

	mux := &usbmux.UsbMux{ByteOrder: binary.LittleEndian}
	err = mux.Connect(addr.String())
	if err != nil { return err }

	x.UsbMux = mux
	//if err = x.handshake(); err == nil {
	//	err = x.validate()
	//}

	us := &usbmux.Service{
		UsbMux:    mux,
		ByteOrder: binary.BigEndian,
	}

	rsp, err := us.QueryType()
	log.Printf(`%+v %v`, rsp, err)

	var ld *lockdown.Service
	if err == nil {
		ld = &lockdown.Service{
			Service: us,
		}

		rsp, err := ld.GetDescriptor()
		log.Printf(`%+v %+v %v`, rsp, rsp.Value, err)
	}

	return err
}

func (x *Service) handshake() error {

	panic(``)
}

func (x *Service) validate() error {
	panic(``)
}


func (x *Service) Send(msg any) error {
	//data := map[string]any{
	//	`plain`: map[string]any{
	//		`_0`: msg,
	//	},
	//	`originatedBy`:   `host`,
	//	`sequenceNumber`: x.n,
	//}
	//
	//buf := &bytes.Buffer{}
	//xpc.Encode(buf, &xpc.Message{
	//
	//})
	//x.n++
	panic(``)
}
