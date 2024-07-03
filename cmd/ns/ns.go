package main

import (
	"encoding/hex"
	"github.com/larryhou/iconsole/ns"
	"github.com/larryhou/j3idevice/api/remote"
	"log"
	"os"
)

func main() {

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
		aux := &remote.MessageAux{}
		aux.AddU32(1)
		aux.AddU64(2)
		aux.AddObj(map[string]any{
			`name`: `larryhou`,
		})
		raw, err := aux.Bytes()
		if err != nil {panic(err)}
		log.Printf(`%s`, hex.EncodeToString(raw))
	}
}
