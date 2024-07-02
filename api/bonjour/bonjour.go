package bonjour

import (
	"context"
	"github.com/larryhou/zeroconf/v2"
	"log"
	"net"
	"time"
)

const (
	RemotePairingServiceName              = `_remotepairing._tcp`
	RemotePairingManualPairingServiceName = `_remotepairing-manual-pairing._tcp`
	Mobdev2ServiceName                    = `_apple-mobdev2._tcp`
	RemotedServiceName                    = `_remoted._tcp`
)


func Browse(name string, handle func(v *zeroconf.ServiceEntry) bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	result := make(chan *zeroconf.ServiceEntry)
	log.Printf(`BROWSE %s ...`, name)
	go zeroconf.Browse(ctx, name, "local.", result)
	for {
		select {
		case <-ctx.Done(): return ctx.Err()
		case entry := <-result:
			if !handle(entry) {
				return nil
			}
		}
	}
}

func TCPAddr(name string) (*net.TCPAddr, error) {
	var ent *zeroconf.ServiceEntry
	err := Browse(name, func(v *zeroconf.ServiceEntry) bool {
		ent = v
		log.Printf(`%+v`, ent)
		return false
	})

	if err != nil {return nil, err}

	port := ent.Port
	//port = (port & 0x00FF) << 8 | (port & 0xFF00) >> 8

	var addr net.IP
	switch {
	case len(ent.AddrIPv4) != 0: addr = ent.AddrIPv4[0]
	case len(ent.AddrIPv6) != 0: addr = ent.AddrIPv6[0]
	}

	ifce, err := net.InterfaceByIndex(ent.IfIndex)
	if err != nil {return nil, err}

	return &net.TCPAddr{
		IP:   addr,
		Port: port,
		Zone: ifce.Name,
	}, nil
}