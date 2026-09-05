package plist

import (
	"fmt"
	"github.com/larryhou/ix/api/j3"
	"github.com/larryhou/ix/api/j3/usbmux"
)

type Service struct {
	*Connection
	PortNumber int
	Handle     *j3.Handle
}

func (x *Service) Connect() error {
	req := &j3.ConnectRequest{
		DeviceID:   x.Handle.DVID,
		PortNumber: x.PortNumber,
	}

	mux := usbmux.NewConnection(x.Conn)
	seq, err := mux.Send(req)
	if err != nil {return err}

	rsp := &j3.ConnectResponse{}
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

func (x *Service) QueryType() (*j3.RequestResponse, error) {
	req := &j3.RequestRequest{
		Label:   j3.ProgramName,
		Request: j3.RequestQueryType,
	}

	rsp := &j3.RequestResponse{}
	return rsp, x.Connection.Get(req, rsp)
}

func (x *Service) Key(name string) (*j3.KeyResponse, error) {
	req := &j3.KeyRequest{
		GetValueRequest: j3.GetValueRequest{
			Label:   j3.ProgramName,
			Request: j3.RequestGetValue,
		},
		Key: name,
	}

	rsp := &j3.KeyResponse{}
	return rsp, x.Connection.Get(req, rsp)
}