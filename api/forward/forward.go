package forward

import "github.com/larryhou/gomobiledevice3/api/usbmux"

func New(service *usbmux.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*usbmux.Service
}