package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"golang.org/x/net/http2"
	"log"
	"net"
	"net/http"
)

func main() {
	client := http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				address, err := net.ResolveTCPAddr(network, addr)
				if err != nil {return nil, err}
				address.Zone = `en6`
				return net.DialTCP(network, nil, address)
			},
		},
	}

	buf := &bytes.Buffer{}
	xpc.Encode(buf, &xpc.Message{
		Flag: 0x0201,
	})

	f := &http2.Framer{}
	f.ReadFrame()

	t2 := http2.Transport{}
	t2.AllowHTTP = true

	resp, err := client.Post("http://[fe80::fc5d:4ff:fecd:10a3]:58783", ``, buf)
	if err != nil {
		log.Fatal(fmt.Errorf("error making request: %v", err))
	}
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Proto)
}
