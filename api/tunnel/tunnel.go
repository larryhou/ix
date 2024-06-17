package tunnel

import "github.com/larryhou/gomobiledevice3/api/usbmux"

const (
	Port = 58783
)

const (
	ServiceName    = `com.apple.internal.devicecompute.CoreDeviceProxy`
)

/**
com.apple.fusion.remote.service/com.apple.fusion.remote.service
com.apple.gputools.remote.agent/com.apple.private.gputoolstransportd
com.apple.internal.dt.coredevice.untrusted.tunnelservice/com.apple.dt.coredevice.tunnelservice.client
com.apple.mobile.insecure_notification_proxy.remote/com.apple.mobile.insecure_notification_proxy.remote
com.apple.mobile.insecure_notification_proxy.shim.remote/com.apple.mobile.lockdown.remote.untrusted
com.apple.mobile.lockdown.remote.untrusted/com.apple.mobile.lockdown.remote.untrusted
com.apple.osanalytics.logTransfer/com.apple.ReportCrash.antenna-access
 */

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