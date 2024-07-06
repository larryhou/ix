package dvt

import (
	"github.com/larryhou/j3idevice/api/dvt/applicationlisting"
	"github.com/larryhou/j3idevice/api/dvt/deviceinfo"
	"github.com/larryhou/j3idevice/api/dvt/processctrl"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/dvt/screenshot"
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
