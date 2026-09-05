package energy

import (
	"github.com/larryhou/ix/api/dvt/remotesvr"
	"log"
)

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(`com.apple.xcode.debug-gauge-data-providers.Energy`)
	if err != nil {
		return nil, err
	}

	s := &Service{
		ch: svr.GetChannel(id),
	}

	go s.runloop()
	return s, nil
}

type Service struct {
	ch  *remotesvr.DTXChannel
	pid []int
}

func (x *Service) Start(pid []int) error {
	if len(x.pid) != 0 {
		x.Stop()
	}
	x.pid = pid
	return x.ch.Send(`startSamplingForPIDs:`, new(remotesvr.ArgumentAux).Obj(pid), false)
}

func (x *Service) Stop() error {
	return x.ch.Send(`stopSamplingForPIDs:`, new(remotesvr.ArgumentAux).Obj(x.pid), false)
}

func (x *Service) runloop() error {
	for {
		err := x.ch.Send(`sampleAttributes:forPIDs:`,
			new(remotesvr.ArgumentAux).Obj(map[string]any{}).Obj(x.pid), true)
		if err != nil {return err}

		rsp, err := x.ch.Recv(nil)
		if err != nil {return err}
		log.Printf(`ENERGY %+v`, rsp)
	}
}