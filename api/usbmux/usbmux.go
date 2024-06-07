package usbmux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"howett.net/plist"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
)

const (
	VersionName = `j3engine-usbmuxd-v1.0`
	ProgramName = `j3engine-idevice`
	LibVersion  = 3
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

func New() (*USBMux, error) {
	mux := &USBMux{ByteOrder: binary.LittleEndian}
	return mux, mux.Connect()
}

type USBMux struct {
	net.Conn
	binary.ByteOrder
	BUID string

	idx uint32
}

func (x *USBMux) Connect() error {
	conn, err := x.dial()
	if err != nil {return err}
	x.Conn = conn

	msg, err := x.ReadBUID()
	if err != nil {return err}
	x.BUID = msg.BUID
	log.Printf(`BUID %s`, x.BUID)
	return nil
}

func (x *USBMux) dial() (conn net.Conn, err error)  {
	if address := os.Getenv(`USBMUX_ADDRESS`); len(address) > 0 {
		switch {
		case strings.IndexByte(address, ':') > 0:
			return net.Dial(`tcp`, address)
		case strings.IndexByte(address, '/') > 0:
			return net.Dial(`unix`, address)
		}
	}

	switch runtime.GOOS {
	case `windows`:
		return net.Dial(`tcp`, `127.0.0.1:27015`)
	case `linux`,`darwin`:
		return net.Dial(`unix`, `/var/run/usbmuxd`)
	default:
		return nil, errors.New(`unsupported system: ` + runtime.GOOS)
	}
}

func (x *USBMux) Spawn() (*USBMux, error) {
	if x.Conn == nil {
		return nil, errors.New(`invalid usbmux connection`)
	}

	addr := x.Conn.RemoteAddr()
	conn, err := net.Dial(addr.Network(), addr.String())
	if err != nil {return nil, err}
	return &USBMux{
		BUID:      x.BUID,
		Conn:      conn,
		ByteOrder: x.ByteOrder,
	}, nil
}

func (x *USBMux) nextSeq() uint32 {
	x.idx++
	return x.idx
}

func (x *USBMux) Send(msg any) (uint32, error) {
	switch data := msg.(type) {
	case *ConnectRequest:
		data.KLibUSBMuxVersion = LibVersion
		data.ClientVersionString = VersionName
		data.ProgName = ProgramName
		data.MessageType = TypeConnect
	}

	rsv := make([]byte, 4)
	buf := &bytes.Buffer{}
	buf.Write(rsv)

	x.PutUint32(rsv, VerPlist)
	buf.Write(rsv)

	x.PutUint32(rsv, MsgPlist)
	buf.Write(rsv)

	seq := x.nextSeq()
	x.PutUint32(rsv, seq)
	buf.Write(rsv)

	err := plist.NewEncoder(buf).Encode(msg)
	if err != nil {return seq, err}

	x.PutUint32(rsv, uint32(buf.Len()))
	copy(buf.Bytes(), rsv)

	_, err = io.Copy(x.Conn, buf)
	return seq, err
}

func (x *USBMux) Recv(msg any, seq uint32) error {
	rsv := make([]byte, 4)
	if _, err := x.Read(rsv); err != nil {return err}

	num := x.Uint32(rsv)
	buf := make([]byte, num - 4)
	if _, err := x.Read(buf); err != nil {return err}

	//ver := x.Uint32(buf)
	//pro := x.Uint32(buf[4:8])
	if tag := x.Uint32(buf[8:12]); tag != seq {
		return fmt.Errorf(`seq echo mismatch: %d != %d`, tag, seq)
	}

	err := plist.NewDecoder(bytes.NewReader(buf[12:])).Decode(msg)
	if err == nil {
		if r, ok := msg.(Retcode); ok {
			err = r.Verify()
		}
	}

	return err
}

func (x *USBMux) Read(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		k, err := x.Conn.Read(b[t:])
		if err != nil {return 0, err}
		t += k
	}
	return n, nil
}

func (x *USBMux) Write(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		k, err := x.Conn.Write(b[t:])
		if err != nil {return 0, err}
		t += k
	}
	return n, nil
}

func (x *USBMux) ReadBUID() (*ReadBUIDResponse, error) {
	req := &ReadBUIDRequest{
		MessageType: TypeReadBUID,
	}

	seq, err := x.Send(req)
	if err != nil {return nil, err}

	rsp := &ReadBUIDResponse{}
	return rsp, x.Recv(rsp, seq)
}

func (x *USBMux) ListDevices() (*ListDevicesResponse, error) {
	req := &ListDevicesRequest{
		MessageType:         TypeListDevices,
		ClientVersionString: VersionName,
		ProgName:            ProgramName,
		KLibUSBMuxVersion:   LibVersion,
	}

	seq, err := x.Send(req)
	if err != nil {return nil, err}

	rsp := &ListDevicesResponse{}
	return rsp, x.Recv(rsp, seq)
}

func (x *USBMux) Get(req, rsp any) error {
	if seq, err := x.Send(req); err == nil {
		return x.Recv(rsp, seq)
	} else {
		return err
	}
}