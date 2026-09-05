package dvt

import (
	"github.com/larryhou/ix/api/dvt/applicationlisting"
	"github.com/larryhou/ix/api/dvt/conditioninducer"
	"github.com/larryhou/ix/api/dvt/deviceinfo"
	"github.com/larryhou/ix/api/dvt/graphics"
	"github.com/larryhou/ix/api/dvt/networkmonitor"
	"github.com/larryhou/ix/api/dvt/processctrl"
	"github.com/larryhou/ix/api/dvt/remotesvr"
	"github.com/larryhou/ix/api/dvt/screenshot"
)

func New(svr *remotesvr.Service) (*Service, error) {
	return &Service{
		Service: svr,
	}, nil
}

type Service struct {
	*remotesvr.Service

	processctrl        *processctrl.Service
	screenshot         *screenshot.Service
	applicationlisting *applicationlisting.Service
	deviceinfo         *deviceinfo.Service
	networkmonitor     *networkmonitor.Service
	graphics           *graphics.Service
	conditioninducer   *conditioninducer.Service
}

func (x *Service) ProcessCtrl() (*processctrl.Service, error) {
	if x.processctrl == nil {
		pc, err := processctrl.New(x.Service)
		if err != nil {return nil, err}
		x.processctrl = pc
	}

	return x.processctrl, nil
}

func (x *Service) ScreenShot() (*screenshot.Service, error) {
	if x.screenshot == nil {
		ss, err := screenshot.New(x.Service)
		if err != nil {return nil, err}
		x.screenshot = ss
	}

	return x.screenshot, nil
}

func (x *Service) ApplicationListing() (*applicationlisting.Service, error) {
	if x.applicationlisting == nil {
		al, err := applicationlisting.New(x.Service)
		if err != nil {return nil, err}
		x.applicationlisting = al
	}

	return x.applicationlisting, nil
}

func (x *Service) DeviceInfo() (*deviceinfo.Service, error) {
	if x.deviceinfo == nil {
		di, err := deviceinfo.New(x.Service)
		if err != nil {return nil, err}
		x.deviceinfo = di
	}

	return x.deviceinfo, nil
}

func (x *Service) NetworkMonitor() (*networkmonitor.Service, error) {
	if x.networkmonitor == nil {
		nm, err := networkmonitor.New(x.Service)
		if err != nil {return nil, err}
		x.networkmonitor = nm
	}

	return x.networkmonitor, nil
}

func (x *Service) Graphics() (*graphics.Service, error) {
	if x.graphics == nil {
		g, err := graphics.New(x.Service)
		if err != nil {return nil, err}
		x.graphics = g
	}

	return x.graphics, nil
}

func (x *Service) ConditionInducer() (*conditioninducer.Service, error) {
	if x.conditioninducer == nil {
		ci, err := conditioninducer.New(x.Service)
		if err != nil {return nil, err}
		x.conditioninducer = ci
	}

	return x.conditioninducer, nil
}
