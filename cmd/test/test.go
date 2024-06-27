package main

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/device"
	"github.com/larryhou/j3idevice/api/remotepair"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/opencoff/go-srp"
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

func testHKDF() error {
	svr := `2a9d248dfb5ff124877ed4fbfa678461:31830180e741d6ea2345f05fd060b809fd4ee0d6fc46cd630057543e498dbe429b9cf46d1775d587cb3bc1c18c325001628140d060dadfc9d58d34f3d52eb6e401ce85ddbfe3ea18d027b8596cc1b1a47602e83d91bc60c4ae8bcdf3ea4a22466697187f3a45f28a9113b77b27f699b0e965f7ffac376214342eac2043e22465ed`
	s, err := srp.NewWithHash(crypto.SHA512, 3072)
	if err != nil {return err}

	c, err := s.NewClient([]byte(`Pair-Setup`), []byte(`000000`))
	if err != nil {return err}
	log.Printf(`CLIENT %s`, c.Credentials())

	proof, err := c.Generate(svr)
	if err != nil {return err}

	log.Printf(`PRF %s`, proof)
	log.Printf(`KEY %s`, hex.EncodeToString(c.RawKey()))
	return nil
}

func opack(data map[string]any) []byte {
	const (
		strBot = 0x61
		strOff = 0x40
		binBot = 0x91
		binOff = 0x70
	)

	num := func(n, i, p int, b io.ByteWriter) {
		switch {
		case n+i <= p:
			b.WriteByte(byte(n + i))
		case n <= 0xFF:
			b.WriteByte(byte(p))
			b.WriteByte(byte(n))
		case n <= 0xFFFF:
			b.WriteByte(byte(p + 1))
			b.WriteByte(byte(n >> 0 & 0xFF))
			b.WriteByte(byte(n >> 8 & 0xFF))
		}
	}

	buf := &bytes.Buffer{}
	buf.WriteByte(byte(len(data)) + 0xE0)
	for k, v := range data {
		num(len(k), strOff, strBot, buf)
		buf.WriteString(k)
		switch v := v.(type) {
		case string:
			num(len(v), strOff, strBot, buf)
			buf.WriteString(v)
		case []byte:
			num(len(v), binOff, binBot, buf)
			buf.Write(v)
		}
	}

	return buf.Bytes()
}

func main() {
	//log.Printf(`OPACK %s`, hex.EncodeToString(opack(map[string]any{
	//	`altIRK`:                      []byte("\xe9\xe8-\xc0jIykVoT\x00\x19\xb1\xc7{"),
	//	`btAddr`:                      `11:22:33:44:55:66`,
	//	`mac`:                         []byte("\x11\x22\x33\x44\x55\x66"),
	//	`remotepairing_serial_number`: `AAAAAAAAAAAA`,
	//	`accountID`:                   `26B8C60C-1F55-3848-AF27-A56856F296B7`,
	//	`model`:                       `computer-model`,
	//	`name`:                        `LARRYHOU-MC10`,
	//})))
	//testHKDF()
	//return
	log.Printf(remotepair.GROUP3072)
	r, err := rsd.New()
	if err != nil {panic(err)}

	rp, err := remotepair.New(r)
	if err != nil {
		<-make(chan struct{})
		panic(err)
	}
	json.NewEncoder(os.Stdout).Encode(rp.Descriptor)

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