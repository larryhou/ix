package plist

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/base/usbmux"
)

type Service struct {
	*Connection
	PortNumber int
	Handle     *base.Handle
}

func (x *Service) Connect() error {
	req := &base.ConnectRequest{
		DeviceID:   x.Handle.DVID,
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
		Label:   base.ProgramName,
		Request: base.RequestQueryType,
	}

	rsp := &base.RequestResponse{}
	return rsp, x.Connection.Get(req, rsp)
}

func (x *Service) Key(name string) (*base.KeyResponse, error) {
	req := &base.KeyRequest{
		GetValueRequest: base.GetValueRequest{
			Label:   base.ProgramName,
			Request: base.RequestGetValue,
		},
		Key: name,
	}

	rsp := &base.KeyResponse{}
	return rsp, x.Connection.Get(req, rsp)
}