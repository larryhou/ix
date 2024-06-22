package main

import (
	"encoding/json"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/device"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
	"os"
	"os/exec"
)

func init() {
	log.SetFlags(log.LstdFlags)
}

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

	rs, err := rsd.New()
	if err != nil {panic(err)}
	log.Printf(`%+v`, rs.Handshake)
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.Encode(rs.Handshake)
}

func test() error {
	cmd := exec.Command(`python3`, `rc.py`)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}