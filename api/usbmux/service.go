package usbmux

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"howett.net/plist"
)

type Service struct {
	*UsbMux
	*DeviceDescriptor
	binary.ByteOrder
	PortNumber int
}

func (x *Service) Connect() error {
	req := &ConnectRequest{
		DeviceID:   x.DeviceDescriptor.DeviceID,
		PortNumber: x.PortNumber,
	}

	seq, err := x.UsbMux.Send(req)
	if err != nil {return err}

	rsp := &ConnectResponse{}
	if err = x.UsbMux.Recv(rsp, seq); err == nil {
		if rsp.Number != ResultOk {
			err = fmt.Errorf(`connect: %d`, rsp.Number)
		}
	}
	return err
}

func (x *Service) Send(msg any) error {
	rsv := make([]byte, 4)
	buf := &bytes.Buffer{}
	buf.Write(rsv)
	err := plist.NewEncoder(buf).Encode(msg)
	if err != nil {return err}
	x.ByteOrder.PutUint32(rsv, uint32(buf.Len()-4))
	copy(buf.Bytes(), rsv)
	_, err = x.UsbMux.Write(buf.Bytes())
	return err
}

func (x *Service) Recv(msg any) error {
	rsv := make([]byte, 4)
	if _, err := x.UsbMux.Read(rsv); err != nil {
		return err
	}

	raw := make([]byte, x.ByteOrder.Uint32(rsv))
	if _, err := x.UsbMux.Read(raw); err != nil {
		return err
	}

	err := plist.NewDecoder(bytes.NewReader(raw)).Decode(msg)
	if err == nil {
		if r, ok := msg.(Retcode); ok {
			err = r.Verify()
		}
	}
	return err
}

func (x *Service) QueryType() (*RequestResponse, error) {
	req := &RequestRequest{
		Label:   ProgramName,
		Request: RequestQueryType,
	}

	rsp := &RequestResponse{}
	return rsp, x.Get(req, rsp)
}

func (x *Service) Key(name string) (*KeyResponse, error) {
	req := &KeyRequest{
		GetValueRequest: GetValueRequest{
			Label:   ProgramName,
			Request: RequestGetValue,
		},
		Key: name,
	}

	rsp := &KeyResponse{}
	return rsp, x.Get(req, rsp)
}

func (x *Service) Get(req, rsp any) error {
	if err := x.Send(req); err == nil {
		return x.Recv(rsp)
	} else {
		return err
	}
}