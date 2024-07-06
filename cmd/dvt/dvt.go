package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/larryhou/iconsole/ns"
	"github.com/larryhou/j3idevice/api/dvt/applicationlisting"
	"github.com/larryhou/j3idevice/api/dvt/deviceinfo"
	"github.com/larryhou/j3idevice/api/dvt/location"
	"github.com/larryhou/j3idevice/api/dvt/notification"
	"github.com/larryhou/j3idevice/api/dvt/processctrl"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/dvt/screenshot"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"
)

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

	loc, err := location.New(r)
	if err != nil {panic(err)}
	err = loc.Simulate(30.6936195,107.254664)
	if err != nil {panic(err)}

	nf, err := notification.New(r)
	if err == nil {
		err = nf.Start()
	}

	//pc, err := processctrl.New(r)
	//if err != nil {panic(err)}
	//
	//pid, err := pc.Launch(`com.tencent.tmgp.dfm.db`, processctrl.LaunchContext{})
	//if err != nil {panic(err)}
	//
	//es, err := energy.New(r)
	//if err == nil {
	//	err = es.Start([]int{pid})
	//	if err != nil {panic(err)}
	//}

	<-make(chan struct{})
}

func main5() {
	opts := struct {
		udid string
	}{}

	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.Parse()
	rs, err := rsd.NewFromTunnelD(opts.udid)
	if err != nil {panic(err)}

	r, err := remotesvr.New(rs)
	if err != nil {panic(err)}

	si, err := deviceinfo.New(r)
	if err != nil {panic(err)}

	//{
	//	rsp, err := si.ReadDir(`/Applications/`)
	//	if err != nil {panic(err)}
	//	log.Printf(`LIST %+v`, rsp)
	//}

	//{
	//	rsp, err := si.GetProcName(0x35)
	//	log.Printf(`ProcName %#v %v`, rsp, err)
	//}

	//{
	//	rsp, _ := si.ListProcesses()
	//	//log.Printf(`Processes %#v %v`, rsp, err)
	//	json.NewEncoder(os.Stdout).Encode(rsp)
	//}

	//util.Print(si.SystemInfomation())
	//util.Print(si.HardwareInformation())
	//util.Print(si.NetworkInformation())
	//util.Print(si.MachTimeInfo())
	//util.Print(si.KpepDatabase())
	//util.Print(si.TraceCodesFile())

	_, err = si.SystemTap()
	if err != nil {panic(err)}
	<-make(chan struct{})
}

func main4() {
	opts := struct {
		udid string
	}{}

	flag.StringVar(&opts.udid, `udid`, `00008130-001975122140001C`, `idevice udid`)
	flag.Parse()
	rs, err := rsd.NewFromTunnelD(opts.udid)
	if err != nil {panic(err)}

	r, err := remotesvr.New(rs)
	if err != nil {panic(err)}

	ss, err := screenshot.New(r)
	if err != nil {panic(err)}

	img, err := ss.Capture()
	if err != nil {panic(err)}

	f, err := os.OpenFile(`screenshot.png`, os.O_CREATE | os.O_WRONLY | os.O_TRUNC, 0644)
	if err != nil {panic(err)}
	io.Copy(f, bytes.NewReader(img))
}

func main3() {
	f, err := os.Open(os.Args[1])
	if err != nil {panic(err)}
	defer f.Close()

	raw, _ := io.ReadAll(f)
	nka := ns.NewNSKeyedArchiver()
	out, err := nka.Unmarshal(raw)
	if err != nil {panic(err)}
	log.Printf(`%+v`, out)
}

func main2() {
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

