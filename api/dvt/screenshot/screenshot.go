package screenshot

import (
	"github.com/larryhou/ix/api/dvt/remotesvr"
	"github.com/larryhou/ix/api/util"
)

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(`com.apple.instruments.server.services.screenshot`)
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
}

func (x *Service) Capture() ([]byte, error) {
	err := x.ch.Send(`takeScreenshot`, new(remotesvr.ArgumentAux), true)
	if err != nil {return nil, err}
	return util.Cast[[]byte](x.ch.Recv(nil))
}