package main

import (
	"encoding/json"
	"fmt"
	"howett.net/plist"
	"os"
)

func main() {
	for _, name := range os.Args[1:] {
		fp, err := os.Open(name)
		if err != nil {panic(err)}
		var out any
		if err := plist.NewDecoder(fp).Decode(&out); err == nil {
			fmt.Printf("%+v\n", out)
			j := json.NewEncoder(os.Stdout)
			j.SetIndent(``, `    `)
			j.SetEscapeHTML(false)
			j.Encode(out)
		}
	}

	//plist.NewEncoder(os.Stdout).Encode(&usbmux.Message{
	//	MessageType: `Connect`,
	//})
}
