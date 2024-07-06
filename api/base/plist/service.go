package plist

import (
	"encoding/binary"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/base/usbmux"
	"net"
)

type Service struct {
	net.Conn
	binary.ByteOrder
	PortNumber int
	DeviceID   int
	Udid       string

	*Connection
}

func (x *Service) Connect() error {
	req := &base.ConnectRequest{
		DeviceID:   x.DeviceID,
		PortNumber: x.PortNumber,
	}

	mux := usbmux.NewConnection(x.Conn)
	seq, err := mux.Send(req)
	if err != nil {return err}

	rsp := &base.ConnectResponse{}
	if err = mux.Recv(rsp, seq); err == nil {
		if rsp.Number != usbmux.ResultOk {
			err = fmt.Errorf(`CONNECT: %d`, rsp.Number)
		}
	}

	if err == nil {
		x.Connection = NewConnection(mux.Conn)
	}

	return err
}

func (x *Service) QueryType() (*base.RequestResponse, error) {
	req := &base.RequestRequest{
		Label:   usbmux.ProgramName,
		Request: base.RequestQueryType,
	}

	rsp := &base.RequestResponse{}
	return rsp, x.Connection.Get(req, rsp)
}

func (x *Service) Key(name string) (*base.KeyResponse, error) {
	req := &base.KeyRequest{
		GetValueRequest: base.GetValueRequest{
			Label:   usbmux.ProgramName,
			Request: base.RequestGetValue,
		},
		Key: name,
	}

	rsp := &base.KeyResponse{}
	return rsp, x.Connection.Get(req, rsp)
}