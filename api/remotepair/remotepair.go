package remotepair

import (
	"bytes"
	"crypto"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fmitra/srp"
	"github.com/google/uuid"
	"github.com/larryhou/gomobiledevice3/api/tunnel/rsd"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"howett.net/plist"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"
)

const (
	WireProtocolVersion = 19
)

type PairRecord struct {
	E25519PriKey ed25519.PrivateKey
	E25519PubKey ed25519.PublicKey
	UnlockKey    []byte
}

func New(r *rsd.Service) (*Service, error) {
	var err error
	s := &Service{}
	s.id = s.generateHostID()

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

	signature []byte

	serverCip cipher.AEAD
	clientCip cipher.AEAD
	sequence  uint64

	sc *srp.Client
	id string
	n  uint64
}

func (x *Service) connect() error {
	err := x.handshake()
	if err == nil {
		if err = x.validate(); err != nil {
			err = x.pair()
		}
	}

	if err == nil {
		err = x.initEncryptionKeys()
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
	if err == nil {
		x.Handshake = hs
		_ = x.retrieve()
	}
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

func (x *Service) doPairing(req any) (map[byte]PairingTLV, error) {
	err := x.SendPlainRequest(map[string]any{
		`event`: map[string]any{
			`_0`: map[string]any{
				`pairingData`: map[string]any{`_0`: req},
			},
		},
	})

	if err != nil {return nil, err}
	return x.recvPairingResponse()
}

type PairVerifyError []byte

func (x PairVerifyError) Error() string {
	return fmt.Sprintf(`PairVerifyError(%s)`, hex.EncodeToString(x))
}

func (x *Service) recvPairingResponse() (map[byte]PairingTLV, error) {
	msg, err := x.RecvPlainResponse()
	if err != nil {return nil, err}
	rsp := msg[`event`].
	(map[string]any)[`_0`].
	(map[string]any)
	log.Printf(`PAIRING RESPONSE %v`, rsp)

	if err, ok := rsp[`pairingRejectedWithError`]; ok {
		return nil, fmt.Errorf(`%+v`, err)
	}

	var peer map[byte]PairingTLV
	if _, ok := rsp[`awaitingUserConsent`]; ok {
		return x.recvPairingResponse()
	}

	if data, ok := rsp[`pairingData`]; !ok {
		return nil, errors.New(`no pairingData field`)
	} else {
		log.Printf(`PairingData %+v`, data)
		peer = x.decodeTLV(data.
		(map[string]any)[`_0`].
		(map[string]any)[`data`].
		([]byte))
		log.Printf(`PeerPairing %+v`, peer)
		if r, ok := peer[TypeError]; ok {
			return peer, PairVerifyError(r.Data)
		}

		return peer, nil
	}
}

func (x *Service) verifyPairing(err error) error {
	if _, ok := err.(PairVerifyError); ok {
		_ = x.SendPlainRequest(map[string]any{
			`event`: map[string]any{
				`_0`: map[string]any{
					`pairVerifyFailed`: map[string]any{},
				},
			},
		})
	}

	return err
}

func (x *Service) validate() error {
	tlv := []PairingTLV{
		{Type: TypeState, Data: []byte{0x01}},
		{Type: TypePublicKey, Data: x.x25519PriKey.PublicKey().Bytes()},
	}

	peer, err := x.doPairing(map[string]any{
		`data`:            x.encodeTLV(tlv),
		`kind`:            `verifyManualPairing`,
		`startNewSession`: true,
	})

	if err = x.verifyPairing(err); err != nil {return err }

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
		privateKey = x.PairRecord.E25519PriKey
	}

	buf := &bytes.Buffer{}
	buf.Write(x.x25519PriKey.PublicKey().Bytes())
	buf.WriteString(x.id)
	buf.Write(pearPublicKey.Bytes())
	log.Printf(`SIGNBYTES %s`, hex.EncodeToString(buf.Bytes()))

	signature := ed25519.Sign(privateKey, buf.Bytes())
	log.Printf(`SIGNATURE %s`, hex.EncodeToString(signature))
	encryptedData := cip.Seal(
		[]byte{},
		[]byte("\x00\x00\x00\x00PV-Msg03"),
		x.encodeTLV([]PairingTLV{
			{Type: TypeIdentifier, Data: []byte(x.id)},
			{Type: TypeSignature, Data: signature},
		}),
		[]byte{},
	)

	paringData := x.encodeTLV([]PairingTLV{
		{Type: TypeState, Data: []byte{0x03}},
		{Type: TypeEncryptedData, Data: encryptedData},
	})

	peer, err = x.doPairing(map[string]any{
		`data`:            paringData,
		`kind`:            `verifyManualPairing`,
		`startNewSession`: false,
	})

	return x.verifyPairing(err)
}

func (x *Service) generateHostID() string {
	name, _ := os.Hostname()
	return uuid.NewMD5(uuid.NameSpaceDNS, []byte(name)).String()
}

const (
	GROUP3072 = "5:" +
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA6" +
		"3B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F2411" +
		"7C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08" +
		"CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D" +
		"04507A33A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7" +
		"ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864D8760273" +
		"3EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E208E24FA074E5AB31" +
		"43DB5BFCE0FD108E4B82D120A93AD2CAFFFFFFFFFFFFFFFF"
)

func (x *Service) pair() error {
	log.Printf("PAIRING...")
	host, _ := os.Hostname()
	peer, err := x.doPairing(map[string]any{
		`data`: x.encodeTLV([]PairingTLV{
			{Type: TypeMethod, Data: []byte{0x00}},
			{Type: TypeState, Data: []byte{0x01}},
		}),
		`kind`:            `setupManualPairing`,
		`sendingHost`:     host,
		`startNewSession`: true,
	})
	if err != nil {return err}

	log.Printf(`PAIRING %+v`, peer)
	err = x.verifyProof(peer[TypePublicKey].Data, peer[TypeSalt].Data)
	if err != nil {return err}

	var tlv map[byte]PairingTLV
	err = x.applyPairing(&tlv)
	if err != nil {return err}
	log.Printf(`SYNC PEER %+v`, tlv)

	err = x.initEncryptionKeys()
	if err == nil {
		err = x.createRemoteUnlock()
	}

	if err == nil {
		err = x.cache()
	}

	return err
}

func (x *Service) cache() error {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, `.j3device`)
	name := fmt.Sprintf(`PAIR_%s.plist`, x.Handshake.PeerDeviceInfo.Identifier)
	if _, err := os.Stat(root); err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(root, 0766)
		if err != nil {return err}
	}

	f, err := os.OpenFile(filepath.Join(root, name), os.O_CREATE | os.O_TRUNC | os.O_WRONLY, 0644)
	if err != nil {return err}
	defer f.Close()

	return plist.NewEncoderForFormat(f, plist.XMLFormat).Encode(x.PairRecord)
}

func (x *Service) retrieve() error {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, `.j3device`)
	name := fmt.Sprintf(`PAIR_%s.plist`, x.Handshake.PeerDeviceInfo.Identifier)
	f, err := os.Open(filepath.Join(root, name))
	if err != nil {return err}
	defer f.Close()

	rp := &PairRecord{}
	err = plist.NewDecoder(f).Decode(rp)
	if err == nil {
		x.PairRecord = rp
	}

	return err
}

func (x *Service) initEncryptionKeys() error {
	clientKey := make([]byte, 32)
	_, err := io.ReadFull(
		hkdf.New(sha512.New, x.x25519EncKey, nil, []byte(`ClientEncrypt-main`)),
		clientKey,
	)
	x.clientCip, err = chacha20poly1305.New(clientKey)
	if err != nil {return err}

	serverKey := make([]byte, 32)
	_, err = io.ReadFull(
		hkdf.New(sha512.New, x.x25519EncKey, nil, []byte(`ServerEncrypt-main`)),
		serverKey,
	)
	x.serverCip, err = chacha20poly1305.New(serverKey)
	return err
}

func (x *Service) applyPairing(tlv *map[byte]PairingTLV) error {
	host, _ := os.Hostname()
	setupKey := make([]byte, 32)
	_, err := io.ReadFull(hkdf.New(
		sha512.New,
		x.x25519EncKey,
		[]byte(`Pair-Setup-Encrypt-Salt`),
		[]byte(`Pair-Setup-Encrypt-Info`),
	), setupKey)
	if err != nil {return err}

	x.PairRecord = &PairRecord{}
	x.E25519PubKey,x.E25519PriKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {return err}

	buf := &bytes.Buffer{}
	buf.Write(setupKey)
	buf.WriteString(x.id)
	buf.Write(x.E25519PubKey)
	x.signature, err = x.E25519PriKey.Sign(rand.Reader, buf.Bytes(), nil)
	if err != nil {return err}

	dev := x.opack(map[string]any{
		`altIRK`:                      []byte("\xe9\xe8-\xc0jIykVoT\x00\x19\xb1\xc7{"),
		`btAddr`:                      `11:22:33:44:55:66`,
		`mac`:                         []byte("\x11\x22\x33\x44\x55\x66"),
		`remotepairing_serial_number`: `AAAAAAAAAAAA`,
		`accountID`:                   x.id,
		`model`:                       `computer-model`,
		`name`:                        host,
	})

	cip, err := chacha20poly1305.New(setupKey)
	if err != nil {return err}

	encrptedData := cip.Seal(
		[]byte{},
		[]byte("\x00\x00\x00\x00PS-Msg05"),
		x.encodeTLV([]PairingTLV{
			{Type: TypeIdentifier, Data: []byte(x.id)},
			{Type: TypePublicKey, Data: x.E25519PubKey},
			{Type: TypeSignature, Data: x.signature},
			{Type: TypeInfo, Data: dev},
		}),
		[]byte{},
	)

	peer, err := x.doPairing(map[string]any{
		`data`: x.encodeTLV([]PairingTLV{
			{Type: TypeEncryptedData, Data: encrptedData[:255]},
			{Type: TypeEncryptedData, Data: encrptedData[255:]},
			{Type: TypeState, Data: []byte{0x05}},
		}),
		`kind`:            `setupManualPairing`,
		`sendingHost`:     host,
		`startNewSession`: false,
	})
	if err != nil {return err}

	data, err := cip.Open(
		[]byte{},
		[]byte("\x00\x00\x00\x00PS-Msg06"),
		peer[TypeEncryptedData].Data,
		[]byte{},
	)

	if err == nil {
		*tlv = x.decodeTLV(data)
	}

	return err
}

func (x *Service) opack(data map[string]any) []byte {
	const (
		strBot = 0x61
		strOff = 0x40
		binBot = 0x91
		binOff = 0x70
	)

	num := func(n, i, p int, b io.ByteWriter) {
		switch {
		case n+i <= p:
			b.WriteByte(byte(n + i))
		case n <= 0xFF:
			b.WriteByte(byte(p))
			b.WriteByte(byte(n))
		case n <= 0xFFFF:
			b.WriteByte(byte(p + 1))
			b.WriteByte(byte(n >> 0 & 0xFF))
			b.WriteByte(byte(n >> 8 & 0xFF))
		}
	}

	buf := &bytes.Buffer{}
	buf.WriteByte(byte(len(data)) + 0xE0)
	for k, v := range data {
		num(len(k), strOff, strBot, buf)
		buf.WriteString(k)
		switch v := v.(type) {
		case string:
			num(len(v), strOff, strBot, buf)
			buf.WriteString(v)
		case []byte:
			num(len(v), binOff, binBot, buf)
			buf.Write(v)
		}
	}

	return buf.Bytes()
}

func (x *Service) verifyProof(key []byte, salt []byte) error {
	host, _ := os.Hostname()
	g, _ := srp.NewGroup(GROUP3072)
	c, err := srp.NewClient(crypto.SHA512, g,`Pair-Setup`, `000000`)
	if err != nil {return err}
	proof, err := c.ProveIdentity(big.NewInt(0).SetBytes(key), hex.EncodeToString(salt))
	if err != nil {return err}

	_, cpkey := c.Auth()
	cpkeyBuf := cpkey.Bytes()

	peer, err := x.doPairing(map[string]any{
		`data`: x.encodeTLV([]PairingTLV{
			{Type: TypeState, Data: []byte{0x03}},
			{Type: TypePublicKey, Data: cpkeyBuf[:255]},
			{Type: TypePublicKey, Data: cpkeyBuf[255:]},
			{Type: TypeProof, Data: proof.Bytes()},
		}),
		`kind`:            `setupManualPairing`,
		`sendingHost`:     host,
		`startNewSession`: false,
	})
	if err != nil {return err}

	if !c.IsProofValid(big.NewInt(0).SetBytes(peer[TypeProof].Data)) {
		return errors.New(`SERVER PROOF MISMATCH`)
	}

	log.Printf(`PROOF PASS`)
	return nil
}

func (x *Service) encrypt() error {
	panic(``)
}

func (x *Service) createRemoteUnlock() error {
	rsp, err := x.EncryptedQuery(map[string]any{
		`request`: map[string]any {
			`_0`: map[string]any{
				`createRemoteUnlockKey`: map[string]any{},
			},
		},
	})

	if err == nil {
		x.UnlockKey = rsp[`createRemoteUnlockKey`].
		(map[string]any)[`hostKey`].
		([]byte)
	}
	return err
}

func (x *Service) EncryptedQuery(req map[string]any) (map[string]any, error) {
	nonce := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonce, x.sequence)
	err := x.SendEncryptedRequest(req, nonce)
	if err == nil {
		return x.RecvEncryptedResponse(nonce)
	}

	return nil, err
}

func (x *Service) SendEncryptedRequest(msg map[string]any, nonce []byte) error {
	buf := &bytes.Buffer{}
	json.NewEncoder(buf).Encode(msg)
	encryptedData := x.clientCip.Seal([]byte{}, nonce, buf.Bytes(), []byte{})
	err := x.SendRequest(map[string]any{
		`message`: map[string]any{
			`streamEncrypted`: map[string]any{`_0`: encryptedData},
			`originatedBy`:    `host`,
			`sequenceNumber`:  x.sequence,
		},
	})
	x.sequence++
	return err
}

func (x *Service) RecvEncryptedResponse(nonce []byte) (map[string]any, error) {
	msg, err := x.RecvResponse()
	if err != nil { return nil, err }

	encryptedData := msg.
	(map[string]any)[`message`].
	(map[string]any)[`streamEncrypted`].
	(map[string]any)[`_0`].
	([]byte)
	data, err := x.serverCip.Open([]byte{}, nonce, encryptedData, []byte{})
	if err != nil {return nil, err}
	var rsp map[string]any
	err = json.Unmarshal(data, &rsp)
	if err == nil {
		if extend, ok := rsp[`errorExtended`]; ok {
			return nil,
				fmt.Errorf(`%v`,
					extend.
					(map[string]any)[`_0`].
					(map[string]any)[`userInfo`],
				)
		}

		return rsp[`response`].
		(map[string]any)[`_1`].
		(map[string]any), nil
	}

	return nil, err
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