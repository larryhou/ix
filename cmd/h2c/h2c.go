package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/device"
	"github.com/larryhou/gomobiledevice3/api/tunnel/h2c"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
	"net"
	"os"
	"os/exec"
)

func startTunnel() {
	mux, err := usbmux.New()

	if err != nil {panic(err)} else {
		fmt.Printf("%s\n", mux.BUID)
		rsp, err := mux.ListDevices()

		dev, err := device.New(mux, rsp.DeviceList[0])
		if err != nil {panic(err)}

		if tunnel, err := dev.TunnelService(); err == nil {
			log.Printf(`%v`, tunnel)
			if err != nil {panic(err)}
		} else {panic(err)}
	}
}

func main() {
	//startTunnel()

	err := rsd.Hijack(handshake)
	if err != nil {panic(err)}
	<-make(chan bool)
}

func test() error {
	cmd := exec.Command(`/usr/local/bin/python3.11`, `rc.py`)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func handshake() error {
	conn, err := net.Dial(`tcp`, `[fe80::fc5d:4ff:fecd:10a3%en6]:58783`)
	if err != nil {return err}

	log.Printf("%+v => %+v", conn.LocalAddr(), conn.RemoteAddr())

	client, err := h2c.NewConnection(conn)
	if err != nil {return err}

	s1, err := client.NewStream()
	if err != nil {return err}
	s1.Recv = Recv
	{
		buf := &bytes.Buffer{}
		xpc.Encode(buf, &xpc.Message{
			Payload: &xpc.Payload{
				Data: map[string]any{},
			},
		})

		s1.Send(buf, int64(buf.Len()))
	}

	{
		buf := &bytes.Buffer{}
		xpc.Encode(buf, &xpc.Message{
			Flag:    0x0201,
		})

		s1.Send(buf, int64(buf.Len()))
	}

	s3, err := client.NewStream()
	if err != nil {return err}
	s3.Recv = Recv

	{
		buf := &bytes.Buffer{}
		xpc.Encode(buf, &xpc.Message{
			Flag:    xpc.FlagInitHandshake,
		})

		s3.Send(buf, int64(buf.Len()))
	}
	//time.Sleep(time.Second)
	return nil
}

func Recv(b[]byte, f bool) error {
	msg := &xpc.Message{}
	err := xpc.Decode(bytes.NewReader(b), msg)
	var data any
	if msg.Payload != nil { data = msg.Data }
	log.Printf(`%+v %v %v #%d %s`, msg, data, f, len(b), hex.EncodeToString(b))
	return err
}
