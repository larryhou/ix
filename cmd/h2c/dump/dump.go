package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/http2"
	"io"
	"os"
)

func main() {
	for _, name := range os.Args[1:] {
		err := dump(name)
		if err != nil {panic(err)}
	}
}

func dump(name string) error {
	handle, err := pcap.OpenOffline(name)
	if err != nil {return err}

	out, err := os.OpenFile(name + `.txt`, os.O_CREATE | os.O_WRONLY | os.O_TRUNC, 0644)
	if err != nil {return err}
	defer out.Close()

	w := io.MultiWriter(out, os.Stdout)

	type context struct {
		bytes.Buffer
		id  int
		num int
	}

	dict := make(map[string]*context)

	source := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)
	for packet := range source.Packets() {
		tcp := packet.TransportLayer().(*layers.TCP)
		if len(tcp.Payload) == 0 {
			continue
		}

		address := ``
		switch ip := packet.NetworkLayer().(type) {
		case *layers.IPv4: address = ip.SrcIP.String()
		case *layers.IPv6: address = ip.SrcIP.String()
		}

		ctx, ok := dict[address]
		if !ok {
			ctx = &context{
				id: len(dict),
			}
			dict[address] = ctx
		}

		flush := func(b []byte) {
			fmt.Fprintf(w, "%d %s %4d %s\n", ctx.id, address, len(b), hex.EncodeToString(b))
		}

		p := tcp.Payload
		if len(p) == len(http2.ClientPreface) &&
			p[0] == 'P' && p[1] == 'R' && p[2] == 'I' {
			flush(p)
			continue
		}

		if ctx.num > ctx.Len() {
			k := min(ctx.num-ctx.Len(), len(p))
			ctx.Write(p[:k])
			p = p[k:]
		}

		if ctx.num > 0 && ctx.num == ctx.Len() {
			flush(ctx.Bytes())
			ctx.Reset()
			ctx.num = 0
		}

		for len(p) >= 9 {
			num := (int(p[0]) << 16 | int(p[1]) << 8 | int(p[2])) + 9
			if len(p) < num {
				ctx.Write(p)
				ctx.num = num
				break
			} else {
				flush(p[:num])
				p = p[num:]
			}
		}
	}

	return nil
}