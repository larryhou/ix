package plist

import (
	"fmt"
	"github.com/larryhou/ix/api/mux"
	"github.com/larryhou/ix/api/mux/usb"
)

type Service struct {
	*Connection
	PortNumber int
	Handle     *mux.Handle
}

func (x *Service) Connect() error {
	req := &mux.ConnectRequest{
		DeviceID:   x.Handle.DVID,
		PortNumber: x.PortNumber,
	}

	uconn := usb.NewConnection(x.Conn)
	seq, err := uconn.Send(req)
	if err != nil {return err}

	rsp := &mux.ConnectResponse{}
	if err = uconn.Recv(rsp, seq); err == nil {
		if rsp.Number != usb.ResultOk {
			err = fmt.Errorf(`CONNECT: %d`, rsp.Number)
		}
	}

	if err == nil {
		x.Connection = NewConnection(uconn.Conn)
	}

	return err
}

func (x *Service) QueryType() (*mux.RequestResponse, error) {
	req := &mux.RequestRequest{
		Label:   mux.ProgramName,
		Request: mux.RequestQueryType,
	}

	rsp := &mux.RequestResponse{}
	return rsp, x.Connection.Get(req, rsp)
}

func (x *Service) Key(name string) (*mux.KeyResponse, error) {
	req := &mux.KeyRequest{
		GetValueRequest: mux.GetValueRequest{
			Label:   mux.ProgramName,
			Request: mux.RequestGetValue,
		},
		Key: name,
	}

	rsp := &mux.KeyResponse{}
	return rsp, x.Connection.Get(req, rsp)
}