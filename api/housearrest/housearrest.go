package housearrest

import (
	"errors"
	"github.com/larryhou/j3idevice/api/afc"
	"github.com/larryhou/j3idevice/api/j3/plist"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"net"
)

const (
	ServiceName = `com.apple.mobile.house_arrest`
)

type VendType string

const (
	VendContainer VendType = `VendContainer`
	VendDocuments VendType = `VendDocuments`
)

func New(conn net.Conn) *Service {
	s := &Service{Conn: conn}
	return s
}

func NewFromRSD(provider lockdown.ServiceProvider) (*Service, error) {
	conn, err := provider.StartService(rsd.ComAppleMobileHouseArrestShimRemote)
	if err != nil {return nil, err}
	return New(conn), nil
}

type Service struct {
	net.Conn
}

func (x *Service) AfcService(identifier string) (*afc.Service, error) {
	plc := plist.NewConnection(x.Conn)
	var rsp map[string]any
	err := plc.Get(map[string]any{
		`Command`:    string(VendContainer),
		`Identifier`: identifier,
	}, &rsp)

	if err == nil {
		if status, ok := rsp[`Status`]; !ok || status != `Complete` {
			err = errors.New(`BAD STATUS`)
		}
	}

	if err == nil {
		return afc.New(x.Conn), nil
	}

	return nil, err
}