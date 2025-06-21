package main

import (
	"encoding/hex"
	"fmt"
	"github.com/larryhou/j3idevice/api/device"
	"github.com/larryhou/j3idevice/api/util"
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
	log.Printf(`%s %v %v %v`,
		hex.EncodeToString(device.VERSION_17_3_1[:]),
		device.VERSION_17_3_1.Compare(device.VERSION_17_0_0),
		device.VERSION_17_3_1.Compare(device.VERSION_17_4_0),
		device.VERSION_17_3_1.Compare(device.NewVersion(`16.4`)),
	)

	{
		if data, err := dev.ScreenShot(); err == nil {
			f, err := os.OpenFile(`test.png`, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {panic(err)}

			f.Write(data)
			f.Close()
		}
	}

	{
		util.Print(dev.ListApplications())
		//util.Print(dev.Launch(`com.tencent.tmgp.dfm.db`, processctrl.LaunchContext{}))
		return
	}

	has, err := dev.HouseArrestService()
	if err != nil {panic(err)}

	afcSvc, err := has.AfcService(`com.tencent.tmgp.dfm.db`)
	if err != nil {panic(err)}

	out, err := afcSvc.List(`/Documents`, true)
	if err != nil {panic(err)}
	for _, it := range out {
		log.Printf(`%s #%d`, it.Name, it.Size)
	}

	if false {
		h, err := afcSvc.Open(`/Documents/DeltaForce/Saved/Puffer/96e3fec9_fashionlong3-0-2-pakchunk119-iosclient.pak`, `r`)
		if err != nil {panic(err)}

		r, err := h.FileReader()
		if err != nil {panic(err)}

		f, err := os.OpenFile(`test.pak`, os.O_CREATE|os.O_WRONLY, 0777)
		if err != nil {panic(err)}

		io.Copy(f, r)
		f.Close()
		h.Close()
	}

	if false {
		h, err := afcSvc.Open(`/Documents/DeltaForce/Saved/Puffer/test111.pak`, `w`)
		if err != nil {panic(err)}

		i, _ := os.Stat(`test.pak`)

		w, err := h.FileWriter(i.Size())
		if err != nil {panic(err)}

		f, err := os.Open(`test.pak`)
		if err != nil {panic(err)}

		io.Copy(w, f)
		f.Close()
		h.Close()
	}

	return

	if afc, err := dev.AfcService(); err == nil {
		//stat, err := afc.Stat(`DCIM/109APPLE/IMG_9081.MOV`)
		//fmt.Printf("%+v %v\n", stat, err)
		//{
		//	out, err := afc.List(`Books`, true)
		//	if err != nil {panic(err)}
		//	for _, it := range out {
		//		log.Printf(`%s #%d`, it.Name, it.Size)
		//	}
		//}
		//
		//return

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