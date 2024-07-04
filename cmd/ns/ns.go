package main

import (
	"encoding/hex"
	"flag"
	"github.com/larryhou/iconsole/ns"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"log"
	"os"
)

func main() {
	opts := struct {
		udid string
	}{}

	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.Parse()

	rs, err := rsd.NewFromTunnelD(opts.udid)
	if err != nil {panic(err)}

	lockd, err := rs.LockdownService()
	if err != nil {panic(err)}

	log.Printf(`%+v`, lockd.Descriptor)

	r, err := remotesvr.New(rs)
	if err != nil {panic(err)}

	log.Printf(`%p`, r)
}

func main2() {

	{
		nka := ns.NewNSKeyedArchiver()
		raw, err := nka.Marshal(map[string]any{
			`name`: `larryhou`,
		})
		if err != nil {panic(err)}

		{
			nka = ns.NewNSKeyedArchiver()
			v, _ := nka.Unmarshal(raw)
			log.Printf(`%+v`, v)
		}

		f, err := os.OpenFile(`test.plist`, os.O_CREATE | os.O_WRONLY | os.O_TRUNC, 0644)
		if err != nil {panic(err)}

		f.Write(raw)
		f.Close()
	}

	{
		aux := &remotesvr.MessageAux{}
		aux.AddU32(1)
		aux.AddU64(2)
		aux.AddObj(map[string]any{
			`name`: `larryhou`,
		})
		raw, err := aux.Encode()
		if err != nil {panic(err)}
		log.Printf(`%s`, hex.EncodeToString(raw))
	}
}
