package tunneld

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/larryhou/j3idevice/api/base/usbmux"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/remotepair"
	"github.com/larryhou/j3idevice/api/tunnel"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/larryhou/zeroconf/v2"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
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

	svcs map[string]*remotepair.Service
	addr map[string]*remotepair.Service
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
			udid := msg[`Properties`].(map[string]any)[`SerialNumber`].(string)
			x.usb.Lock()
			x.usb.live[udid] = msg
			x.usb.udid[msg[`DeviceID`].(uint64)] = udid
			x.usb.Unlock()
		case `Detached`:
			x.usb.Lock()
			dvid := msg[`DeviceID`].(uint64)
			udid := x.usb.udid[dvid]
			delete(x.usb.live, udid)
			delete(x.usb.udid, dvid)
			x.usb.Unlock()

			x.RLock()
			rp, ok := x.svcs[udid]
			x.RUnlock()
			if ok {
				tun := rp.Tunnel()
				if tun != nil { tun.Stop() }
			}
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
	mux.Handle(`/rsd`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rsp := &response{Msg: `success`}
		defer x.json(w, rsp)

		var data []map[string]any

		x.RLock()
		defer x.RUnlock()
		for _, rp := range x.svcs {
			tun := rp.Tunnel()
			if tun == nil || tun.RSD == nil {continue}
			data = append(data, map[string]any{
				`Descriptor`: tun.RSD.Descriptor,
				`RSD`:        tun.RSD.TCPAddr.String(),
			})
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

		var tun *tunnel.Service
		x.RLock()
		defer x.RUnlock()
		if rp, ok := x.svcs[udid]; ok { tun = rp.Tunnel() }

		if tun != nil {
			rsp.Data = map[string]any{
				`Descriptor`: tun.RSD.Descriptor,
				`RSD`:        tun.RSD.TCPAddr.String(),
			}
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
	x.svcs = make(map[string]*remotepair.Service)
	x.addr = make(map[string]*remotepair.Service)

	go http.ListenAndServe(fmt.Sprintf(`:%d`, rsd.SvrPort), x.http())
	go x.listen()

	const domain = `local.`
	go func() error {
		go x.browse(true)
		return zeroconf.Browse(
			context.Background(),
			bonjour.RemotePairingServiceName,
			domain,
			x.data,
		)
	}()

	x.data = make(chan *zeroconf.ServiceEntry)
	defer close(x.data)

	go x.browse(false)
	return zeroconf.Browse(
		context.Background(),
		bonjour.RemotedServiceName,
		domain,
		x.data,
	)
}

func (x *daemon) browse(wifi bool) {
	for ent := range x.data {
		if len(ent.AddrIPv6) == 0 {continue}
		ifce, err := net.InterfaceByIndex(ent.IfIndex)
		if err != nil {continue}
		var ip net.IP
		switch {
		case len(ent.AddrIPv6) != 0: ip = ent.AddrIPv6[0]
		case len(ent.AddrIPv4) != 0: ip = ent.AddrIPv4[0]
		}

		addr := &net.TCPAddr{
			IP:   ip,
			Port: ent.Port,
			Zone: ifce.Name,
		}
		go func(addr *net.TCPAddr) {
			x.Lock()
			defer x.Unlock()
			_, ok := x.addr[addr.IP.String()]
			if ok {return}

			var rp *remotepair.Service
			if !wifi {
				r, err := rsd.New(addr)
				if err != nil {return}

				_, ok = x.svcs[r.Descriptor.Properties.UniqueDeviceID]
				if ok {return}

				rp, err = remotepair.NewFromRSD(r)
				if err != nil {return}
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

			if err != nil || rp == nil {return}

			udid := rp.Udid
			x.svcs[udid] = rp
			x.addr[ip.String()] = rp

			go func() {
				log.Printf(`%s START`, udid)
				rp.StartQuicTunnel()
				log.Printf(`%s STOP `, udid)

				x.Lock()
				delete(x.svcs, udid)
				delete(x.addr, addr.IP.String())
				x.Unlock()
			}()
		}(addr)
	}
}