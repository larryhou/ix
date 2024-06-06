package usbmux

import (
	"fmt"
)

const (
	TypeReadBUID       = `ReadBUID`
	TypeListDevices    = `ListDevices`
	TypeConnect        = `Connect`
	TypeResult         = `Result`
	TypeReadPairRecord = `ReadPairRecord`
	TypeAttached       = `Attached`
)

const (
	RequestQueryType = `QueryType`
	RequestGetValue  = `GetValue`
)

type ReadBUIDRequest struct {
	MessageType string `plist:"MessageType"`
}

type ReadBUIDResponse struct {
	BUID string `plist:"BUID"`
}

type DeviceProperties struct {
	ConnectionSpeed int    `plist:"ConnectionSpeed"`
	ConnectionType  string `plist:"ConnectionType"`
	DeviceID        int    `plist:"DeviceID"`
	LocationID      int    `plist:"LocationID"`
	ProductID       int    `plist:"ProductID"`
	SerialNumber    string `plist:"SerialNumber"`
	USBSerialNumber string `plist:"USBSerialNumber"`
}

type Device struct {
	DeviceID    int               `plist:"DeviceID"`
	MessageType string            `plist:"MessageType"`
	Properties  *DeviceProperties `plist:"Properties"`
}

func (x *Device) String() string {
	return fmt.Sprintf(`%d %s %s %d %s %d`, x.DeviceID, x.MessageType, x.Properties.ConnectionType, x.Properties.ProductID, x.Properties.SerialNumber, x.Properties.ConnectionSpeed)
}

type ListDevicesRequest struct {
	ClientVersionString string `plist:"ClientVersionString"`
	MessageType         string `plist:"MessageType"`
	ProgName            string `plist:"ProgName"`
	KLibUSBMuxVersion   int    `plist:"kLibUSBMuxVersion"`
}

type ListDevicesResponse struct {
	DeviceList []*Device `plist:"DeviceList"`
}

type ConnectRequest struct {
	ClientVersionString string `plist:"ClientVersionString"`
	DeviceID            int    `plist:"DeviceID"`
	MessageType         string `plist:"MessageType"`
	PortNumber          int    `plist:"PortNumber"`
	ProgName            string `plist:"ProgName"`
	KLibUSBMuxVersion   int    `plist:"kLibUSBMuxVersion"`
}

type ConnectResponse struct {
	MessageType string `plist:"MessageType"`
	Number      int    `plist:"Number"`
}

type RequestRequest struct {
	Label   string `plist:"Label"`
	Request string `plist:"Request"`
}

type RequestResponse struct {
	Request string `plist:"Request"`
	Type    string `plist:"Type"`
}

type GetValueRequest RequestRequest

type GetValueResponse[T any] struct {
	Request string `plist:"Request"`
	Value   *T     `plist:"Value"`
}

type StartSessionRequest struct {
	RequestRequest
	HostID     string `plist:"HostID"`
	SystemBUID string `plist:"SystemBUID"`
}

type StartSessionResponse struct {
	EnableSessionSSL bool   `plist:"EnableSessionSSL"`
	Request          string `plist:"Request"`
	SessionID        string `plist:"SessionID"`
}

type ReadPairRecordRequest struct {
	ClientVersionString string `plist:"ClientVersionString"`
	MessageType         string `plist:"MessageType"`
	PairRecordID        string `plist:"PairRecordID"`
	ProgName            string `plist:"ProgName"`
	KLibUSBMuxVersion   int    `plist:"kLibUSBMuxVersion"`
}

type ReadPairRecordResponse struct {
	PairRecordData []byte `plist:"PairRecordData"`
}

type PairRecord struct {
	DeviceCertificate []byte `plist:"DeviceCertificate"`
	DevicePublicKey   []byte `plist:"DevicePublicKey"`
	EscrowBag         []byte `plist:"EscrowBag"`
	HostCertificate   []byte `plist:"HostCertificate"`
	HostID            string `plist:"HostID"`
	HostPrivateKey    []byte `plist:"HostPrivateKey"`
	RootCertificate   []byte `plist:"RootCertificate"`
	RootPrivateKey    []byte `plist:"RootPrivateKey"`
	SystemBUID        string `plist:"SystemBUID"`
	WiFiMACAddress    string `plist:"WiFiMACAddress"`
}