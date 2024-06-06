package main

import (
	"encoding/json"
	"fmt"
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
		{
			rsp, err := mux.ListDevices()
			fmt.Printf("%+v %v\n", rsp, err)
			//dump(rsp)
		}
	}
}
