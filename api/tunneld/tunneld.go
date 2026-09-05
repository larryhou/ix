package tunneld

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/larryhou/ix/api/bonjour"
	"github.com/larryhou/ix/api/j3"
	"github.com/larryhou/ix/api/j3/usbmux"
	"github.com/larryhou/ix/api/lockdown"
	"github.com/larryhou/ix/api/remotepair"
	"github.com/larryhou/ix/api/tunnel"
	"github.com/larryhou/ix/api/tunnel/rsd"
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
			_, alreadyTracked := x.usb.live[udid]
			x.usb.live[udid] = msg
			x.usb.udid[uint64(dvid)] = udid
			x.usb.Unlock()
			// 仅首次 Attached 启动 goroutine；
			// usbmux 重连后重发的 Attached 事件由已在运行的 tryConnectUSB 循环处理
			if !alreadyTracked {
				go x.tryConnectUSB(udid, dvid)
			}
		case `Detached`:
			x.usb.Lock()
			dvid := msg[`DeviceID`].(uint64)
			udid := x.usb.udid[dvid]
			delete(x.usb.live, udid)
			delete(x.usb.udid, dvid)
			x.usb.Unlock()

			// dvid 未记录时 udid 为空，跳过后续清理避免误删
			if udid == `` {
				log.Printf(`USB Detached unknown dvid=%d`, dvid)
				break
			}

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
	// 快速检查：仅持读锁检查 map，不做任何网络操作
	x.RLock()
	_, ok := x.addr[addr.IP.String()]
	x.RUnlock()
	if ok {return pass}

	// 网络操作在锁外执行，避免长时间持锁
	var rp *remotepair.Service
	if !remotep {
		r, err := rsd.New(addr)
		if err != nil {return err}

		x.Lock()
		_, ok = x.svcs[r.Descriptor.Properties.UniqueDeviceID]
		x.Unlock()
		if ok {return pass}

		rp, err = remotepair.NewFromRSD(r)
		if err != nil {return err}
	} else {
		for _, udid := range remotepair.ListUdid() {
			udid := udid
			x.RLock()
			_, ok := x.svcs[udid]
			x.RUnlock()
			if ok {continue}

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

	// 再次检查防止并发重复注册
	udid := rp.Udid
	x.Lock()
	if _, ok = x.svcs[udid]; ok {
		x.Unlock()
		return pass
	}
	if _, ok = x.addr[addr.IP.String()]; ok {
		x.Unlock()
		return pass
	}
	x.svcs[udid] = rp
	x.addr[addr.IP.String()] = rp
	x.Unlock()

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
// Retries automatically as long as the device stays connected (present in usb.live).
func (x *daemon) tryConnectUSB(udid string, dvid int) {
	for attempt := 1; ; attempt++ {
		// 设备已拔出则停止重试
		x.usb.Lock()
		_, connected := x.usb.live[udid]
		x.usb.Unlock()
		if !connected {
			log.Printf(`USB %s disconnected, stop retry`, udid)
			return
		}

		x.RLock()
		_, active := x.usbTuns[udid]
		x.RUnlock()
		if active {
			log.Printf(`USB %s already active`, udid)
			return
		}

		if err := x.connectUSB(udid, dvid); err != nil {
			log.Printf(`USB %s attempt %d failed: %v, retry in 3s`, udid, attempt, err)
			time.Sleep(3 * time.Second)
			continue
		}
		// connectUSB 正常退出（tunnel 断开）后直接重试
		log.Printf(`USB %s tunnel ended, reconnecting (attempt %d)`, udid, attempt+1)
	}
}

// connectUSB 执行一次完整的 USB tunnel 连接，直到 tunnel 断开或出错。
func (x *daemon) connectUSB(udid string, dvid int) error {
	mux, err := usbmux.New()
	if err != nil {
		return fmt.Errorf(`usbmux: %w`, err)
	}

	handle := &j3.Handle{UDID: udid, DVID: dvid}
	lockd, err := lockdown.New(mux, handle)
	if err != nil {
		return fmt.Errorf(`lockdown: %w`, err)
	}
	defer lockd.Close()

	svc, err := lockd.StartService(rsd.ComAppleInternalDevicecomputeCoreDeviceProxy)
	if err != nil {
		return fmt.Errorf(`start CoreDeviceProxy: %w`, err)
	}

	tun, err := tunnel.New(svc, tunnel.MtuTcp, context.Background())
	if err != nil {
		svc.Close()
		return fmt.Errorf(`tunnel.New: %w`, err)
	}

	x.Lock()
	if _, active := x.usbTuns[udid]; active {
		x.Unlock()
		tun.Stop()
		svc.Close()
		return nil
	}
	x.usbTuns[udid] = tun
	x.Unlock()

	log.Printf(`USB %s tunnel START`, udid)
	err = tun.Start(svc)
	log.Printf(`USB %s tunnel STOP: %v`, udid, err)

	x.Lock()
	delete(x.usbTuns, udid)
	x.Unlock()

	return err
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

		// ip 为空时跳过，避免 TCPAddr.String() panic
		if ip == nil {
			log.Printf(`BROWSE skip entry with no IP: %s`, ent.ServiceInstanceName())
			continue
		}

		// 每条 Bonjour 事件独立 goroutine 处理，不阻塞后续事件
		// 必须在启动 goroutine 前复制循环变量，避免闭包捕获到后续迭代的值
		ent, ip, ifce := ent, ip, ifce
		go func() {
			err := x.tryConnect(&net.TCPAddr{
				IP:   ip,
				Port: ent.Port,
				Zone: ifce.Name,
			}, strings.HasSuffix(ent.Service, bonjour.RemotePairingServiceName))
			if err != nil && err != pass {
				log.Printf(`BROWSE %s connect error: %v`, ip, err)
			}
		}()
	}
}