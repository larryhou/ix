package remotepair

import (
	"bytes"
	"context"
	"crypto"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/larryhou/ix/api/tunnel"
	"github.com/larryhou/ix/api/tunnel/rsd"
	"github.com/larryhou/ix/api/tunnel/xpc"
	"github.com/larryhou/srp"
	"github.com/mitchellh/mapstructure"
	"github.com/quic-go/quic-go"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"howett.net/plist"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	WireProtocolVersion = 19
)

type PairType int

const (
	PairTypeWire PairType = iota
	PairTypeWiFi
)

type PairRecord struct {
	Ed25519Key ed25519.PrivateKey
	HostKey    string
}

func New(addr *net.TCPAddr, typ PairType, ops ...func(s *Service)) (*Service, error) {
	s := &Service{}
	s.id = s.generateHostID()

	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {return nil, err}
	s.privateKey = privateKey

	switch typ {
	case PairTypeWire:
		conn, err := xpc.NewRemoteXpc(addr)
		if err != nil {return nil, err}
		s.pairConnection = &wirePairConnection{
			addr:    addr,
			xpcConn: conn,
		}

	case PairTypeWiFi:
		conn, err := net.Dial(`tcp`, addr.String())
		if err != nil {return nil, err}
		s.pairConnection = &wifiPairConnection{
			addr:    addr,
			netConn: conn,
		}

	default:
		return nil, errors.New(`unknown pair type`)
	}

	for _, op := range ops {op(s)}
	return s, s.connect()
}

func NewFromRSD(r *rsd.Service) (*Service, error) {
	addr, err := r.GetServiceAddr(rsd.ComAppleInternalDtCoredeviceUntrustedTunnelservice, true, nil)
	if err != nil {return nil, err}
	return New(addr, PairTypeWire)
}

type Service struct {
	pairConnection
	*Descriptor
	*PairRecord
	Udid string

	privateKey *ecdh.PrivateKey
	encryptKey []byte

	serverCip cipher.AEAD
	clientCip cipher.AEAD

	tcpTun  *tunnel.Service
	quicTun *tunnel.Service

	sc *srp.Client
	id string
	sn uint64
	en uint64
}

func (x *Service) connect() error {
	err := x.handshake()
	if err == nil {
		if err = x.validate(); err != nil {
			if x.pairConnection.canPair() {
				err = x.pair()
			}
		}
	}

	if err == nil {
		err = x.initCipherKeys()
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

	err := x.sendPlainRequest(map[string]any{
		`request`: map[string]any{
			`_0`: map[string]any{
				`handshake`: map[string]any{
					`_0`: req,
				},
			},
		},
	})

	if err != nil {return err}
	msg, err := x.recvPlainResponse()
	if err != nil {return err}

	rsp := msg[`response`].
	(map[string]any)[`_1`].
	(map[string]any)[`handshake`].
	(map[string]any)[`_0`]

	des := &Descriptor{}
	err = mapstructure.Decode(rsp, des)
	if err == nil {
		x.Descriptor = des
		if des.PeerDeviceInfo != nil {
			x.Udid = des.PeerDeviceInfo.Udid
		}

		if len(x.Udid) != 0 {
			x.retrieve()
		}
	}
	return err
}

func (x *Service) encodeTLV(tlv []*PairingTLV) []byte {
	buf := &bytes.Buffer{}
	for _, it := range tlv {
		buf.WriteByte(it.Type)
		buf.WriteByte(byte(len(it.Data)))
		buf.Write(it.Data)
	}
	return buf.Bytes()
}

func (x *Service) decodeTLV(b []byte) map[byte]*PairingTLV {
	out := make(map[byte]*PairingTLV)
	for p := 0; p < len(b); {
		typ := b[p]
		p++
		num := int(b[p])
		p++
		if v, ok := out[typ]; ok {
			data := make([]byte, len(v.Data)+num)
			copy(data, v.Data)
			copy(data[len(v.Data):], b[p:p+num])
			v.Data = data
		} else {
			out[typ] = &PairingTLV{
				Data: b[p:p+num],
				Type: typ,
			}
		}

		p += num
	}

	return out
}

func (x *Service) doPairing(req any) (map[byte]*PairingTLV, error) {
	err := x.sendPlainRequest(map[string]any{
		`event`: map[string]any{
			`_0`: map[string]any{
				`pairingData`: map[string]any{`_0`: req},
			},
		},
	})

	if err != nil {return nil, err}
	return x.recvPairingResponse()
}

type PairError []byte

func (x PairError) Error() string {
	return fmt.Sprintf(`PairError(%s)`, hex.EncodeToString(x))
}

func (x *Service) recvPairingResponse() (map[byte]*PairingTLV, error) {
	msg, err := x.recvPlainResponse()
	if err != nil {return nil, err}
	rsp := msg[`event`].
	(map[string]any)[`_0`].
	(map[string]any)

	if err, ok := rsp[`pairingRejectedWithError`]; ok {
		return nil, fmt.Errorf(`pairing rejected: %v`, err)
	}

	var peer map[byte]*PairingTLV
	if _, ok := rsp[`awaitingUserConsent`]; ok {
		log.Printf(`WAITING USER CONSENT...`)
		return x.recvPairingResponse()
	}

	if data, ok := rsp[`pairingData`]; !ok {
		return nil, errors.New(`no pairingData field`)
	} else {
		peer = x.decodeTLV(x.bytes(data.
		(map[string]any)[`_0`].
		(map[string]any)[`data`]))
		if r, ok := peer[TypeError]; ok {
			return peer, PairError(r.Data)
		}

		return peer, nil
	}
}

func (x *Service) verifyPairing(err error) error {
	if _, ok := err.(PairError); ok {
		x.sendPlainRequest(map[string]any{
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
	tlv := []*PairingTLV{
		{Type: TypeState, Data: []byte{0x01}},
		{Type: TypePublicKey, Data: x.privateKey.PublicKey().Bytes()},
	}

	peer, err := x.doPairing(map[string]any{
		`data`:            x.encodeTLV(tlv),
		`kind`:            `verifyManualPairing`,
		`startNewSession`: true,
	})

	if err = x.verifyPairing(err); err != nil {return err }

	pearPublicKey, err := ecdh.X25519().NewPublicKey(peer[TypePublicKey].Data)
	if err != nil {return err}
	x.encryptKey, err = x.privateKey.ECDH(pearPublicKey)
	if err != nil {return err}

	key := make([]byte, 32)
	_, err = io.ReadFull(hkdf.New(
		sha512.New,
		x.encryptKey,
		[]byte(`Pair-Verify-Encrypt-Salt`),
		[]byte(`Pair-Verify-Encrypt-Info`),
	), key)

	cip, err := chacha20poly1305.New(key)
	if err != nil {return err}

	var privateKey ed25519.PrivateKey
	if x.PairRecord == nil {
		privateKey = make(ed25519.PrivateKey, 0x40)
	} else {
		privateKey = x.PairRecord.Ed25519Key
	}

	buf := &bytes.Buffer{}
	buf.Write(x.privateKey.PublicKey().Bytes())
	buf.WriteString(x.id)
	buf.Write(pearPublicKey.Bytes())

	signature := ed25519.Sign(privateKey, buf.Bytes())
	encryptedData := cip.Seal(
		[]byte{},
		[]byte("\x00\x00\x00\x00PV-Msg03"),
		x.encodeTLV([]*PairingTLV{
			{Type: TypeIdentifier, Data: []byte(x.id)},
			{Type: TypeSignature, Data: signature},
		}),
		[]byte{},
	)

	paringData := x.encodeTLV([]*PairingTLV{
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

func (x *Service) pair() error {
	host, _ := os.Hostname()
	peer, err := x.doPairing(map[string]any{
		`data`: x.encodeTLV([]*PairingTLV{
			{Type: TypeMethod, Data: []byte{0x00}},
			{Type: TypeState, Data: []byte{0x01}},
		}),
		`kind`:            `setupManualPairing`,
		`sendingHost`:     host,
		`startNewSession`: true,
	})
	if err != nil {return err}

	err = x.verifyProof(peer[TypePublicKey].Data, peer[TypeSalt].Data)
	if err != nil {return err}

	var tlv map[byte]*PairingTLV
	err = x.applyPairing(&tlv)
	if err != nil {return err}

	err = x.initCipherKeys()
	if err == nil {
		err = x.createUnlockKey()
	}

	if err == nil {
		err = x.cache()
	}

	return err
}

func workspace() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, `.j3idevice`)
}

func ListUdid() []string {
	var udid []string
	root := workspace()
	items, _ := os.ReadDir(root)
	for _, ent := range items {
		name := ent.Name()
		if strings.HasPrefix(name, `RP_`) && strings.HasSuffix(name, `.plist`) {
			udid = append(udid, name[3:len(name)-6])
		}
	}

	return udid
}

func (x *Service) cache() error {
	root := workspace()
	name := fmt.Sprintf(`RP_%s.plist`, x.Udid)
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
	name := fmt.Sprintf(`RP_%s.plist`, x.Udid)
	f, err := os.Open(filepath.Join(workspace(), name))
	if err != nil {return err}
	defer f.Close()

	rp := &PairRecord{}
	err = plist.NewDecoder(f).Decode(rp)
	if err == nil {
		if len(rp.Ed25519Key) == ed25519.PrivateKeySize {
			x.PairRecord = rp
		}
	}

	return err
}

func (x *Service) initCipherKeys() error {
	clientKey := make([]byte, 32)
	_, err := io.ReadFull(
		hkdf.New(sha512.New, x.encryptKey, nil, []byte(`ClientEncrypt-main`)),
		clientKey,
	)
	x.clientCip, err = chacha20poly1305.New(clientKey)
	if err != nil {return err}

	serverKey := make([]byte, 32)
	_, err = io.ReadFull(
		hkdf.New(sha512.New, x.encryptKey, nil, []byte(`ServerEncrypt-main`)),
		serverKey,
	)
	x.serverCip, err = chacha20poly1305.New(serverKey)
	return err
}

func (x *Service) applyPairing(tlv *map[byte]*PairingTLV) error {
	host, _ := os.Hostname()
	setupEncryptKey := make([]byte, 32)
	_, err := io.ReadFull(hkdf.New(
		sha512.New,
		x.encryptKey,
		[]byte(`Pair-Setup-Encrypt-Salt`),
		[]byte(`Pair-Setup-Encrypt-Info`),
	), setupEncryptKey)
	if err != nil {return err}

	signKey := make([]byte, 32)
	_, err = io.ReadFull(hkdf.New(
		sha512.New,
		x.encryptKey,
		[]byte(`Pair-Setup-Controller-Sign-Salt`),
		[]byte(`Pair-Setup-Controller-Sign-Info`),
	), signKey)
	if err != nil {return err}

	x.PairRecord = &PairRecord{}
	_, x.Ed25519Key, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {return err}

	buf := &bytes.Buffer{}
	buf.Write(signKey)
	buf.WriteString(x.id)
	buf.Write(x.Ed25519Key.Public().(ed25519.PublicKey))
	signature, err := x.Ed25519Key.Sign(rand.Reader, buf.Bytes(), crypto.Hash(0))
	if err != nil {return err}

	info := x.pack(map[string]any{
		`altIRK`:                      []byte("\xe9\xe8-\xc0jIykVoT\x00\x19\xb1\xc7{"),
		`btAddr`:                      `11:22:33:44:55:66`,
		`mac`:                         []byte("\x11\x22\x33\x44\x55\x66"),
		`remotepairing_serial_number`: `AAAAAAAAAAAA`,
		`accountID`:                   x.id,
		`model`:                       `computer-model`,
		`name`:                        host,
	})

	cip, err := chacha20poly1305.New(setupEncryptKey)
	if err != nil {return err}

	encrptedData := cip.Seal(
		[]byte{},
		[]byte("\x00\x00\x00\x00PS-Msg05"),
		x.encodeTLV([]*PairingTLV{
			{Type: TypeIdentifier, Data: []byte(x.id)},
			{Type: TypePublicKey, Data: x.Ed25519Key.Public().(ed25519.PublicKey)},
			{Type: TypeSignature, Data: signature},
			{Type: TypeInfo, Data: info},
		}),
		[]byte{},
	)

	peer, err := x.doPairing(map[string]any{
		`data`: x.encodeTLV([]*PairingTLV{
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

func (x *Service) pack(data map[string]any) []byte {
	const (
		strLow = 0x61
		strOff = 0x40
		binLow = 0x91
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
		num(len(k), strOff, strLow, buf)
		buf.WriteString(k)
		switch v := v.(type) {
		case string:
			num(len(v), strOff, strLow, buf)
			buf.WriteString(v)
		case []byte:
			num(len(v), binOff, binLow, buf)
			buf.Write(v)
		}
	}

	return buf.Bytes()
}

func (x *Service) verifyProof(skey []byte, salt []byte) error {
	host, _ := os.Hostname()
	g, _ := srp.NewGroup(srp.Group3072)
	c, err := srp.NewClient(crypto.SHA512, g,`Pair-Setup`, `000000`)
	if err != nil {return err}
	proof, err := c.ProveIdentity(new(big.Int).SetBytes(skey), string(salt))
	if err != nil {return err}

	h := c.H.New()
	h.Write(c.PremasterKey.Bytes())
	x.encryptKey = h.Sum(nil)

	_, pkey := c.Auth()
	pkeyBuf := pkey.Bytes()

	peer, err := x.doPairing(map[string]any{
		`data`: x.encodeTLV([]*PairingTLV{
			{Type: TypeState, Data: []byte{0x03}},
			{Type: TypePublicKey, Data: pkeyBuf[:255]},
			{Type: TypePublicKey, Data: pkeyBuf[255:]},
			{Type: TypeProof, Data: proof.Bytes()},
		}),
		`kind`:            `setupManualPairing`,
		`sendingHost`:     host,
		`startNewSession`: false,
	})
	if err != nil {return err}

	if !c.IsProofValid(new(big.Int).SetBytes(peer[TypeProof].Data)) {
		return errors.New(`SERVER PROOF MISMATCH`)
	}
	return nil
}

func (x *Service) createUnlockKey() error {
	rsp, err := x.secureQuery(map[string]any{
		`createRemoteUnlockKey`: map[string]any{},
	})
	if err == nil {
		if data, ok := rsp[`createRemoteUnlockKey`]; ok {
			x.HostKey = data.(map[string]any)[`hostKey`].(string)
		}
	}
	return err
}

func (x *Service) secureQuery(req map[string]any) (map[string]any, error) {
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce, x.en)
	err := x.sendSecureRequest(map[string]any{
		`request`: map[string]any{`_0`: req},
	}, nonce)
	if err != nil {return nil, err}

	rsp, err := x.recvSecureResponse(nonce)
	if err == nil {
		return rsp[`response`].
		(map[string]any)[`_1`].
		(map[string]any), nil
	}

	return nil, err
}

func (x *Service) sendSecureRequest(msg map[string]any, nonce []byte) error {
	buf := &bytes.Buffer{}
	json.NewEncoder(buf).Encode(msg)
	encryptedData := x.clientCip.Seal([]byte{}, nonce, buf.Bytes(), []byte{})
	err := x.sendRequest(map[string]any{
		`message`: map[string]any{
			`streamEncrypted`: map[string]any{`_0`: encryptedData},
		},
		`originatedBy`:   `host`,
		`sequenceNumber`: x.sn,
	})
	x.en++
	x.sn++
	return err
}

func (x *Service) recvSecureResponse(nonce []byte) (map[string]any, error) {
	msg, err := x.recvResponse()
	if err != nil { return nil, err }

	encryptedData := x.bytes(msg.
	(map[string]any)[`message`].
	(map[string]any)[`streamEncrypted`].
	(map[string]any)[`_0`])
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

		return rsp, nil
	}

	return nil, err
}

func (x *Service) sendPlainRequest(msg map[string]any) error {
	data := map[string]any{
		`message`: map[string]any{
			`plain`: map[string]any{`_0`: msg},
		},
		`originatedBy`:   `host`,
		`sequenceNumber`: x.sn,
	}
	x.sn++
	return x.sendRequest(data)
}

func (x *Service) recvPlainResponse() (map[string]any, error) {
	msg, err := x.recvResponse()
	if err == nil {
		rsp := msg.
		(map[string]any)[`message`].
		(map[string]any)[`plain`].
		(map[string]any)[`_0`].
		(map[string]any)

		return rsp, nil
	}

	return nil, err
}

func (x *Service) createQuicListener(key *rsa.PublicKey) (map[string]any, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {return nil, err}

	rsp, err := x.secureQuery(map[string]any{
		`createListener`: map[string]any{
			`key`: base64.StdEncoding.EncodeToString(der),
			`peerConnectionsInfo`: []any{
				map[string]any{
					`owningPID`:         os.Getpid(),
					`owningProcessName`: `CoreDeviceService`,
				},
			},
			`transportProtocolType`: `quic`,
		},
	})

	if err == nil {
		return rsp[`createListener`].(map[string]any), nil
	}
	return nil, err
}

func (x *Service) createTcpListener() (map[string]any, error) {
	rsp, err := x.secureQuery(map[string]any{
		`createListener`: map[string]any{
			`key`: base64.StdEncoding.EncodeToString(x.encryptKey),
			`peerConnectionsInfo`: []any{
				map[string]any{
					`owningPID`:         os.Getpid(),
					`owningProcessName`: `CoreDeviceService`,
				},
			},
			`transportProtocolType`: `tcp`,
		},
	})

	if err == nil {
		return rsp[`createListener`].(map[string]any), nil
	}
	return nil, err
}

type scopeType int
const (
	scopeTypeQuic scopeType = (1 + iota) << 16
	scopeTypeConn
)

type scopeError struct {
	scope scopeType
	error
}

func (x *scopeError) Error() string {
	return fmt.Sprintf(`0x%X %+v`, x.scope, x.error)
}

func (x *Service) StartQuicTunnel() error {
	if x.quicTun != nil {return nil}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {return &scopeError{scope: scopeTypeQuic + 1, error: err}}

	rsp, err := x.createQuicListener(&key.PublicKey)
	if err != nil {return &scopeError{scope: scopeTypeQuic + 2, error: err}}
	log.Printf(`LISTENER %+v`, rsp)

	addr := *x.pairConnection.tcpAddr()
	addr.Port = int(rsp[`port`].(float64))

	tpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{},
		Issuer:                pkix.Name{},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SignatureAlgorithm:    x509.SHA256WithRSA,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	crt, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &key.PublicKey, key)
	if err != nil {return &scopeError{scope: scopeTypeQuic + 3, error: err}}

	keyPem := pem.EncodeToMemory(&pem.Block{Type: `RSA PRIVATE KEY`, Bytes: x509.MarshalPKCS1PrivateKey(key)})
	crtPem := pem.EncodeToMemory(&pem.Block{Type: `CERTIFICATE`, Bytes: crt})
	tlsCert, err := tls.X509KeyPair(crtPem, keyPem)
	if err != nil {return &scopeError{scope: scopeTypeQuic + 4, error: err}}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{tlsCert},
		ClientAuth:         tls.NoClientCert,
		NextProtos:         []string{`RemotePairingTunnelProtocol`},
		CurvePreferences:   []tls.CurveID{tls.CurveP256},
	}
	log.Printf(`DIAL %v`, addr)
	conn, err := quic.DialAddr(context.Background(), addr.String(), tlsConfig, &quic.Config{
		EnableDatagrams:         true,
		KeepAlivePeriod:         time.Second,
		//DisablePathMTUDiscovery: true,
		//InitialPacketSize:       tunnel.MtuUdp,
	})
	if err != nil {return &scopeError{scope: scopeTypeQuic + 5, error: err}}

	err = conn.SendDatagram(make([]byte, 1024))
	if err != nil {return &scopeError{scope: scopeTypeQuic + 6, error: err}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := conn.OpenStream()
	if err != nil {return &scopeError{scope: scopeTypeQuic + 7, error: err}}

	x.quicTun, err = tunnel.New(stream, tunnel.MtuUdp-40, ctx)
	if err != nil {return &scopeError{scope: scopeTypeQuic + 8, error: err}}

	err = x.quicTun.Start(conn)
	if err != nil {return &scopeError{scope: scopeTypeQuic + 9, error: err}}
	return nil
}

func (x *Service) StartTcpTunnel() error {
	if x.tcpTun != nil {return nil}

	rsp, err := x.createTcpListener()
	if err != nil {return err}

	addr := *x.pairConnection.tcpAddr()
	addr.Port = int(rsp[`port`].(float64))
	tlsConn, err := NewPSKConn(x.encryptKey, &addr)

	if err == nil {
		x.tcpTun, err = tunnel.New(tlsConn, tunnel.MtuTcp, context.Background())
	}

	if err == nil {
		err = x.tcpTun.Start(tlsConn)
	}
	return err
}

func (x *Service) StopTcpTunnel() {
	if x.tcpTun != nil {
		x.tcpTun.Stop()
		x.tcpTun = nil
	}
}

func (x *Service) StopQuicTunnel() {
	if x.quicTun != nil {
		x.quicTun.Stop()
		x.quicTun = nil
	}
}

func (x *Service) Tunnel() *tunnel.Service {
	if x.quicTun != nil {return x.quicTun}
	return x.tcpTun
}