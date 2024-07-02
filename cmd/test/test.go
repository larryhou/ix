package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/google/gopacket/layers"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/device"
	"github.com/larryhou/j3idevice/api/remotepair"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"io"
	"log"
	"os"
)

func init() {
	log.SetFlags(log.LstdFlags)
}

func main() {
	addr, err := bonjour.TCPAddr(bonjour.RemotePairingServiceName)
	if err != nil {panic(err)}
	log.Printf(`%s`, addr.String())

	rp, err := remotepair.New(addr)
	if err != nil {
		panic(err)
	}

	err = rp.StartQuicTunnel()
	if err != nil {panic(err)}

	json.NewEncoder(os.Stdout).Encode(rp.Descriptor)
}

func main6() {
	r, err := rsd.BrowseRSD()
	if err != nil {panic(err)}

	rp, err := remotepair.NewFromRSD(r)
	if err != nil {
		panic(err)
	}

	err = rp.StartQuicTunnel()
	if err != nil {panic(err)}

	json.NewEncoder(os.Stdout).Encode(rp.Descriptor)
}

func main5() {
	mux, err := base.New()
	//mux.Listen(func(msg any) {
	//	fmt.Printf("%+v\n", msg)
	//})

	if err != nil {panic(err)} else {
		fmt.Printf("%s\n", mux.BUID)
		rsp, _ := mux.ListDevices()
		log.Printf(`%+v`, rsp)
	}

	<-make(chan struct{})
}

func main4() {
	r, err := rsd.BrowseRSD()
	if err != nil {panic(err)}

	json.NewEncoder(os.Stdout).Encode(r.Descriptor)
	nc, err := r.LockdownService()
	if err != nil {panic(err)}
	_ = layers.Loopback{

	}
	_ = tls.Config{

	}
	log.Printf(`%+v`, nc.Descriptor)
}


func main3() {
	mux, err := base.New()
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
	mux, err := base.New()
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