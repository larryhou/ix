package housearrest

import (
	"errors"
	"github.com/larryhou/ix/api/afc"
	"github.com/larryhou/ix/api/mux/plist"
	"github.com/larryhou/ix/api/lockdown"
	"github.com/larryhou/ix/api/tunnel/rsd"
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

func (x *Service) vend(vt VendType, identifier string) (*afc.Service, error) {
	plc := plist.NewConnection(x.Conn)
	var rsp map[string]any
	err := plc.Get(map[string]any{
		`Command`:    string(vt),
		`Identifier`: identifier,
	}, &rsp)

	if err != nil {
		return nil, err
	}

	if status, ok := rsp[`Status`]; !ok || status != `Complete` {
		if errMsg, ok := rsp[`Error`].(string); ok && errMsg != `` {
			return nil, errors.New(errMsg)
		}
		return nil, errors.New(`BAD STATUS`)
	}

	return afc.New(x.Conn), nil
}

// AfcService opens the app's Documents directory via VendDocuments.
func (x *Service) AfcService(identifier string) (*afc.Service, error) {
	return x.vend(VendDocuments, identifier)
}

// ContainerService opens the app's full sandbox container via VendContainer.
func (x *Service) ContainerService(identifier string) (*afc.Service, error) {
	return x.vend(VendContainer, identifier)
}