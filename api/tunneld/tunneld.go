package tunneld

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/larryhou/j3idevice/api/base"
	"github.com/larryhou/j3idevice/api/bonjour"
	"github.com/larryhou/j3idevice/api/remotepair"
	"github.com/larryhou/j3idevice/api/tunnel/rsd"
	"github.com/larryhou/zeroconf/v2"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

func init() {
	log.SetFlags(log.LstdFlags)
}

func Run() error {
	return (&daemon{}).start()
}

type Response struct {
	Ret  int    `json:"Ret"`
	Msg  string `json:"Msg"`
	Data any    `json:"Data,omitempty"`
}

type daemon struct {
	data chan *zeroconf.ServiceEntry
	svcs map[string]*remotepair.Service
	addr map[string]*rsd.Service
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

	c, err := base.New()
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
		rsp := &Response{Msg: `success`}
		defer x.json(w, rsp)

		var data []map[string]any

		x.RLock()
		for _,rp := range x.svcs {
			tun := rp.Tunnel()
			if tun == nil || tun.RSD == nil {continue}
			data = append(data, map[string]any{
				`Descriptor`: tun.RSD.Descriptor,
				`RSD`:        tun.RSD.TCPAddr.String(),
			})
		}
		x.RUnlock()

		if len(data) == 0 {
			rsp.Msg = `No running tunnels`
			rsp.Ret = http.StatusNotFound
		} else {
			rsp.Data = data
		}
	}))
	mux.Handle(`/rsd/`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		udid := r.URL.Path[5:]
		rsp := &Response{Msg: `success`}
		defer x.json(w, rsp)

		var rp *remotepair.Service
		x.RLock()
		rp = x.svcs[udid]
		x.RUnlock()

		tun := rp.Tunnel()
		if rp == nil {
			rsp.Ret = http.StatusNotFound
			rsp.Msg = fmt.Sprintf(`No rsd found with %s`, udid)
		} else {
			rsp.Data = map[string]any{
				`Descriptor`: tun.RSD.Descriptor,
				`RSD`:        tun.RSD.TCPAddr.String(),
			}
		}
	}))

	mux.Handle(`/`, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	return mux
}

func (x *daemon) start() error {
	x.svcs = make(map[string]*remotepair.Service)
	x.addr = make(map[string]*rsd.Service)
	x.data = make(chan *zeroconf.ServiceEntry)
	defer close(x.data)
	go http.ListenAndServe(fmt.Sprintf(`:%d`, rsd.SvrPort), x.http())
	go x.listen()
	go x.browse()

	const domain = `local.`
	return zeroconf.Browse(
		context.Background(),
		bonjour.RemotePairingServiceName,
		domain,
		x.data,
	)
}

func (x *daemon) browse() {
	for ent := range x.data {
		if len(ent.AddrIPv6) == 0 {continue}
		ifce, err := net.InterfaceByIndex(ent.IfIndex)
		if err != nil {continue}
		var ip net.IP
		switch {
		case len(ent.AddrIPv4) != 0: ip = ent.AddrIPv4[0]
		case len(ent.AddrIPv6) != 0: ip = ent.AddrIPv6[0]
		}

		r, err := rsd.New(&net.TCPAddr{
			IP:   ip,
			Zone: ifce.Name,
		})

		if err == nil {
			x.RLock()
			_, ok := x.svcs[r.Descriptor.Properties.UniqueDeviceID]
			x.RUnlock()
			if ok {continue}
		}

		go func(r *rsd.Service, addr *net.TCPAddr) {
			var rp *remotepair.Service
			var err error

			x.Lock()
			if r == nil {
				rp, err = remotepair.New(addr)
			} else {
				rp, err = remotepair.NewFromRSD(r)
			}
			if err != nil {
				x.Unlock()
				return
			}

			udid := rp.Descriptor.PeerDeviceInfo.Udid
			x.svcs[udid] = rp
			x.Unlock()

			log.Printf(`%s START`, udid)
			rp.StartQuicTunnel()
			log.Printf(`%s STOP `, udid)

			x.Lock()
			delete(x.svcs, udid)
			x.Unlock()
		}(r, &net.TCPAddr{
			IP:   ip,
			Port: ent.Port,
			Zone: ifce.Name,
		})
	}
}