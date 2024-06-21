package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/device"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
	"net"
	"time"
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
	startTunnel()

	conn, err := net.Dial(`tcp`, `[fe80::fc5d:4ff:fecd:10a3%en6]:58783`)
	if err != nil {panic(err)}
	conn.(*net.TCPConn).SetNoDelay(true)
	time.Sleep(time.Millisecond)

	log.Printf("%+v => %+v", conn.LocalAddr(), conn.RemoteAddr())

	client, err := rsd.NewClient(conn)
	if err != nil {panic(err)}

	s1, err := client.NewStream()
	if err != nil {panic(err)}
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
	if err != nil {panic(err)}
	s3.Recv = Recv

	{
		buf := &bytes.Buffer{}
		xpc.Encode(buf, &xpc.Message{
			Flag:    xpc.FlagInitHandshake,
		})

		s3.Send(buf, int64(buf.Len()))
	}



	<-make(chan bool)
}

func Recv(b[]byte, f bool) error {
	log.Printf(`%s %v`, hex.EncodeToString(b), f)
	return nil
}
