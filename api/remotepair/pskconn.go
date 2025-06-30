package remotepair

//#cgo CXXFLAGS: -Wno-deprecated-declarations
//#cgo LDFLAGS: -lPSKConn -lssl -lcrypto -lstdc++
//#include "src/PSKConn.h"
//#include <stdlib.h>
//#include <string.h>
import "C"
import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"syscall"
	"unsafe"
)

func NewPSKConn(key []byte, addr *net.TCPAddr) (net.Conn, error) {
	x := &pskConn{
		key: key,
		addr: addr,
	}

	return x, x.connect()
}

type pskConn struct {
	key  []byte
	addr *net.TCPAddr

	h unsafe.Pointer
	net.Conn
}

func (x *pskConn) connect() error {
	log.Printf(`CONNECT %+v`, x.addr)

	c, err := net.Dial(`tcp`, x.addr.String())
	if err != nil {return err}
	x.Conn = c

	fd := -1
	{
		conn := reflect.ValueOf(c.(*net.TCPConn)).Elem().FieldByName("conn")
		netfd := conn.FieldByName("fd")
		polfd := netfd.Elem().FieldByName("pfd")
		sysfd := polfd.FieldByName("Sysfd")

		fd = int(sysfd.Int())
		syscall.SetNonblock(fd, false)
	}

	key := C.CString(hex.EncodeToString(x.key))
	defer C.free(unsafe.Pointer(key))

	errStr := C.PSK_newConn(C.int(fd), key, &x.h)
	if errStr != nil {
		return errors.New(C.GoString(errStr))
	}

	return nil
}

func (x *pskConn) Write(p []byte) (int, error) {
	n := int(C.PSK_write(x.h, unsafe.Pointer(&p[0]), C.int(len(p))))
	if n >= 0 {
		return n, nil
	}

	return 0, fmt.Errorf(`PSK WRITE ERROR: %d`, -n)
}

func (x *pskConn) Read(p []byte) (int, error) {
	n := int(C.PSK_read(x.h, unsafe.Pointer(&p[0]), C.int(len(p))))
	if n >= 0 {
		return n, nil
	}

	return 0, fmt.Errorf(`PSK READ ERROR: %d`, -n)
}

func (x *pskConn) Close() error {
	C.PSK_close(x.h)
	if x.Conn != nil {
		x.Conn.Close()
		x.Conn = nil
	}
	return nil
}
