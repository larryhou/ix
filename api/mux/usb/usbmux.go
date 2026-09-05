package usb

import (
	"fmt"
	"github.com/larryhou/ix/api/mux"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	VerBinary = 0
	VerPlist  = 1
)

const (
	ResultOk          = 0
	ResultBadCommand  = 1
	ResultBadDevice   = 2
	ResultConnRefused = 3
	ResultBadVersion  = 6
)

const (
	MsgResult  = 1
	MsgConnect = 2
	MsgListen  = 3
	MsgAdd     = 4
	MsgRemove  = 5
	MsgPaired  = 6
	MsgPlist   = 8
)

func New() (*UsbMux, error) {
	m := &UsbMux{}
	return m, m.Connect(``)
}

type UsbMux struct {
	*Connection
}

func (x *UsbMux) Connect(address string) error {
	if len(address) == 0 {
		address = os.Getenv(`USBMUX_ADDRESS`)
	}
	conn, err := x.connect(address)
	if err != nil {return err}
	x.Connection = NewConnection(conn)
	log.Printf(`CONNECT %s => %s`, conn.LocalAddr(), conn.RemoteAddr())
	return nil
}

func (x *UsbMux) connect(address string) (conn net.Conn, err error) {
	if len(address) > 0 {
		switch {
		case strings.ContainsRune(address, ':'):
			return net.Dial(`tcp`, address)
		case strings.ContainsRune(address, '/'):
			return net.Dial(`unix`, address)
		}
	}

	switch runtime.GOOS {
	case `windows`:
		return net.Dial(`tcp`, `127.0.0.1:27015`)
	case `linux`, `darwin`:
		return net.Dial(`unix`, `/var/run/usbmuxd`)
	default:
		return nil, fmt.Errorf(`unsupported system: %s`, runtime.GOOS)
	}
}

func (x *UsbMux) ReadBUID() (*mux.ReadBUIDResponse, error) {
	req := &mux.ReadBUIDRequest{
		MessageType: mux.TypeReadBUID,
	}

	seq, err := x.Send(req)
	if err != nil {return nil, err}

	rsp := &mux.ReadBUIDResponse{}
	return rsp, x.Recv(rsp, seq)
}

func (x *UsbMux) List() (*mux.ListDevicesResponse, error) {
	req := &mux.ListDevicesRequest{
		MessageType:         mux.TypeListDevices,
		ClientVersionString: mux.VersionName,
		ProgName:            mux.ProgramName,
		KLibUSBMuxVersion:   mux.MuxVersion,
	}

	seq, err := x.Send(req)
	if err != nil {return nil, err}

	rsp := &mux.ListDevicesResponse{}
	return rsp, x.Recv(rsp, seq)
}

func (x *UsbMux) Listen(handle func(msg map[string]any)) error {
	x.Conn.SetDeadline(time.Time{})
	type ListenRequest struct {
		ClientVersionString string `plist:"ClientVersionString"`
		MessageType         string `plist:"MessageType"`
		ProgName            string `plist:"ProgName"`
	}

	type ListenResponse mux.ConnectResponse

	req := &ListenRequest{
		ClientVersionString: mux.VersionName,
		ProgName:            mux.ProgramName,
		MessageType:         `Listen`,
	}

	rsp := &ListenResponse{}
	if err := x.Get(req, rsp); err != nil {
		return err
	}

	for {
		var msg map[string]any
		if err := x.Recv(&msg, 0); err == nil {
			go handle(msg)
		} else {
			if err == io.EOF {
				time.Sleep(time.Second)
				if err = x.Connect(``); err != nil {
					return err
				} else {
					return x.Listen(handle)
				}
			}
		}
	}
}
