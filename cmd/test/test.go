package main

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/device"
	"io"
	"log"
	"os"
)

func init() {
	log.SetFlags(log.LstdFlags)
}


func main3() {
	dev, err := device.New(device.Any)
	if err != nil {panic(err)}

	if tunnel, err := dev.StartCoreDeviceTunnelService(); err == nil {
		log.Printf(`%v`, tunnel)
		if err != nil {panic(err)}
	} else {panic(err)}

	<-make(chan struct{})
}

func main() {
	dev, err := device.New(device.Any)
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
			if err != nil {
				panic(err)
			}
			defer r.Close()
			info, _ := r.Stat()

			h, err := afc.Open(`DCIM/109APPLE/TEST.MOV`, `w`)
			if err != nil {
				panic(err)
			}
			defer h.Close()

			w, err := h.FileWriter(info.Size())
			if err != nil {
				panic(err)
			}

			if err == nil {
				_, err = io.Copy(w, r)
				log.Printf(`WRITE %v`, err)
			}

			if err != nil {
				panic(err)
			}

			stat, err := afc.Stat(`DCIM/109APPLE/TEST.MOV`)
			fmt.Printf("%+v %v\n", stat, err)

			err = afc.Remove(`DCIM/109APPLE/TEST.MOV`)
			fmt.Printf("RM %v\n", err)
		}
	}
}