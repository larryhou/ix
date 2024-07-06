package location

import (
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
)

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(`com.apple.instruments.server.services.LocationSimulation`)
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

func (x *Service) Simulate(latitude, longitude float64) error {
	err := x.ch.Send(`simulateLocationWithLatitude:longitude:`,
		new(remotesvr.ArgumentAux).Obj(latitude).Obj(longitude), true)
	if err != nil {return err}
	_, err = x.ch.Recv(nil)
	return err
}

func (x *Service) Stop() error {
	return x.ch.Send(`stopLocationSimulation`, new(remotesvr.ArgumentAux), false)
}

