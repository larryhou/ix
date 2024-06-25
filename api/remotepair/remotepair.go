package remotepair

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"io"
	"log"
	"os"
)

const (
	WireProtocolVersion = 19
)

type PairRecord struct {
	PrivateKey []byte
	PublicKey  []byte
	UnlockKey  []byte
}

func New(r *rsd.Service) (*Service, error) {
	var err error
	s := &Service{}
	s.identifier = s.generateHostID()
	s.e25519PubKey,s.e25519PriKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {return nil, err}

	s.x25519PriKey, err = ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {return nil, err}

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
	*PairRecord

	x25519PriKey *ecdh.PrivateKey
	x25519EncKey []byte
	e25519PriKey ed25519.PrivateKey
	e25519PubKey ed25519.PublicKey

	identifier string
	n uint64
}

func (x *Service) connect() error {
	err := x.handshake()
	if err == nil {
		if err = x.validate(); err != nil {
			err = x.pair()
		}
	}

	if err == nil {
		//err = x.encrypt()
	}

	return err
}

func (x *Service) handshake() error {
	req := map[string]any{
		`hostOptions`: map[string]any{
			`attemptPairVerify`: true,
		},
		`wireProtocolVersion`: WireProtocolVersion,
	}

	err := x.SendPlainRequest(map[string]any{
		`request`: map[string]any{
			`_0`: map[string]any{
				`handshake`: map[string]any{
					`_0`: req,
				},
			},
		},
	})

	if err != nil {return err}
	msg, err := x.RecvPlainResponse()
	if err != nil {return err}

	rsp := msg[`response`].
	(map[string]any)[`_1`].
	(map[string]any)[`handshake`].
	(map[string]any)[`_0`]

	hs := &Handshake{}
	err = mapstructure.Decode(rsp, hs)
	if err == nil { x.Handshake = hs }
	return err
}

func (x *Service) encodeTLV(tlv []PairingTLV) []byte {
	buf := &bytes.Buffer{}
	for _, it := range tlv {
		buf.WriteByte(it.Type)
		buf.WriteByte(byte(len(it.Data)))
		buf.Write(it.Data)
	}
	return buf.Bytes()
}

func (x *Service) decodeTLV(b []byte) map[byte]PairingTLV {
	out := make(map[byte]PairingTLV)
	for p := 0; p < len(b); {
		typ := b[p]
		p++
		num := int(b[p])
		p++
		out[typ] = PairingTLV{
			Type: typ,
			Data: b[p:p+num],
		}

		p += num
	}

	return out
}

func (x *Service) performManualPairing(req any) (map[byte]PairingTLV, error) {
	err := x.SendPlainRequest(map[string]any{
		`event`: map[string]any{
			`_0`: map[string]any{
				`pairingData`: map[string]any{`_0`: req},
			},
		},
	})
	if err != nil {return nil, err}

	rsp, err := x.RecvPlainResponse()
	if err != nil {return nil, err}
	data := rsp[`event`].
	(map[string]any)[`_0`].
	(map[string]any)

	if err, ok := data[`pairingRejectedWithError`]; ok {
		return nil, fmt.Errorf(`%+v`, err)
	}

	var peer map[byte]PairingTLV
	if data, ok := data[`pairingData`]; !ok {
		return nil, errors.New(`no pairingData field`)
	} else {
		log.Printf(`PairingData %+v`, data)
		peer = x.decodeTLV(data.
		(map[string]any)[`_0`].
		(map[string]any)[`data`].
		([]byte))
		log.Printf(`PearPairing %+v`, peer)
		return peer, nil
	}
}

func (x *Service) verifyPairFailed(peer map[byte]PairingTLV) error {
	if _, ok := peer[TypeError]; ok {
		err := x.SendPlainRequest(map[string]any{
			`event`: map[string]any{
				`_0`: map[string]any{
					`pairVerifyFailed`: map[string]any{},
				},
			},
		})

		if err == nil {
			err = errors.New(`PairVerifyFailed`)
		}
		return err
	}

	return nil
}

func (x *Service) validate() error {
	tlv := []PairingTLV{
		{Type: TypeState, Data: []byte{0x01}},
		{Type: TypePublicKey, Data: x.x25519PriKey.PublicKey().Bytes()},
	}

	peer, err := x.performManualPairing(map[string]any{
		`data`:            x.encodeTLV(tlv),
		`kind`:            `verifyManualPairing`,
		`startNewSession`: true,
	})
	if err = x.verifyPairFailed(peer); err != nil {return err}

	pearPublicKey, err := ecdh.X25519().NewPublicKey(peer[TypePublicKey].Data)
	if err != nil {return err}
	x.x25519EncKey, err = x.x25519PriKey.ECDH(pearPublicKey)
	if err != nil {return err}

	key := make([]byte, 32)
	_, err = io.ReadFull(hkdf.New(
		sha512.New,
		x.x25519EncKey,
		[]byte(`Pair-Verify-Encrypt-Salt`),
		[]byte(`Pair-Verify-Encrypt-Info`),
	), key)

	cip, err := chacha20poly1305.New(key)
	if err != nil {return err}

	var privateKey ed25519.PrivateKey
	if x.PairRecord == nil {
		privateKey = make(ed25519.PrivateKey, 0x40)
	} else {
		privateKey = x.PairRecord.PrivateKey
	}

	buf := &bytes.Buffer{}
	buf.Write(x.x25519PriKey.PublicKey().Bytes())
	buf.WriteString(x.identifier)
	buf.Write(pearPublicKey.Bytes())
	log.Printf(`SIGNBYTES %s`, hex.EncodeToString(buf.Bytes()))

	signature := ed25519.Sign(privateKey, buf.Bytes())
	log.Printf(`SIGNATURE %s`, hex.EncodeToString(signature))
	encryptedData := cip.Seal(
		[]byte{},
		[]byte("\x00\x00\x00\x00PV-Msg03"),
		x.encodeTLV([]PairingTLV{
			{Type: TypeIdentifier, Data: []byte(x.identifier)},
			{Type: TypeSignature, Data: signature},
		}),
		[]byte{},
	)

	paringData := x.encodeTLV([]PairingTLV{
		{Type: TypeState, Data: []byte{0x03}},
		{Type: TypeEncryptedData, Data: encryptedData},
	})

	_, err = x.performManualPairing(map[string]any{
		`data`:            paringData,
		`kind`:            `verifyManualPairing`,
		`startNewSession`: false,
	})

	if err == nil {
		err = x.verifyPairFailed(peer)
	}

	return err
}

func (x *Service) generateHostID() string {
	name, _ := os.Hostname()
	return uuid.NewMD5(uuid.NameSpaceDNS, []byte(name)).String()
}

func (x *Service) pair() error {
	host, _ := os.Hostname()
	peer, err := x.performManualPairing(map[string]any{
		`data`: x.encodeTLV([]PairingTLV{
			{Type: TypeMethod, Data: []byte{0x00}},
			{Type: TypeState, Data: []byte{0x01}},
		}),
		`kind`:            `setupManualPairing`,
		`sendingHost`:     host,
		`startNewSession`: true,
	})
	if err != nil {return err}
	log.Printf(`PAIR %+v`, peer)
	//pkey := peer[TypePublicKey]
	//salt := peer[TypeSalt]
	//
	//s, err := srp.NewWithHash(crypto.SHA256, 3072)
	//if err != nil {return err}
	//
	//c, err := s.NewClient([]byte(`Pair-Setup`), []byte(`000000`))
	//if err != nil {return err}

	return nil
}

func (x *Service) encrypt() error {
	panic(``)
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