package main

import (
	"encoding/json"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/device"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"os"
)

func dump(msg any) {
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.SetEscapeHTML(false)
	j.Encode(msg)
}

func main() {
	mux, err := usbmux.New()
	if err != nil {panic(err)} else {
		fmt.Printf("%s\n", mux.BUID)
		rsp, err := mux.ListDevices()
		fmt.Printf("%+v %v %s\n", rsp, err, mux.RemoteAddr())

		dev, err := device.New(mux, rsp.DeviceList[0])
		if err != nil {panic(err)}

		if app, err := dev.ApplicationService(); err != nil {panic(err)} else {
			data, err := app.List()
			fmt.Printf("%+v %v\n", data, err)
		}
	}
}
