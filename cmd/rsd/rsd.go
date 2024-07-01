package main

import (
	"encoding/json"
	"fmt"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"log"
	"net"
	"os"
	"sort"
)

func main() {
	addr, err := net.ResolveTCPAddr(`tcp`, os.Args[1])
	if err != nil {panic(err)}

	rt, err := rsd.NewFromTunnel(&xpc.Context{Network:xpc.NetworkTCP, TCPAddr: addr})
	if err != nil {panic(err)}

	log.Printf(`%+v`, rt.Descriptor)
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.Encode(rt.Descriptor)

	var keys []string
	for name := range rt.Descriptor.Services {
		keys = append(keys, name)
	}

	sort.Slice(keys, func(i, j int) bool {
		vi := rt.Descriptor.Services[keys[i]]
		vj := rt.Descriptor.Services[keys[j]]
		if vi.Entitlement != vj.Entitlement {
			return vi.Entitlement < vj.Entitlement
		}

		return keys[i] < keys[j]
	})

	var group string

	for _, name := range keys {
		data := []byte(name)
		f := false
		p := 0
		for i := 0; i < len(data); i++ {
			c := data[i]
			C := c
			if C >= 'a' { C -= 32 }
			if i == 0 || f {
				data[p] = C
				f = false
			} else {
				switch c {
				case '.','_','-':
					f = true
					continue
				default:
					data[p] = c
				}
			}

			p++
		}
		svc := rt.Descriptor.Services[name]
		if len(group) == 0 || group != svc.Entitlement {
			group = svc.Entitlement
			fmt.Printf("\n// %s\n", group)
		}
		fmt.Printf("%s = `%s`\n", string(data[:p]), name)
	}
}
