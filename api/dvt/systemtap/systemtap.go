package systemtap

import (
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"log"
)

func New(sv *remotesvr.Service, conf map[string]any) (*Service, error) {
	id, err := sv.OpenChannel(`com.apple.instruments.server.services.sysmontap`)
	if err != nil {
		return nil, err
	}

	ch := sv.GetChannel(id)
	err = ch.Send(`setConfig:`, new(remotesvr.ArgumentAux).Obj(conf), false)
	if err != nil {return nil, err}

	s := &Service{
		ch: ch,
	}

	go s.runloop(sv.GetChannel(-id))
	return s, s.start()
}

type Service struct {
	ch  *remotesvr.DTXChannel
}

func (x *Service) start() error {
	err := x.ch.Send(`start`, new(remotesvr.ArgumentAux), false)
	if err != nil {return err}
	return err
}

func (x *Service) Stop() error {
	return x.ch.Send(`clear`, new(remotesvr.ArgumentAux), false)
}

func (x *Service) runloop(ch *remotesvr.DTXChannel) error {
	for {
		rsp, err := ch.Recv(nil)
		log.Printf(`TAP EVENT %+v %v`, rsp, err)
		if err != nil {return err}
	}
}
