package housearrest

import "github.com/larryhou/j3idevice/api/base"

const (
	ServiceName = `com.apple.mobile.house_arrest`
)

func New(service *base.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*base.Service
}