package main

import (
	"flag"
	"github.com/larryhou/j3idevice/api/dvt/processctrl"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"
)

func main() {
	go http.ListenAndServe(`:11111`, nil)
	opts := struct {
		udid string
	}{}

	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.Parse()
	log.Printf(`#0`)
	rs, err := rsd.NewFromTunnelD(opts.udid)
	if err != nil {panic(err)}
	log.Printf(`#1`)

	lockd, err := rs.LockdownService()
	if err != nil {panic(err)}
	log.Printf(`#2`)

	log.Printf(`%+v`, lockd.Descriptor)
	log.Printf(`#3`)

	r, err := remotesvr.New(rs)
	if err != nil {panic(err)}
	log.Printf(`#4`)

	//log.Printf(`%p`, r)

	pc, err := processctrl.New(r)
	if err != nil {panic(err)}
	log.Printf(`#5`)

	pid, err := pc.Launch(`com.tencent.tmgp.dfm.db`, processctrl.LaunchContext{})
	if err != nil {panic(err)}
	log.Printf(`PID %d`, pid)
	time.Sleep(time.Second)
	err = pc.Signal(32508, 9)
	log.Printf(`%+v`, err)
}

