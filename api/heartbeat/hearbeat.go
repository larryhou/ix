package heartbeat

import (
	"github.com/larryhou/ix/api/mux/plist"
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
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		var rsp any
		_ = x.Recv(&rsp)
		log.Printf(`HEARTBEAT %+v`, rsp)

		<-ticker.C
		if err := x.Send(map[string]any{`Command`: `Polo`}); err != nil {
			return err
		}
	}
}