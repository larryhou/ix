package lockdown

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
)

const (
	ServiceName = `com.apple.mobile.lockdown`
	PortNumber  = 32498
)

const (
	RequestResetPairing   = `ResetPairing`
	RequestReadPairRecord = `ReadPairRecord`
	RequestStartService   = `StartService`
)

func New(mux *usbmux.UsbMux, device *usbmux.DeviceDescriptor) (*Service, error) {
	u, err := mux.Spawn()
	if err != nil {
		return nil, err
	}

	s := &usbmux.Service{
		UsbMux:           u,
		DeviceDescriptor: device,
		ByteOrder:        binary.BigEndian,
		PortNumber:       PortNumber,
	}

	//name, _ := os.Hostname()
	//host := uuid.NewMD5(uuid.NameSpaceDNS, []byte(name))
	//fmt.Printf("%s %s\n", host, name)

	service := &Service{Service: s}

	if rsp, err := service.ReadPairRecord(); err == nil {
		record, err := rsp.PairRecord()
		if err != nil {return nil, err}
		service.PairRecord = record
	} else {
		panic(`request pairing`)
	}

	if err = service.Connect(); err == nil {
		err = service.StartSession()
	}

	return service, err
}

type Service struct {
	*usbmux.Service
	*Descriptor
	*PairRecord
	Lockdown         *Lockdown
	EnableSessionSSL *bool
	SessionID        *string

	tlsConfig *tls.Config
}

func (x *Service) ReadDescriptorValue() (*usbmux.GetValueResponse[Descriptor], error) {
	if x.SessionID != nil {return nil, errors.New(`only accessible before session start`)}
	req := &usbmux.GetValueRequest{
		Label:   usbmux.ProgramName,
		Request: usbmux.RequestGetValue,
	}

	rsp := &usbmux.GetValueResponse[Descriptor]{}
	err := x.Get(req, rsp)
	if err == nil { x.Descriptor = rsp.Value }
	return rsp, err
}

func (x *Service) ReadValue() (*usbmux.GetValueResponse[Lockdown], error) {
	if x.SessionID == nil {return nil, errors.New(`only accessible after session start`)}
	req := &usbmux.GetValueRequest{
		Label:   usbmux.ProgramName,
		Request: usbmux.RequestGetValue,
	}

	rsp := &usbmux.GetValueResponse[Lockdown]{}
	err := x.Get(req, rsp)
	if err == nil { x.Lockdown = rsp.Value }
	return rsp, err
}

func (x *Service) ReadPairRecord() (*ReadPairRecordResponse, error) {
	req := &ReadPairRecordRequest{
		ClientVersionString: usbmux.VersionName,
		ProgName:            usbmux.ProgramName,
		KLibUSBMuxVersion:   usbmux.LibVersion,
		MessageType:         RequestReadPairRecord,
		PairRecordID:        x.DeviceDescriptor.Properties.SerialNumber,
	}

	idx, err := x.UsbMux.Send(req)
	if err != nil {return nil, err}

	rsp := &ReadPairRecordResponse{}
	err = x.UsbMux.Recv(rsp, idx)
	if err == nil {
		if len(rsp.PairRecordData) == 0 {
			err = fmt.Errorf(`not pair record: %s`, req.PairRecordID)
		}
	}
	return rsp, err
}

func (x *Service) TLSConfig() (*tls.Config, error) {
	if x.tlsConfig == nil {
		cert, err := tls.X509KeyPair(x.PairRecord.HostCertificate, x.PairRecord.HostPrivateKey)
		if err != nil {
			return nil, err
		}

		x.tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		}
	}

	return x.tlsConfig, nil
}

func (x *Service) StartSession() error {
	req := &usbmux.StartSessionRequest{
		Label:      usbmux.ProgramName,
		Request:    usbmux.RequestStartSession,
		SystemBUID: x.SystemBUID,
		HostID:     x.HostID,
	}

	if err := x.Send(req); err != nil {return err}
	rsp := &usbmux.StartSessionResponse{}
	if err := x.Recv(rsp); err != nil {return err}
	x.EnableSessionSSL = &rsp.EnableSessionSSL
	x.SessionID = &rsp.SessionID

	log.Printf(`StartSession %s %v`, *x.SessionID, *x.EnableSessionSSL)

	if rsp.EnableSessionSSL {
		conf, err := x.TLSConfig()
		if err != nil {return err}

		tlsConn := tls.Client(x.Conn, conf)
		if err = tlsConn.Handshake(); err == nil { x.Conn = tlsConn }
		return err
	}

	return nil
}

func (x *Service) StopSession() error {
	if x.SessionID == nil {
		return errors.New(`session not started`)
	}

	req := &usbmux.StopSessionRequest{
		Label:     usbmux.ProgramName,
		Request:   usbmux.RequestStopSession,
		SessionID: *x.SessionID,
	}

	if err := x.Send(req); err != nil {return err}

	rsp := &usbmux.StopSessionResponse{}
	if err := x.Recv(rsp); err != nil {
		return err
	}

	log.Printf(`StopSession %s`, *x.SessionID)

	x.SessionID = nil
	if *x.EnableSessionSSL {
		x.Conn = x.Conn.(*tls.Conn).NetConn()
		x.EnableSessionSSL = nil
	}

	return nil
}

func (x *Service) StartService(name string) (*StartServiceResponse, error) {
	req := &StartServiceRequest{
		RequestRequest: usbmux.RequestRequest{
			Label:   usbmux.ProgramName,
			Request: RequestStartService,
		},
		Service: name,
	}

	rsp := &StartServiceResponse{}
	return rsp, x.Get(req, rsp)
}
