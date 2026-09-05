package main

import (
	"encoding/json"
	"flag"
	"fmt"
	device2 "github.com/larryhou/ix/api/device"
	"github.com/larryhou/ix/api/tunnel/rsd"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
)

func main() {
	opts := struct {
		udid     string
		bundleid string
	}{}

	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.StringVar(&opts.bundleid, `bundleid`, `com.tencent.tmgp.dfm.db`, `application bundle id`)
	flag.Parse()

	device, err := device2.NewFromTunnelD(opts.udid)
	if err != nil {panic(err)}

	//log.Fatal(device.Heartbeat())

	log.Fatal(device.Logcat(os.Stdout))

	//r, err := rsd.NewFromTunnelD(opts.udid)
	//if err != nil {panic(err)}
	//
	//lds, err := r.LockdownService()
	//if err != nil {panic(err)}
	//log.Printf(`LOCKDOWN %+v`, lds.Descriptor)
	//
	//ha, err := housearrest.NewFromRSD(r)
	//if err != nil {panic(err)}
	//
	//afc, err := ha.AfcService(opts.bundleid, housearrest.VendDocuments)
	//if err != nil {panic(err)}
	//
	//items, err := afc.List(`/Documents/`, true)
	//if err != nil {panic(err)}
	//
	//for _, it := range items {
	//	log.Printf(`%s #%d`, it.Name, it.Size)
	//}
}

func main1() {
	opts := struct {
		udid string
	}{}
	
	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.Parse()

	rsp, err := http.Get(fmt.Sprintf(`http://127.0.0.1:%d/rsd/%s`, rsd.SvrPort, opts.udid))
	if err != nil {panic(err)}
	var data map[string]any
	err = json.NewDecoder(rsp.Body).Decode(&data)
	if err != nil {panic(err)}

	addr, err := net.ResolveTCPAddr(`tcp`, data[`Data`].(map[string]any)[`RSD`].(string))
	if err != nil {panic(err)}

	rt, err := rsd.NewFromTunnel(addr)
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
		fmt.Printf("%s = `%s`  // xpc:%v\n", string(data[:p]), name, svc.Properties.UsesRemoteXPC)
	}
}
