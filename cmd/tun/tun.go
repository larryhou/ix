package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/ginuerzh/gost"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/songgao/water"
	"io"
	"log"
	"reflect"
	"unsafe"
)

func main() {
	ln, err := gost.TunListener(gost.TunConfig{
		Addr: `fd18:ccc9:b73a::2/64`,
		MTU:  16000,
	})
	if err != nil {panic(err)}

	conn, err := ln.Accept()
	if err != nil {panic(err)}

	ifce := *(**water.Interface)(unsafe.Pointer(reflect.ValueOf(conn).Pointer()))
	f := *(*io.ReadWriteCloser)(unsafe.Pointer(reflect.ValueOf(ifce.ReadWriteCloser).Pointer()))


	LOOPBACK := make([]byte, 4)
	binary.BigEndian.PutUint32(LOOPBACK, uint32(layers.ProtocolFamilyIPv6Darwin))
	hdr := make([]byte, 16000+4)
	for err == nil {
		_, err = f.Read(hdr)
		if bytes.Compare(hdr[:4], LOOPBACK) != 0 {
			panic(`BAD LOOPBACK: ` + hex.EncodeToString(hdr[:4]))
		}

		if err == nil {
			pak := gopacket.NewPacket(hdr[4:], layers.LayerTypeIPv6, gopacket.Default)
			log.Printf(`%s`, pak)
		}
	}

	panic(err)
}
