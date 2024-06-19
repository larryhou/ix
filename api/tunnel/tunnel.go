package tunnel

import (
	"bytes"
	"encoding/json"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"github.com/songgao/water"
	"io"
	"log"
)

const (
	Port = 58783
	Mtu  = 16000
)

const (
	ServiceName = `com.apple.internal.devicecompute.CoreDeviceProxy`
	Magic       = `CDTunnel`
)

const (
	TypeClientHandshakeRequest = `clientHandshakeRequest`
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

type Handshake struct {
	ServerRSDPort    int    `json:"serverRSDPort"`
	ServerAddress    string `json:"serverAddress"`
	Type             string `json:"type"`
	ClientParameters struct {
		Mtu     int    `json:"mtu"`
		Address string `json:"address"`
		Netmask string `json:"netmask"`
	} `json:"clientParameters"`
}

func New(service *usbmux.Service) (*Service, error) {
	s := &Service{Service: service}
	err := s.handshake()
	if err == nil {
		err = s.start()
	}

	return s, err
}

type Service struct {
	*usbmux.Service
	*Handshake
}

func (x *Service) Send(msg any) error {
	num := make([]byte, 2)
	buf := &bytes.Buffer{}
	buf.WriteString(Magic)
	buf.Write(num)
	err := json.NewEncoder(buf).Encode(msg)
	if err == nil {
		k := len(Magic)
		x.ByteOrder.PutUint16(buf.Bytes()[k:], uint16(buf.Len()-k-2))
		_, err = io.Copy(x.Conn, buf)
	}

	return err
}

func (x *Service) Recv(msg any) error {
	rsv := make([]byte, len(Magic))
	_, err := x.Read(rsv)
	if err == nil {
		_, err = x.Read(rsv[:2])
	}

	buf := &bytes.Buffer{}
	if err == nil {
		num := x.ByteOrder.Uint16(rsv)
		_, err = io.Copy(buf, io.LimitReader(x.Conn, int64(num)))
	}

	if err == nil {
		err = json.NewDecoder(buf).Decode(msg)
	}

	return err
}

func (x *Service) handshake() error {
	err := x.Send(map[string]any{
		`type`: TypeClientHandshakeRequest,
		`mtu`:  Mtu,
	})

	rsp := &Handshake{}
	if err == nil {
		err = x.Recv(rsp)
	}

	if err == nil {
		x.Handshake = rsp
	}

	return err
}

func (x *Service) start() error {
	config := water.Config{
		DeviceType: water.TUN,
	}

	ifce, err := water.New(config)
	if err != nil { return err }
	packet := make([]byte, 2000)
	for {
		n, err := ifce.Read(packet)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Packet Received: % x\n", packet[:n])
	}
}

