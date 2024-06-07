package usbmux

import (
	"errors"
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
	RequestQueryType    = `QueryType`
	RequestGetValue     = `GetValue`
	RequestStartSession = `StartSession`
	RequestStopSession  = `StopSession`
)

type Retcode interface {
	Verify() error
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

type DeviceDescriptor struct {
	DeviceID    int               `plist:"DeviceID"`
	MessageType string            `plist:"MessageType"`
	Properties  *DeviceProperties `plist:"Properties"`
}

func (x *DeviceDescriptor) String() string {
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
	DeviceList []*DeviceDescriptor `plist:"DeviceList"`
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
	Label      string `plist:"Label"`
	Request    string `plist:"Request"`
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
	Label     string `plist:"Label"`
	Request   string `plist:"Request"`
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
