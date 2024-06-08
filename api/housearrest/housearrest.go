package housearrest

import "github.com/larryhou/gomobiledevice3/api/usbmux"

const (
	ServiceName    = `com.apple.mobile.house_arrest`
	RSDServiceName = `com.apple.mobile.house_arrest.shim.remote`
)

func New(service *usbmux.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*usbmux.Service
}