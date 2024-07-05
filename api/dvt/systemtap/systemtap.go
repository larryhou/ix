package systemtap

import (
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"log"
)

func New(ch *remotesvr.DTXChannel) (*Service, error) {
	s := &Service{
		ch: ch,
	}
	go s.runloop()
	return s, s.start()
}

type Service struct {
	ch  *remotesvr.DTXChannel
}

func (x *Service) start() error {
	err := x.ch.Send(`start`, new(remotesvr.ArgumentAux), false)
	if err != nil {return err}

	rsp, err := x.ch.Recv(nil)
	log.Printf(`TAP START %+v %v`, rsp, err)
	return err
}

func (x *Service) Stop() error {
	return x.ch.Send(`clear`, new(remotesvr.ArgumentAux), false)
}

func (x *Service) runloop() error {
	for {
		rsp, err := x.ch.Recv(nil)
		log.Printf(`TAP EVENT %+v %v`, rsp, err)
		if err != nil {return err}
	}
}
