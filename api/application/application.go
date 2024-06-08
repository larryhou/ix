package application

import (
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
)

const (
	ServiceName    = `com.apple.mobile.installation_proxy`
	RSDServiceName = `com.apple.mobile.installation_proxy.shim.remote`
)

const (
	TypeAny = `Any`
)

const (
	CommandLookup    = `Lookup`
	CommandUninstall = `Uninstall`
)

func New(service *usbmux.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*usbmux.Service
}

func (x *Service) List(opaque bool) (any, error) {
	req := &ListRequest{
		Command: CommandLookup,
		ClientOptions: &ClientOptions{
			ApplicationType: TypeAny,
		},
	}

	if !opaque {
		rsp := &ListResponse{}
		return rsp, x.Get(req, rsp)
	} else {
		var rsp any
		return rsp, x.Get(req, &rsp)
	}
}

func (x *Service) Uninstall(identifier string) error {
	req := &UninstallRequest{
		Command: CommandUninstall,
		ClientOptions: &ClientOptions{
			ApplicationIdentifier: identifier,
		},
	}

	const success = `Complete`
	rsp := &UninstallResponse{}
	for rsp.Status != success {
		if err := x.Get(req, rsp); err != nil {
			return err
		}

		if rsp.Status == success {
			rsp.PercentComplete = 100
		}

		log.Printf(`Uninstall %s[%s]: %d%%`, identifier, rsp.Status, rsp.PercentComplete)
	}

	return nil
}