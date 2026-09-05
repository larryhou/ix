//go:build windows

package tunnel

import (
	"errors"
	"github.com/ginuerzh/gost"
	"net"
	"strconv"
)

func (x *Service) Start(conn any) error {
	n := netmaskBits(net.ParseIP(x.Descriptor.ClientParameters.Netmask))

	listener, err := gost.TunListener(gost.TunConfig{
		Addr: x.Descriptor.ClientParameters.Address + `/` + strconv.Itoa(n),
		MTU:  x.Descriptor.ClientParameters.Mtu,
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
	if ifce == nil {
		return errors.New(`WaterInterface: unexpected conn type from TunListener`)
	}

	// On Windows, IPv6 zone IDs must be numeric interface indices, not names.
	iface, err := net.InterfaceByName(ifce.Name())
	if err != nil {
		return err
	}
	addr := &net.TCPAddr{
		IP:   net.ParseIP(x.ServerAddress),
		Port: x.ServerRSDPort,
		Zone: strconv.Itoa(iface.Index),
	}

	return x.startTunnel(tun, addr, conn)
}
