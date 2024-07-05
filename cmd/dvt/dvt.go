package main

import (
	"encoding/json"
	"flag"
	"github.com/larryhou/iconsole/ns"
	"github.com/larryhou/j3idevice/api/dvt/applicationlisting"
	"github.com/larryhou/j3idevice/api/dvt/processctrl"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"
)

func main2() {
	f, err := os.Open(os.Args[1])
	if err != nil {panic(err)}
	defer f.Close()

	raw, _ := io.ReadAll(f)
	nka := ns.NewNSKeyedArchiver()
	out, err := nka.Unmarshal(raw)
	if err != nil {panic(err)}
	log.Printf(`%+v`, out)
}

func main() {
	opts := struct {
		udid string
	}{}

	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.Parse()
	rs, err := rsd.NewFromTunnelD(opts.udid)
	if err != nil {panic(err)}

	r, err := remotesvr.New(rs)
	if err != nil {panic(err)}

	al, err := applicationlisting.New(r)
	if err != nil {panic(err)}

	rsp, err := al.List()
	if err != nil {panic(err)}

	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.Encode(rsp)
}

func main1() {
	go http.ListenAndServe(`:11111`, nil)
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

	pc, err := processctrl.New(r)
	if err != nil {panic(err)}

	pid, err := pc.Launch(`com.tencent.tmgp.dfm.db`, processctrl.LaunchContext{})
	if err != nil {panic(err)}
	log.Printf(`PID %d`, pid)
	time.Sleep(time.Second*2)
	err = pc.Signal(32508, 9)
	log.Printf(`SIG %+v`, err)
	time.Sleep(time.Second*2)
	err = pc.Kill(pid)
	log.Printf(`KIL %d`, pid)
}

