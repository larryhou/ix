package bonjour

import (
	"context"
	"github.com/grandcat/zeroconf"
	"net"
)

const (
	RemotePairingServiceName              = `_remotepairing._tcp`
	RemotePairingManualPairingServiceName = `_remotepairing-manual-pairing._tcp`
	Mobdev2SericeName                     = `_apple-mobdev2._tcp`
	RemotedServiceName                    = `_remoted._tcp`
)


func Browse(name string, handle func(v *zeroconf.ServiceEntry) bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan *zeroconf.ServiceEntry)
	defer close(result)

	go zeroconf.Browse(ctx, name, "local.", result)
	for entry := range result {
		if !handle(entry) {break}
	}

	return nil
}

func TCPAddr(ip net.IP) *net.TCPAddr {
	//items, _ := net.Interfaces()
	//for _, ifce := range items {
	//	list, _ := ifce.Addrs()
	//	for _, addr := range list {
	//		addr.String()
	//	}
	//}
	panic(``)
}