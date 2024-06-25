package remotepair

import (
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"log"
)

const (
	WireProtocolVersion = 19
)

func New(r *rsd.Service) (*Service, error) {
	s := &Service{}
	rxc, err := r.StartRemoteService(rsd.TunnelService)
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

	err := x.SendPlainRequest(map[string]any{
		`request`: map[string]any{
			`_0`: map[string]any{
				`handshake`: map[string]any{
					`_0`: handshake,
				},
			},
		},
	})

	if err != nil {return err}
	msg, err := x.RecvPlainResponse()
	if err != nil {return err}

	data := msg[`response`].
	(map[string]any)[`_1`].
	(map[string]any)[`handshake`].
	(map[string]any)[`_0`]

	hs := &Handshake{}
	err = mapstructure.Decode(data, hs)
	if err == nil {
		x.Handshake = hs
		log.Printf(`HANDSHAKE %+v`, x.Handshake)
	}

	return err
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

func (x *Service) RecvPlainResponse() (map[string]any, error) {
	msg, err := x.RecvResponse()
	if err == nil {
		return msg.
		(map[string]any)[`message`].
		(map[string]any)[`plain`].
		(map[string]any)[`_0`].
		(map[string]any), nil
	}

	return nil, err
}

func (x *Service) SendRequest(msg any) error {
	return x.RemoteXpcConnection.Send(map[string]any{
		`mangledTypeName`: `RemotePairing.ControlChannelMessageEnvelope`,
		`value`:           msg,
	})
}

func (x *Service) RecvResponse() (any, error) {
	rsp, err := x.RemoteXpcConnection.Recv()
	if err != nil {
		return nil, err
	}
	return rsp.
	(map[string]any)[`value`], nil
}