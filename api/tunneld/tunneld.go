package tunneld

import (
	"context"
	"encoding/json"
	"fmt"
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

func Launch() error {
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
	sync.RWMutex
}

func (x *daemon) json(w io.Writer, msg any) {
	j := json.NewEncoder(w)
	j.SetIndent(``, `    `)
	j.SetEscapeHTML(false)
	j.Encode(msg)
}

func (x *daemon) http() *http.ServeMux {
	mux := http.NewServeMux()
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
				`RSD`:        tun.Addr.String(),
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
	x.data = make(chan *zeroconf.ServiceEntry)
	defer close(x.data)

	go http.ListenAndServe(`:33333`, x.http())
	go x.browse()

	const domain = `local.`
	return zeroconf.Browse(
		context.Background(),
		bonjour.RemotedServiceName,
		domain,
		x.data,
		zeroconf.SelectIPTraffic(zeroconf.IPv6),
	)
}

func (x *daemon) browse() {
	for ent := range x.data {
		if len(ent.AddrIPv6) == 0 {continue}
		ifce, err := net.InterfaceByIndex(ent.IfIndex)
		if err != nil {continue}

		addr := &net.TCPAddr{
			IP:   ent.AddrIPv6[0],
			Port: (ent.Port&0xFF00)>>8 | (ent.Port&0x00F)<<8,
			Zone: ifce.Name,
		}

		r, err := rsd.New(addr)
		if err != nil {continue}

		x.RLock()
		_, ok := x.svcs[r.Descriptor.Properties.UniqueDeviceID]
		x.RUnlock()
		if ok {continue}

		rp, err := remotepair.New(r)
		if err != nil {return}

		go func(rp *remotepair.Service) {
			udid := rp.Descriptor.PeerDeviceInfo.Udid

			x.Lock()
			x.svcs[udid] = rp
			x.Unlock()

			log.Printf(`%s START`, udid)
			rp.StartQuicTunnel()
			log.Printf(`%s STOP`, udid)

			x.Lock()
			delete(x.svcs, udid)
			x.Unlock()
		}(rp)
	}
}