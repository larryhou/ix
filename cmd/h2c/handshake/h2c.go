package main

import (
	"encoding/json"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/device"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"log"
	"net/http"
	"os"
	"os/exec"

	_ "net/http/pprof"
)

func init() {
	log.SetFlags(log.LstdFlags)
}

func startTunnel() {
	mux, err := base.New()

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

	go http.ListenAndServe(`:11111`, nil)

	rs, err := rsd.New()
	if err != nil {panic(err)}
	log.Printf(`%+v`, rs.Descriptor)
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.Encode(rs.Descriptor)
}

func test() error {
	cmd := exec.Command(`python3`, `rc.py`)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}