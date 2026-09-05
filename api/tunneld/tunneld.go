package tunneld

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/j3"
	"github.com/larryhou/j3idevice/api/j3/usbmux"
	"github.com/larryhou/j3idevice/api/lockdown"
	"github.com/larryhou/j3idevice/api/remotepair"
	"github.com/larryhou/j3idevice/api/tunnel"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/larryhou/zeroconf/v2"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"sync"
	"time"
)

func init() {
	log.SetFlags(log.LstdFlags)
}

func Run() error {
	go http.ListenAndServe(fmt.Sprintf(`:%d`, rsd.SvrPort+1), nil)
	return (&daemon{}).start()
}

type response struct {
	Ret  int    `json:"Ret"`
	Msg  string `json:"Msg"`
	Data any    `json:"Data,omitempty"`
}

type daemon struct {
	data chan *zeroconf.ServiceEntry

	svcs    map[string]*remotepair.Service  // WiFi/RSD 路径
	addr    map[string]*remotepair.Service
	usbTuns map[string]*tunnel.Service      // USB CoreDeviceProxy 路径
	sync.RWMutex

	usb struct {
		live map[string]any
		udid map[uint64]string
		sync.Mutex
	}
}

func (x *daemon) listen() error {
	x.usb.live = make(map[string]any)
	x.usb.udid = make(map[uint64]string)

	c, err := usbmux.New()
	if err != nil {return err}
	return c.Listen(func(msg map[string]any) {
		switch msg[`MessageType`] {
		case `Attached`:
			props := msg[`Properties`].(map[string]any)
			udid  := props[`SerialNumber`].(string)
			dvid  := int(msg[`DeviceID`].(uint64))
			x.usb.Lock()
			x.usb.live[udid] = msg
			x.usb.udid[uint64(dvid)] = udid
			x.usb.Unlock()
			go x.tryConnectUSB(udid, dvid)
		case `Detached`:
			x.usb.Lock()
			dvid := msg[`DeviceID`].(uint64)
			udid := x.usb.udid[dvid]
			delete(x.usb.live, udid)
			delete(x.usb.udid, dvid)
			x.usb.Unlock()

			// 停止 WiFi 路径的 tunnel
			x.RLock()
			rp, ok := x.svcs[udid]
			x.RUnlock()
			if ok {
				if tun := rp.Tunnel(); tun != nil { tun.Stop() }
			}

			// 停止 USB 路径的 tunnel
			x.Lock()
			if tun, ok := x.usbTuns[udid]; ok {
				tun.Stop()
				delete(x.usbTuns, udid)
			}
			x.Unlock()
		}
		log.Printf(`USB %+v`, msg)
	})
}

func (x *daemon) json(w io.Writer, msg any) {
	j := json.NewEncoder(w)
	j.SetIndent(``, `    `)
	j.SetEscapeHTML(false)
	j.Encode(msg)
}

func (x *daemon) http() *http.ServeMux {
	mux := http.NewServeMux()
	tunInfo := func(tun *tunnel.Service) map[string]any {
		return map[string]any{
			`Descriptor`: tun.RSD.Descriptor,
			`RSD`:        tun.RSD.TCPAddr.String(),
		}
	}

	mux.Handle(`/rsd`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rsp := &response{Msg: `success`}
		defer x.json(w, rsp)

		var data []map[string]any

		x.RLock()
		defer x.RUnlock()
		for _, rp := range x.svcs {
			tun := rp.Tunnel()
			if tun == nil || tun.RSD == nil {continue}
			data = append(data, tunInfo(tun))
		}
		for _, tun := range x.usbTuns {
			if tun.RSD == nil {continue}
			data = append(data, tunInfo(tun))
		}

		if len(data) == 0 {
			rsp.Msg = `No running tunnels`
			rsp.Ret = http.StatusNotFound
		} else {
			rsp.Data = data
		}
	}))
	mux.Handle(`/rsd/`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		udid := r.URL.Path[5:]
		rsp := &response{Msg: `success`}
		defer x.json(w, rsp)

		x.RLock()
		defer x.RUnlock()

		var tun *tunnel.Service
		if rp, ok := x.svcs[udid]; ok { tun = rp.Tunnel() }
		if tun == nil { tun = x.usbTuns[udid] }

		if tun != nil && tun.RSD != nil {
			rsp.Data = tunInfo(tun)
		} else {
			rsp.Ret = http.StatusNotFound
			rsp.Msg = fmt.Sprintf(`No rsd found with %s`, udid)
		}
	}))

	mux.Handle(`/`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	return mux
}

func (x *daemon) start() error {
	x.svcs    = make(map[string]*remotepair.Service)
	x.addr    = make(map[string]*remotepair.Service)
	x.usbTuns = make(map[string]*tunnel.Service)

	go http.ListenAndServe(fmt.Sprintf(`:%d`, rsd.SvrPort), x.http())
	go x.listen()

	const interval = time.Second * 2
	update := zeroconf.SelectInterval(interval)

	x.data = make(chan *zeroconf.ServiceEntry)
	defer close(x.data)
	go x.browse()

	const domain = `local.`
	go func() error {
		time.Sleep(interval>>1)
		return zeroconf.Browse(
			context.Background(),
			bonjour.RemotePairingServiceName,
			domain,
			x.data,
			update,
		)
	}()

	return zeroconf.Browse(
		context.Background(),
		bonjour.RemotedServiceName,
		domain,
		x.data,
		update,
	)
}

var (
	pass = errors.New(`PASS ACTIVE`)
)

func (x *daemon) tryConnect(addr *net.TCPAddr, remotep bool) (err error) {
	x.Lock()
	defer x.Unlock()

	_, ok := x.addr[addr.IP.String()]
	if ok {return pass}
	var rp *remotepair.Service
	if !remotep {
		r, err := rsd.New(addr)
		if err != nil {return err}
		_, ok = x.svcs[r.Descriptor.Properties.UniqueDeviceID]
		if ok {return pass}
		rp, err = remotepair.NewFromRSD(r)
		if err != nil {return err}
	} else {
		for _, udid := range remotepair.ListUdid() {
			udid := udid
			if _, ok := x.svcs[udid]; ok {continue}
			rp, err = remotepair.New(addr,
				remotepair.PairTypeWiFi,
				func(s *remotepair.Service) {
					s.Udid = udid
				},
			)
			if err == nil {break}
		}
	}

	if err != nil || rp == nil {
		return
	}

	udid := rp.Udid
	x.svcs[udid] = rp
	x.addr[addr.IP.String()] = rp

	go func() {
		log.Printf(`%s START quic`, udid)
		err := rp.StartQuicTunnel()
		if err != nil {
			log.Printf(`%s QUIC FAILED %+v, fallback to TCP`, udid, err)
			err = rp.StartTcpTunnel()
		}
		log.Printf(`%s STOP %+v`, udid, err)

		x.Lock()
		delete(x.svcs, udid)
		delete(x.addr, addr.IP.String())
		x.Unlock()
	}()

	return
}

// tryConnectUSB mirrors pymobiledevice3's CoreDeviceTunnelProxy path:
//   lockdown.StartService(CoreDeviceProxy) → CDTunnel handshake → TUN interface
// No RemotePairing/PSK involved — the lockdown pairing trust is sufficient.
func (x *daemon) tryConnectUSB(udid string, dvid int) {
	x.RLock()
	_, active := x.usbTuns[udid]
	x.RUnlock()
	if active {
		log.Printf(`USB %s already active`, udid)
		return
	}

	mux, err := usbmux.New()
	if err != nil {
		log.Printf(`USB %s usbmux: %v`, udid, err)
		return
	}

	handle := &j3.Handle{UDID: udid, DVID: dvid}
	lockd, err := lockdown.New(mux, handle)
	if err != nil {
		log.Printf(`USB %s lockdown: %v`, udid, err)
		return
	}

	svc, err := lockd.StartService(rsd.ComAppleInternalDevicecomputeCoreDeviceProxy)
	if err != nil {
		log.Printf(`USB %s start CoreDeviceProxy: %v`, udid, err)
		return
	}

	tun, err := tunnel.New(svc, tunnel.MtuTcp, context.Background())
	if err != nil {
		log.Printf(`USB %s tunnel.New: %v`, udid, err)
		return
	}

	x.Lock()
	if _, active = x.usbTuns[udid]; active {
		x.Unlock()
		tun.Stop()
		return
	}
	x.usbTuns[udid] = tun
	x.Unlock()

	log.Printf(`USB %s tunnel START`, udid)
	if err = tun.Start(svc); err != nil {
		log.Printf(`USB %s tunnel STOP: %v`, udid, err)
	}

	x.Lock()
	delete(x.usbTuns, udid)
	x.Unlock()
}

func (x *daemon) browse() {
	for ent := range x.data {

		ifce, err := net.InterfaceByIndex(ent.IfIndex)
		if err != nil {continue}

		var ip net.IP
		switch {
		case len(ent.AddrIPv6) != 0: ip = ent.AddrIPv6[0]
		case len(ent.AddrIPv4) != 0: ip = ent.AddrIPv4[0]
		}

		go x.tryConnect(&net.TCPAddr{
			IP:   ip,
			Port: ent.Port,
			Zone: ifce.Name,
		}, strings.HasSuffix(ent.Service, bonjour.RemotePairingServiceName))
		time.Sleep(time.Second * 5)
	}
}