package main

import (
	"encoding/json"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"log"
	"net"
	"os"
)

func main() {
	address := `[fd38:7634:f0a0::1%utun10]:56125`
	addr, err := net.ResolveTCPAddr(`tcp`, address)
	if err != nil {panic(err)}

	rt, err := rsd.NewFromTunnel(addr)
	if err != nil {panic(err)}

	log.Printf(`%+v`, rt.Descriptor)
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.Encode(rt.Descriptor)
}
