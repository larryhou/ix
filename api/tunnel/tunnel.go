package tunnel

import "github.com/larryhou/gomobiledevice3/api/usbmux"

const (
	Port = 58783
)

const (
	ServiceName    = `com.apple.internal.devicecompute.CoreDeviceProxy`
)

type HandshakeRequest struct {
	Type string `json:"type"`
	Mtu  int    `json:"mtu"`
}

type HandshakeResponse struct {
	ServerRSDPort    int    `json:"serverRSDPort"`
	ServerAddress    string `json:"serverAddress"`
	Type             string `json:"type"`
	ClientParameters struct {
		Mtu     int    `json:"mtu"`
		Address string `json:"address"`
		Netmask string `json:"netmask"`
	} `json:"clientParameters"`
}

func New(service *usbmux.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*usbmux.Service
}