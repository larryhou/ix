package mounter

import (
	"net"
)

const (
	ServiceName = `com.apple.mobile.mobile_image_mounter`
)

func New(conn net.Conn) *Service {
	s := &Service{Conn: conn}
	return s
}

type Service struct {
	net.Conn
}

