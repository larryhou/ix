package main

import (
	"bytes"
	"crypto"
	"crypto/sha512"
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
	g, _ := srp2.NewGroup(remotepair.Group3072)
	c, err := srp2.NewClient(crypto.SHA512, g,`Pair-Setup`, `000000`)
	if err != nil {return err}

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

	priKey := set(`ephemeralPrivateKey`, `9a13e5c6c0bd477d271ebf49b05103f16df9245aef40982675b5d9dc9045644ee8ad6ce8b15565bdf85affb8d31aad971c1fef8d781b6f849d5a0c61c08c3a52c1dc2d92b061f994b3097155704a31faaaae8e7f888ca8de98bd78cbe9d3f8561d4b3ba7ad5c91f0691f601e8ccaf2bea5fe0c42fe578280c550226c10101da9`)

	A := new(big.Int)
	A.Exp(g.G, priKey, g.N)
	pubKey := set(`ephemeralPublicKey`, hex.EncodeToString(A.Bytes()))
	log.Printf(`CPRI %s`, hex.EncodeToString(priKey.Bytes()))
	log.Printf(`CKEY %s`, hex.EncodeToString(pubKey.Bytes()))

	skey, _ := hex.DecodeString(`6b065d1031b7ebc1237d61dc5913ecf9788324e34b1fd3984cba7ee63a38bec87a0caec4ae913adf7cfffa88c610ce402123976b0daca7c7828aad5eee08642655cc7e3beaedb72c834c0c68d420d96bf4882421bbf04a911e216b5a35bcf7f5afe7d9894152f824d22b56c897e02570c090ac5913e57f1ccb4b148c5940fd1e8f64feaa5ade30d7e2e4a96cfb2aa795dc04e6346787c505f382b18b259d59403c6924dc99652f853b42c00f5fc6e11ffd70de1fa23636188dfe509cae03e78d9a90527b91f063f570c4c2ba32adbadb101f9d707e979f219b99f012e50a533aaa0be1f2fb5a24f405104a40ebf9731b4bd6f3de9b308158c57069ed36b52ddf4eec7c80921dffab8acdb5514829ab4c9df5e8710e50e398c30d81395302e2d70084e1afb7f90e6f164df5e8f1a8ed0895a0f534ab4ac80c6edef82b3a6f55fe3fde3bc178061f7e5579ff89811a261553373331394bd19716e731a629dd90549daf74ee20d017f8f47faba1ba7ed42ec7d2f7222d32c8f2b2112bf34d9eba56`)
	salt, _ := hex.DecodeString(`4e0a90f838144aecff1a0d3b68b9cf01`)

	proof, err := c.ProveIdentity(new(big.Int).SetBytes(skey), string(salt))
	if err != nil {return err}
	log.Printf(`CPRF %s`, hex.EncodeToString(proof.Bytes()))
	log.Printf(`PWHS %s`, hex.EncodeToString(c.Secret.Bytes()))
	log.Printf(`u %s`, hex.EncodeToString(get(`u`).Bytes()))
	log.Printf(`k %s`, hex.EncodeToString(get(`k`).Bytes()))
	K := sha512.Sum512(c.PremasterKey.Bytes())
	log.Printf(`PK %s`, hex.EncodeToString(c.PremasterKey.Bytes()))
	log.Printf(`K %s`, hex.EncodeToString(K[:]))
	return nil
}

func count() {
	a := []byte{6,1,3,3,255,223,33,134,164,39,141,174,198,44,57,64,196,220,210,72,70,118,128,150,215,230,241,57,109,205,140,69,103,21,45,36,250,57,154,254,33,222,196,38,73,97,231,84,105,24,254,30,180,250,32,172,68,134,189,131,221,142,65,206,116,0,179,101,223,162,25,102,110,7,131,140,212,58,187,235,253,115,21,71,24,146,198,96,36,55,98,140,98,3,63,45,6,72,112,116,210,230,64,152,95,96,7,45,155,255,62,106,63,37,248,190,191,61,157,162,58,188,106,57,217,251,156,190,193,113,143,180,177,128,183,16,31,197,29,134,232,133,81,241,177,242,81,96,101,38,141,137,65,237,54,91,103,201,64,11,46,249,155,9,70,132,128,243,41,247,19,78,197,30,167,181,204,16,63,134,190,131,244,226,131,230,163,177,196,32,136,130,164,213,190,72,215,122,10,170,77,54,214,11,149,139,0,88,140,193,239,78,74,226,136,251,211,142,108,152,196,98,106,101,254,100,214,188,107,83,7,45,93,224,240,52,252,190,79,197,157,243,37,103,61,40,111,11,42,174,222,251,181,159,157,141,148,252,31,188,3,129,215,89,67,2,198,82,87,196,20,68,247,146,173,15,16,203,206,124,90,95,92,253,41,140,197,247,227,37,25,241,17,244,57,243,235,237,235,35,215,218,81,17,80,172,3,56,91,195,227,208,121,59,38,243,173,235,37,97,150,129,38,82,167,68,200,76,177,169,127,190,209,79,69,66,81,215,98,249,75,51,129,20,212,243,225,150,158,87,140,107,78,98,59,67,144,158,171,97,206,28,187,92,53,206,144,159,40,12,70,106,188,189,31,64,0,55,65,69,10,88,244,72,169,47,97,47,48,188,118,4,64,202,55,150,65,14,17,99,119,45,207,168,152,110,135,40,43,192,112,234,33,11,21,115,105,235,84,45,169,79,93,11,246,30,114,147,38,230,28,140,212,115,189,194,164,202,97,139,215,180,205,201,214,37,0,60,246,211,228,249,19,111,189,101,0}
	log.Printf(`#%d`, len(a))
}

func main() {
	//testSRP()
	//return
	log.Printf(remotepair.Group3072)
	r, err := rsd.New()
	if err != nil {panic(err)}

	rp, err := remotepair.New(r)
	if err != nil {
		log.Printf(`PAIRING %v`, err)
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