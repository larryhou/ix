package lockdown

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
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

func New(mux *base.UsbMux, device *base.DeviceDescriptor) (*Service, error) {
	u, err := mux.Spawn()
	if err != nil {
		return nil, err
	}

	s := &base.Service{
		UsbMux:           u,
		DeviceDescriptor: device,
		ByteOrder:        binary.BigEndian,
		PortNumber:       PortNumber,
	}

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
	*base.Service
	*Descriptor
	*PairRecord
	Lockdown         *Lockdown
	EnableSessionSSL *bool
	SessionID        *string

	tlsConfig *tls.Config
}

func (x *Service) GetDescriptor() (*base.GetValueResponse[Descriptor], error) {
	if x.SessionID != nil {return nil, errors.New(`only accessible before session start`)}
	req := &base.GetValueRequest{
		Label:   base.ProgramName,
		Request: base.RequestGetValue,
	}

	rsp := &base.GetValueResponse[Descriptor]{}
	err := x.Get(req, rsp)
	if err == nil { x.Descriptor = rsp.Value }
	return rsp, err
}

func (x *Service) GetValue() (*base.GetValueResponse[Lockdown], error) {
	if x.SessionID == nil {return nil, errors.New(`only accessible after session start`)}
	req := &base.GetValueRequest{
		Label:   base.ProgramName,
		Request: base.RequestGetValue,
	}

	rsp := &base.GetValueResponse[Lockdown]{}
	err := x.Get(req, rsp)
	if err == nil { x.Lockdown = rsp.Value }
	return rsp, err
}

func (x *Service) ReadPairRecord() (*ReadPairRecordResponse, error) {
	req := &ReadPairRecordRequest{
		ClientVersionString: base.VersionName,
		ProgName:            base.ProgramName,
		KLibUSBMuxVersion:   base.LibVersion,
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
	if x.SessionID != nil {return nil}
	req := &base.StartSessionRequest{
		RequestRequest: base.RequestRequest{
			Label:   base.ProgramName,
			Request: base.RequestStartSession,
		},
		SystemBUID: x.SystemBUID,
		HostID:     x.HostID,
	}

	rsp := &base.StartSessionResponse{}
	if err := x.Get(req, rsp); err != nil {return err}
	x.EnableSessionSSL = &rsp.EnableSessionSSL
	x.SessionID = &rsp.SessionID

	log.Printf(`StartSession %s SSL/%v`, *x.SessionID, *x.EnableSessionSSL)
	return x.tlsUsbMux(rsp.EnableSessionSSL, &x.UsbMux)
}

func (x *Service) StopSession() error {
	if x.SessionID == nil {return nil}

	req := &base.StopSessionRequest{
		RequestRequest: base.RequestRequest{
			Label:   base.ProgramName,
			Request: base.RequestStartSession,
		},
		SessionID: *x.SessionID,
	}

	if err := x.Send(req); err != nil {return err}

	rsp := &base.StopSessionResponse{}
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

func (x *Service) StartService(name string) (*base.Service, error) {
	req := &StartServiceRequest{
		RequestRequest: base.RequestRequest{
			Label:   base.ProgramName,
			Request: RequestStartService,
		},
		Service: name,
	}

	rsp := &StartServiceResponse{}
	if err := x.Get(req, rsp); err != nil {return nil, err}

	mux, err := x.Spawn()
	if err != nil {return nil, err}

	port := rsp.Port
	port = (port & 0xFF) << 8 | (port & 0xFF00) >> 8

	log.Printf(`StartService %s/%d SSL/%v`, rsp.Service, port, rsp.EnableServiceSSL)

	s := &base.Service{
		UsbMux:           mux,
		DeviceDescriptor: x.DeviceDescriptor,
		ByteOrder:        binary.BigEndian,
		PortNumber:       port,
	}

	if err = s.Connect(); err == nil {
		err = x.tlsUsbMux(rsp.EnableServiceSSL, &mux)
	}

	return s, err
}

func (x *Service) tlsUsbMux(ssl bool, mux **base.UsbMux) error {
	if *mux == nil {
		mux_, err := x.UsbMux.Spawn()
		if err != nil {return err}
		*mux = mux_
	}

	if ssl {
		tlsConfig, err := x.TLSConfig()
		if err != nil {return err}
		tlsConn := tls.Client((*mux).Conn, tlsConfig)
		if err = tlsConn.Handshake(); err != nil {
			return fmt.Errorf(`TLS HANDSHAKE %v`, err)
		}
		(*mux).Conn = tlsConn
	}
	
	return nil
}
