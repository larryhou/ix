package notification

import (
	"context"
	"github.com/larryhou/ix/api/dvt/remotesvr"
	"log"
)

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(`com.apple.instruments.server.services.mobilenotifications`)
	if err != nil {
		return nil, err
	}

	s := &Service{
		ch: svr.GetChannel(id),
	}

	return s, nil
}

type Service struct {
	ch  *remotesvr.DTXChannel
	cancel func()
}

func (x *Service) Start() error {
	if x.cancel != nil {
		x.Stop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	x.cancel = cancel

	go x.runloop(ctx)
	err := x.ch.Send(`setApplicationStateNotificationsEnabled:`, new(remotesvr.ArgumentAux).Obj(true), false)
	if err == nil {
		return x.ch.Send(`setMemoryNotificationsEnabled:`, new(remotesvr.ArgumentAux).Obj(true), false)
	}
	return err
}

func (x *Service) Stop() error {
	if x.cancel != nil {
		x.cancel()
		x.cancel = nil
	}

	err := x.ch.Send(`setApplicationStateNotificationsEnabled:`, new(remotesvr.ArgumentAux).Obj(false), false)
	if err == nil {
		return x.ch.Send(`setMemoryNotificationsEnabled:`, new(remotesvr.ArgumentAux).Obj(false), false)
	}
	return err
}

func (x *Service) runloop(ctx context.Context) error {
	ch := x.ch.Streaming()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rsp, err := ch.Recv(nil)
		if err != nil {return err}
		log.Printf(`NOTIFICATION %+v`, rsp)
	}
}
