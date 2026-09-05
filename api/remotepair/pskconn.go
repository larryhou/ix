package remotepair

import (
	"log"
	"net"
)

// NewPSKConn dials addr and performs a TLS 1.2 PSK handshake using key as the
// pre-shared secret. It returns a net.Conn whose Read/Write are transparently
// encrypted with TLS_PSK_WITH_AES_256_GCM_SHA384. No CGo or OpenSSL required.
func NewPSKConn(key []byte, addr *net.TCPAddr) (net.Conn, error) {
	log.Printf(`CONNECT %+v`, addr)

	conn, err := net.Dial(`tcp`, addr.String())
	if err != nil {
		return nil, err
	}

	return newPSKConn(conn, key)
}
