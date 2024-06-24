package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/larryhou/gomobiledevice3/api/device"
	"github.com/larryhou/gomobiledevice3/api/remotepair"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"io"
	"log"
	"os"
)

func dump(msg any) {
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.SetEscapeHTML(false)
	j.Encode(msg)
}

func init() {
	log.SetFlags(log.LstdFlags)
}

func main() {
	r, err := rsd.New()
	if err != nil {panic(err)}

	_, err = remotepair.New(r)
	if err != nil {
		panic(err)
	}

	<-make(chan struct{})
}

func main5() {
	raw, _ := hex.DecodeString(`6000000000380001fe800000000000003e7d0afffe2543a1ff0200000000000000000000000000163a000100050200008f009fe30000000204000000ff0200000000000000000001ff2543a104000000ff0200000000000000000001ff000002`)

	pak := gopacket.NewPacket(raw, layers.LayerTypeIPv6, gopacket.Default)
	log.Printf(`%+v`, pak)
}

func main4() {
	r, err := rsd.New()
	if err != nil {panic(err)}

	json.NewEncoder(os.Stdout).Encode(r.Handshake)

	nc, err := r.StartLockdownService()
	if err != nil {panic(err)}

	log.Printf(`%+v`, nc.Descriptor)
}


func main3() {
	mux, err := usbmux.New()
	//mux.Listen(func(msg any) {
	//	fmt.Printf("%+v\n", msg)
	//})

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

	<-make(chan struct{})
}

func main2() {
	mux, err := usbmux.New()
	//mux.Listen(func(msg any) {
	//	fmt.Printf("%+v\n", msg)
	//})

	if err != nil {panic(err)} else {
		fmt.Printf("%s\n", mux.BUID)
		rsp, err := mux.ListDevices()
		fmt.Printf("%+v %v %s\n", rsp, err, mux.RemoteAddr())

		dev, err := device.New(mux, rsp.DeviceList[0])
		if err != nil {panic(err)}

		if afc, err := dev.AfcService(); err == nil {
			stat, err := afc.Stat(`DCIM/109APPLE/IMG_9081.MOV`)
			fmt.Printf("%+v %v\n", stat, err)

			//{
			//	h, err := afc.Open(`DCIM/109APPLE/IMG_9081.MOV`, `r`)
			//	if err != nil {panic(err)}
			//	r, err := h.FileReader()
			//	if err != nil {panic(err)}
			//	defer r.Close()
			//	w, err := os.OpenFile(`/Users/larryhou/Downloads/IMG_9081.MOV`, os.O_CREATE | os.O_TRUNC | os.O_WRONLY, 0644)
			//	fmt.Printf("%v %v\n", r, w)
			//	if err == nil {
			//		_, err = io.Copy(w, r)
			//		log.Printf(`READ %v`, err)
			//	}
			//
			//	if err != nil {panic(err)}
			//}

			{
				r, err := os.Open(`/Users/larryhou/Downloads/IMG_9081.MOV`)
				if err != nil {panic(err)}
				defer r.Close()
				info, _ := r.Stat()

				h, err := afc.Open(`DCIM/109APPLE/TEST.MOV`, `w`)
				if err != nil {panic(err)}
				w, err := h.FileWriter(info.Size())
				if err != nil {panic(err)}
				defer w.Close()

				if err == nil {
					_, err = io.Copy(w, r)
					log.Printf(`WRITE %v`, err)
				}

				if err != nil {panic(err)}

				stat, err := afc.Stat(`DCIM/109APPLE/TEST.MOV`)
				fmt.Printf("%+v %v\n", stat, err)

				err = afc.Remove(`DCIM/109APPLE/TEST.MOV`)
				fmt.Printf("RM %v\n", err)
			}


		}

		//if app, err := dev.ApplicationService(); err != nil {panic(err)} else {
		//	err := app.Uninstall(`com.microsoft.azure`)
		//	if err != nil {panic(err)}
		//}
	}
}