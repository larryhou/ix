package remotepair

import (
	"encoding/binary"
	"fmt"
	"github.com/grandcat/zeroconf"
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
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
	var ent *zeroconf.ServiceEntry
	err := bonjour.Browse(bonjour.RemotePairingServiceName, func(v *zeroconf.ServiceEntry) bool {
		ent = v
		return false
	})

	if err != nil {return err}

	var address string
	switch {
	case len(ent.AddrIPv6) != 0:
		address = fmt.Sprintf(`[%s]:%d`, ent.AddrIPv6[0], ent.Port)
	case len(ent.AddrIPv4) != 0:
		address = fmt.Sprintf(`%s:%d`, ent.AddrIPv4[0], ent.Port)
	}

	mux := &usbmux.UsbMux{ByteOrder: binary.LittleEndian}
	err = mux.Connect(address)
	if err != nil { return err }

	x.UsbMux = mux
	//if err = x.handshake(); err == nil {
	//	err = x.validate()
	//}

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
