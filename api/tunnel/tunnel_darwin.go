//go:build darwin

package tunnel

import (
	"github.com/ginuerzh/gost"
	"net"
	"strconv"
)

func (x *Service) Start(conn any) error {
	n := netmaskBits(net.ParseIP(x.Descriptor.ClientParameters.Netmask))

	listener, err := gost.TunListener(gost.TunConfig{
		Addr: x.Descriptor.ClientParameters.Address + `/` + strconv.Itoa(n),
		MTU:  x.Descriptor.ClientParameters.Mtu,
		Peer: x.Descriptor.ServerAddress,
	})
	if err != nil {
		return err
	}
	tun, err := listener.Accept()
	if err != nil {
		return err
	}
	defer tun.Close()

	ifce := gost.WaterInterface(tun)
	addr := &net.TCPAddr{
		IP:   net.ParseIP(x.ServerAddress),
		Port: x.ServerRSDPort,
		Zone: ifce.Name(),
	}

	return x.startTunnel(tun, addr, conn)
}
