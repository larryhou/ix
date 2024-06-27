package main

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	srp2 "github.com/fmitra/srp"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/device"
	"github.com/larryhou/j3idevice/api/remotepair"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/opencoff/go-srp"
	"io"
	"log"
	"math/big"
	"os"
	"reflect"
	"unsafe"
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

func testSRP() error {
	g, _ := srp2.NewGroup(remotepair.GROUP3072)
	c, err := srp2.NewClient(crypto.SHA512, g,`Pair-Setup`, `000000`)
	if err != nil {return err}

	log.Printf(`PADG %s`, hex.EncodeToString(c.Pad(g.G.Bytes())))

	rv := reflect.ValueOf(c).Elem()
	rt := rv.Type()

	get := func(name string) *big.Int {
		rf, _ := rt.FieldByName(name)
		return *(**big.Int)(unsafe.Pointer(uintptr(unsafe.Pointer(c))+rf.Offset))
	}

	set := func(name string, value string) *big.Int {
		b, _ := hex.DecodeString(value)
		return get(name).SetBytes(b)
	}

	priKey := set(`ephemeralPrivateKey`, `781b8dd23a15c2c67bf893ee335ec593b22117fa3251f1380f67d2df78b16a17e0cbb2ad4f103e263f6ca702389aed46f8158a537a026ccffc94ad7e9b38391e`)

	A := new(big.Int)
	A.Exp(g.G, priKey, g.N)
	pubKey := set(`ephemeralPublicKey`, hex.EncodeToString(A.Bytes()))
	log.Printf(`CPRI %s`, hex.EncodeToString(priKey.Bytes()))
	log.Printf(`CKEY %s`, hex.EncodeToString(pubKey.Bytes()))

	skey, _ := hex.DecodeString(`b06fbc51747050e9ab5af0843c1be8e96d984b4369668f5edb00fdceacc01ff622077781096e8430f585d9b3423abc884ee881d1c290274799168276f81f19a6e7f6ffd7f92fab56378357f004b556a974df3bc35924185cd12d5bfd12c213a33b697126a52af0931a23633fb983bb5bb314dc975246c97f08c1f8d191328c3c6c`)
	salt, _ := hex.DecodeString(`f4f1368f61e6d9ce8ebd130928d18f50`)

	proof, err := c.ProveIdentity(new(big.Int).SetBytes(skey), string(salt))
	if err != nil {return err}
	log.Printf(`CPRF %s`, hex.EncodeToString(proof.Bytes()))
	log.Printf(`PWHS %s`, hex.EncodeToString(c.Secret.Bytes()))
	log.Printf(`u %s`, hex.EncodeToString(get(`u`).Bytes()))
	log.Printf(`k %s`, hex.EncodeToString(get(`k`).Bytes()))
	//K := sha512.Sum512(c.PremasterKey.Bytes())
	log.Printf(`K %s`, hex.EncodeToString(c.PremasterKey.Bytes()))
	return nil
}

func main() {
	testSRP()
	return
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