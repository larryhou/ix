package lockdown

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/larryhou/ix/api/mux"
	"github.com/larryhou/ix/api/mux/plist"
	"github.com/larryhou/ix/api/mux/usb"
	"log"
	"strconv"
	"strings"
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

func New(umux *usb.UsbMux, device *mux.Handle) (*Service, error) {
	u, err := umux.Spawn()
	if err != nil {
		return nil, err
	}

	s := &plist.Service{
		Connection: plist.NewConnection(u.Conn),
		PortNumber: PortNumber,
		Handle:     device,
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

type ServiceProvider interface {
	UserServiceName(port int) string
	StartService(name string) (*plist.Service, error)
}

type Service struct {
	*plist.Service
	*Descriptor
	*PairRecord
	Lockdown         *Lockdown
	EnableSessionSSL *bool
	SessionID        *string

	tlsConfig *tls.Config
}

func (x *Service) Connect() error {
	err :=  x.Service.Connect()
	if err == nil {
		_, err = x.GetDescriptor()
	}
	return err
}

func (x *Service) GetDescriptor() (*mux.GetValueResponse[Descriptor], error) {
	if x.SessionID != nil {return nil, errors.New(`only accessible before session start`)}
	req := &mux.GetValueRequest{
		Label:   mux.ProgramName,
		Request: mux.RequestGetValue,
	}

	rsp := &mux.GetValueResponse[Descriptor]{}
	err := x.Get(req, rsp)
	if err == nil { x.Descriptor = rsp.Value }
	return rsp, err
}

func (x *Service) GetValue() (*mux.GetValueResponse[Lockdown], error) {
	if x.SessionID == nil {return nil, errors.New(`only accessible after session start`)}
	req := &mux.GetValueRequest{
		Label:   mux.ProgramName,
		Request: mux.RequestGetValue,
	}

	rsp := &mux.GetValueResponse[Lockdown]{}
	err := x.Get(req, rsp)
	if err == nil { x.Lockdown = rsp.Value }
	return rsp, err
}

func (x *Service) ReadPairRecord() (*ReadPairRecordResponse, error) {
	req := &ReadPairRecordRequest{
		ClientVersionString: mux.VersionName,
		ProgName:            mux.ProgramName,
		KLibUSBMuxVersion:   mux.MuxVersion,
		MessageType:         RequestReadPairRecord,
		PairRecordID:        x.Handle.UDID,
	}

	rsp := &ReadPairRecordResponse{}
	uconn := usb.NewConnection(x.Conn)
	err := uconn.Get(req, rsp)
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
	req := &mux.StartSessionRequest{
		RequestRequest: mux.RequestRequest{
			Label:   mux.ProgramName,
			Request: mux.RequestStartSession,
		},
		SystemBUID: x.SystemBUID,
		HostID:     x.HostID,
	}

	rsp := &mux.StartSessionResponse{}
	if err := x.Get(req, rsp); err != nil {return err}
	x.EnableSessionSSL = &rsp.EnableSessionSSL
	x.SessionID = &rsp.SessionID

	log.Printf(`StartSession %s SSL/%v`, *x.SessionID, *x.EnableSessionSSL)
	return x.tlsUsbMux(rsp.EnableSessionSSL, &x.Connection.Connection)
}

func (x *Service) StopSession() error {
	if x.SessionID == nil {return nil}

	req := &mux.StopSessionRequest{
		RequestRequest: mux.RequestRequest{
			Label:   mux.ProgramName,
			Request: mux.RequestStartSession,
		},
		SessionID: *x.SessionID,
	}

	if err := x.Send(req); err != nil {return err}

	rsp := &mux.StopSessionResponse{}
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

const (
	customServiceNamePrefix = `CustomLockdownServiceNamePrefix:`
)

func (x *Service) UserServiceName(port int) string {
	return customServiceNamePrefix + strconv.Itoa(port)
}

func (x *Service) StartService(name string) (*plist.Service, error) {
	port, ssl := 0, false
	if strings.HasPrefix(name, customServiceNamePrefix) {
		n, err := strconv.Atoi(name[len(customServiceNamePrefix):])
		if err != nil {return nil, err}
		port = n
	} else {
		req := &StartServiceRequest{
			RequestRequest: mux.RequestRequest{
				Label:   mux.ProgramName,
				Request: RequestStartService,
			},
			Service: name,
		}

		rsp := &StartServiceResponse{}
		if err := x.Get(req, rsp); err != nil {return nil, err}

		port = rsp.Port
		port = (port & 0xFF) << 8 | (port & 0xFF00) >> 8
		log.Printf(`StartService %s/%d SSL/%v`, rsp.Service, port, rsp.EnableServiceSSL)
		ssl = rsp.EnableServiceSSL
	}

	conn, err := x.Spawn()
	if err != nil {return nil, err}

	s := &plist.Service{
		Connection: plist.NewConnection(conn),
		PortNumber: port,
		Handle:     x.Handle,
	}

	if err = s.Connect(); err == nil {
		err = x.tlsUsbMux(ssl, &conn)
	}

	return s, err
}

func (x *Service) tlsUsbMux(ssl bool, conn **mux.Connection) error {
	if *conn == nil {
		cOnn, err := x.Spawn()
		if err != nil {return err}
		*conn = cOnn
	}

	if ssl {
		tlsConfig, err := x.TLSConfig()
		if err != nil {return err}
		tlsConn := tls.Client((*conn).Conn, tlsConfig)
		if err = tlsConn.Handshake(); err != nil {
			return fmt.Errorf(`TLS HANDSHAKE: %w`, err)
		}
		(*conn).Conn = tlsConn
	}
	
	return nil
}
