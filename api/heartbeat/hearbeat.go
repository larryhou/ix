package heartbeat

import (
	"github.com/larryhou/j3idevice/api/j3/plist"
	"log"
	"net"
	"time"
)

const (
	ServiceName = `com.apple.mobile.heartbeat`
)

func New(conn net.Conn) *Service {
	s := &Service{Connection: plist.NewConnection(conn)}
	return s
}

type Service struct {
	*plist.Connection
}

func (x *Service) Run() error {
	err := error(nil)
	for err == nil {
		var rsp any
		_ = x.Recv(&rsp)
		log.Printf(`HEARTBEAT %+v %v`, rsp, err)

		select {
		case <-time.After(time.Second):
			err = x.Send(map[string]any{
				`Command`: `Polo`,
			})
		}
	}

	return err
}