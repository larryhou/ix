package housearrest

import (
	"errors"
	"github.com/larryhou/j3idevice/api/afc"
	"github.com/larryhou/j3idevice/api/base/plist"
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
	s := &Service{Service: afc.New(conn)}
	return s
}

func NewFromRSD(provider lockdown.Provider) (*Service, error) {
	conn, err := provider.StartService(rsd.ComAppleMobileHouseArrestShimRemote)
	if err != nil {return nil, err}
	return New(conn), nil
}

type Service struct {
	*afc.Service
}

func (x *Service) Connect(identifier string, vend VendType) error {
	pc := plist.NewConnection(x.Conn())
	err := pc.Send(map[string]any{
		`Command`:    string(vend),
		`Identifier`: identifier,
	})
	if err != nil {return err}

	var rsp map[string]any
	err = pc.Recv(&rsp)
	if err == nil {
		if errStr, ok := rsp[`Error`]; ok {
			err = errors.New(errStr.(string))
		}
	}

	return err
}