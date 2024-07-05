package processctrl

import (
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
)

func New(svr *remotesvr.Service) (*Service, error) {
	s := &Service{
		svr: svr,
	}

	return s, s.connect()
}

type Service struct {
	svr *remotesvr.Service
	ch  *remotesvr.DTXChannel
}

func (x *Service) connect() error {
	id, err := x.svr.CreateChannel(`com.apple.instruments.server.services.processcontrol`)
	if err == nil {
		x.ch = x.svr.GetChannel(id)
		push := x.svr.GetChannel(-id)
		go func() error {
			for {
				var aux *remotesvr.ArgumentAux
				sel, err := push.Recv(&aux)
				switch sel {
				case `outputReceived:fromProcess:atTime:`:
					//log.Printf(`%s`, aux.Values[0].Data)
				}
				if err != nil {return err}
			}
		}()
	}

	return err
}

func (x *Service) Signal(pid, sig int) error {
	err := x.ch.Send(`sendSignal:toPid:`,
		new(remotesvr.ArgumentAux).Obj(sig).Obj(pid), true)
	if err != nil {return err}
	_, err = x.ch.Recv(nil)
	return err
}

func (x *Service) Kill(pid int) error {
	return x.ch.Send(`killPid:`,
		new(remotesvr.ArgumentAux).Obj(pid), false)
}

type LaunchContext struct {
	Arguments      []string
	NoKillExisting bool
	StartSuspended bool
	Environ        map[string]string
	Options        map[string]string
}

func (x *Service) Launch(bundleid string, ctx LaunchContext) (int, error) {
	options := map[string]any{
		`StartSuspendedKey`: ctx.StartSuspended,
		`KillExisting`:      !ctx.NoKillExisting,
	}

	for k, v := range ctx.Options {
		options[k] = v
	}

	environ := ctx.Environ
	if environ == nil { environ = make(map[string]string) }
	arguments := ctx.Arguments
	if arguments == nil {arguments = []string{}}

	args := new(remotesvr.ArgumentAux).
		Obj(``).Obj(bundleid).Obj(environ).Obj(arguments).Obj(options)
	err := x.ch.Send(`launchSuspendedProcessWithDevicePath:bundleIdentifier:environment:arguments:options:`, args, true)
	if err != nil {return 0, err}
	rsp, err := x.ch.Recv(nil)
	if err == nil {
		return int(rsp.(uint64)), nil
	}

	return 0, err
}