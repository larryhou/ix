package applicationlisting

import (
	"github.com/larryhou/ix/api/dvt/remotesvr"
	"github.com/mitchellh/mapstructure"
)

type Application struct {
	BundlePath                string `json:"BundlePath"`
	CFBundleIdentifier        string `json:"CFBundleIdentifier"`
	ContainerBundleIdentifier string `json:"ContainerBundleIdentifier"`
	ContainerBundlePath       string `json:"ContainerBundlePath"`
	DisplayName               string `json:"DisplayName"`
	ExecutableName            string `json:"ExecutableName"`
	ExtensionDictionary       struct {
		NSExtensionAttributes struct {
			NSExtensionVersion string `json:"NSExtensionVersion"`
		} `json:"NSExtensionAttributes"`
		NSExtensionMainStoryboard  string `json:"NSExtensionMainStoryboard"`
		NSExtensionPointIdentifier string `json:"NSExtensionPointIdentifier"`
	} `json:"ExtensionDictionary"`
	PluginIdentifier string `json:"PluginIdentifier"`
	PluginUUID       string `json:"PluginUUID"`
	Restricted       int    `json:"Restricted"`
	Type             string `json:"Type"`
	Version          string `json:"Version"`
}

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(`com.apple.instruments.server.services.device.applictionListing`)
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

func (x *Service) List() ([]*Application, error) {
	err := x.ch.Send(`installedApplicationsMatching:registerUpdateToken:`,
		new(remotesvr.ArgumentAux).Obj(map[string]any{}).Obj(``), true)
	if err != nil {return nil, err}

	var aux *remotesvr.ArgumentAux
	rsp, err := x.ch.Recv(&aux)
	if err == nil {
		var out []*Application
		err = mapstructure.Decode(rsp, &out)
		if err == nil {
			return out, nil
		}
	}

	return nil, err
}
