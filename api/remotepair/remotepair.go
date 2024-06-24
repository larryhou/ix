package remotepair

import (
	"encoding/json"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"log"
	"os"
)

const (
	WireProtocolVersion = 19
)

func New(r *rsd.Service) (*Service, error) {
	s := &Service{}
	rxc, err := r.StartRemoteService(rsd.TunnelService, s.handle)
	if err == nil {
		s.RemoteXpcConnection = rxc
		err = s.connect()
	}
	return s, err
}

type Service struct {
	*xpc.RemoteXpcConnection
	*Handshake
	n uint64
}

func (x *Service) connect() error {
	err := x.handshake()
	return err
}

func (x *Service) handshake() error {
	handshake := map[string]any{
		`hostOptions`: map[string]any{
			`attemptPairVerify`: true,
		},
		`wireProtocolVersion`: WireProtocolVersion,
	}

	return x.SendPlainRequest(map[string]any{
		`request`: map[string]any{
			`_0`: map[string]any{
				`handshake`: map[string]any{
					`_0`: handshake,
				},
			},
		},
	})
}

func (x *Service) handle(msg *xpc.Message) (err error) {
	if msg.Payload == nil || msg.Data == nil {return}
	json.NewEncoder(os.Stdout).Encode(msg.Data)
	data := msg.Data.(map[string]any)
	if len(data) == 0 {return}

	{
		pak, ok := data, true
		for _, k := range []string{`value`, `message`,`plain`,`_0`} {
			pak, ok = pak[k].(map[string]any)
			if !ok {break}
		}
		
		if ok {
			return x.plain(pak)
		}
	}
	
	return
}

func (x *Service) plain(msg map[string]any) (err error) {
	data := msg[`response`].(map[string]any)[`_1`].(map[string]any)
	for k, v := range data {
		switch k {
		case `handshake`:
			hs := &Handshake{}
			err = mapstructure.Decode(v.(map[string]any)[`_0`], hs)
			if err == nil {
				x.Handshake = hs
				log.Printf(`HANDSHAKE %+v`, x.Handshake)
			}
		}
	}
	return
}

func (x *Service) SendPlainRequest(msg map[string]any) error {
	data := map[string]any{
		`message`: map[string]any{
			`plain`: map[string]any{`_0`: msg},
		},
		`originatedBy`:   `host`,
		`sequenceNumber`: x.n,
	}
	x.n++
	return x.SendRequest(data)
}

func (x *Service) SendRequest(msg any) error {
	return x.RemoteXpcConnection.SendRequest(map[string]any{
		`mangledTypeName`: `RemotePairing.ControlChannelMessageEnvelope`,
		`value`:           msg,
	})
}