package j3

import (
	"errors"
	"fmt"
)

const (
	VersionName = `j3engine-usbmuxd-v1.0`
	ProgramName = `j3engine-idevice`
	MuxVersion  = 3
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
	RequestQueryType    = `QueryType`
	RequestGetValue     = `GetValue`
	RequestStartSession = `StartSession`
	RequestStopSession  = `StopSession`
)

type Handle struct {
	DVID int
	UDID string
}

type Response struct {
	Error string `plist:"Error"`
}

func (x *Response) Verify() error {
	if len(x.Error) != 0 {
		return errors.New(x.Error)
	}

	return nil
}

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
	Response
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
	Response
	MessageType string `plist:"MessageType"`
	Number      int    `plist:"Number"`
}

type RequestRequest struct {
	Label   string `plist:"Label"`
	Request string `plist:"Request"`
}

type RequestResponse struct {
	Response
	Request string `plist:"Request"`
	Type    string `plist:"Type"`
}

type GetValueRequest RequestRequest

type GetValueResponse[T any] struct {
	Response
	Request string `plist:"Request"`
	Value   *T     `plist:"Value"`
}

type StartSessionRequest struct {
	RequestRequest
	HostID     string `plist:"HostID"`
	SystemBUID string `plist:"SystemBUID"`
}

type StartSessionResponse struct {
	Response
	EnableSessionSSL bool   `plist:"EnableSessionSSL"`
	Request          string `plist:"Request"`
	SessionID        string `plist:"SessionID"`
}

type StopSessionRequest struct {
	RequestRequest
	SessionID string `plist:"SessionID"`
}

type StopSessionResponse struct {
	Response
	Request string `plist:"Request"`
}

type KeyRequest struct {
	GetValueRequest
	Key string `plist:"Key"`
}

type KeyResponse GetValueResponse[any]

