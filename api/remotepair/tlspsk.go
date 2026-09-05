package remotepair

// Pure-Go TLS 1.2 PSK client — adapted from:
// https://github.com/danielpaulus/go-ios/blob/master/ios/tunnel/tlspsk/tlspsk.go
//
// Go's standard crypto/tls deliberately omits all TLS_PSK_WITH_* cipher suites.
// This file implements exactly one suite — TLS_PSK_WITH_AES_256_GCM_SHA384
// (0x00A9) — client-side only, with no certificate, ECDHE or signature handling.
// All security comes from the 32-byte shared secret passed as psk.
//
// References: RFC 4279 (PSK), RFC 5487 (PSK+GCM, SHA-384 PRF),
//             RFC 5246 (TLS 1.2), RFC 5288 (AES-GCM record layer).

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"time"
)

const (
	tlsPSKRecordChangeCipherSpec uint8 = 20
	tlsPSKRecordAlert            uint8 = 21
	tlsPSKRecordHandshake        uint8 = 22
	tlsPSKRecordApplicationData  uint8 = 23

	tlsPSKVersionTLS12 uint16 = 0x0303

	tlsPSKHsClientHello       uint8 = 1
	tlsPSKHsServerHello       uint8 = 2
	tlsPSKHsCertificate       uint8 = 11
	tlsPSKHsServerKeyExchange uint8 = 12
	tlsPSKHsServerHelloDone   uint8 = 14
	tlsPSKHsClientKeyExchange uint8 = 16
	tlsPSKHsFinished          uint8 = 20

	// TLS_PSK_WITH_AES_256_GCM_SHA384 (RFC 5487)
	tlsPSKCipherSuite uint16 = 0x00A9

	tlsPSKGcmTagSize       = 16
	tlsPSKGcmExplicitNonce = 8
	tlsPSKGcmFixedIVLen    = 4
	tlsPSKAES256KeyLen     = 32
	tlsPSKMasterSecretLen  = 48
	tlsPSKVerifyDataLen    = 12
	tlsPSKMaxRecordPayload = 16384
)

func tlsPSKPrfHash() hash.Hash { return sha512.New384() }

// newPSKConn performs a TLS 1.2 plain-PSK handshake over conn using psk as the
// pre-shared key and returns a net.Conn that transparently encrypts/decrypts
// application data. The returned conn takes ownership of the underlying conn.
func newPSKConn(conn net.Conn, psk []byte) (net.Conn, error) {
	c := &tlsPSKConn{conn: conn, psk: psk}
	if err := c.handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	log.Printf(`PSK TLS_PSK_WITH_AES_256_GCM_SHA384 handshake OK %s`, conn.RemoteAddr())
	return c, nil
}

type tlsPSKConn struct {
	conn net.Conn
	psk  []byte

	writeAEAD, readAEAD cipher.AEAD
	writeIV, readIV     [tlsPSKGcmFixedIVLen]byte
	writeSeq, readSeq   uint64
	writeEnc, readEnc   bool

	rawBuf     bytes.Buffer
	hsBuf      bytes.Buffer
	appBuf     bytes.Buffer
	transcript bytes.Buffer
}

func (c *tlsPSKConn) handshake() error {
	clientRandom := make([]byte, 32)
	if _, err := rand.Read(clientRandom); err != nil {
		return err
	}
	if err := c.writeHandshake(c.buildClientHello(clientRandom)); err != nil {
		return fmt.Errorf("tlspsk: write ClientHello: %w", err)
	}

	var serverRandom []byte
	for {
		msgType, body, err := c.readHandshakeMsg()
		if err != nil {
			return fmt.Errorf("tlspsk: read handshake: %w", err)
		}
		switch msgType {
		case tlsPSKHsServerHello:
			sr, err := tlsPSKParseServerHello(body)
			if err != nil {
				return err
			}
			serverRandom = sr
		case tlsPSKHsCertificate, tlsPSKHsServerKeyExchange:
			// plain PSK: no certificate; ServerKeyExchange only carries a PSK
			// identity hint which we ignore.
		case tlsPSKHsServerHelloDone:
			goto donewithserverflight
		default:
			return fmt.Errorf("tlspsk: unexpected handshake message type %d", msgType)
		}
	}
donewithserverflight:
	if serverRandom == nil {
		return errors.New("tlspsk: server never sent ServerHello")
	}

	// ClientKeyExchange: plain PSK body is just the (empty) PSK identity.
	cke := []byte{0x00, 0x00}
	if err := c.writeHandshake(tlsPSKHandshakeMsg(tlsPSKHsClientKeyExchange, cke)); err != nil {
		return fmt.Errorf("tlspsk: write ClientKeyExchange: %w", err)
	}

	pms := tlsPSKPremasterSecret(c.psk)
	master := tlsPSKPrf12(pms, "master secret", append(append([]byte{}, clientRandom...), serverRandom...), tlsPSKMasterSecretLen)
	c.setupKeys(master, clientRandom, serverRandom)

	if err := c.writeRecord(tlsPSKRecordChangeCipherSpec, []byte{0x01}); err != nil {
		return err
	}
	c.writeEnc = true
	clientFinished := tlsPSKHandshakeMsg(tlsPSKHsFinished, c.verifyData(master, "client finished"))
	if err := c.writeHandshake(clientFinished); err != nil {
		return fmt.Errorf("tlspsk: write Finished: %w", err)
	}

	ct, _, err := c.readRecord()
	if err != nil {
		return err
	}
	if ct != tlsPSKRecordChangeCipherSpec {
		return fmt.Errorf("tlspsk: expected ChangeCipherSpec, got record type %d", ct)
	}
	c.readEnc = true
	expected := c.verifyData(master, "server finished")
	msgType, body, err := c.readHandshakeMsg()
	if err != nil {
		return fmt.Errorf("tlspsk: read server Finished: %w", err)
	}
	if msgType != tlsPSKHsFinished {
		return fmt.Errorf("tlspsk: expected server Finished, got %d", msgType)
	}
	if !hmac.Equal(body, expected) {
		return errors.New("tlspsk: server Finished verify_data mismatch")
	}
	return nil
}

func (c *tlsPSKConn) buildClientHello(clientRandom []byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{byte(tlsPSKVersionTLS12 >> 8), byte(tlsPSKVersionTLS12 & 0xff)})
	b.Write(clientRandom)
	b.WriteByte(0) // session_id length
	b.Write([]byte{0x00, 0x02, byte(tlsPSKCipherSuite >> 8), byte(tlsPSKCipherSuite & 0xff)})
	b.Write([]byte{0x01, 0x00}) // compression_methods: 1 method, null

	// Extensions: renegotiation_info (type=0xff01) with empty renegotiated_connection.
	// OpenSSL 3.x rejects ClientHellos that lack this extension (RFC 5746).
	//   extension type  = 0xff01  (2 bytes)
	//   extension len   = 0x0001  (2 bytes) — one byte payload
	//   ri_connection   = 0x00    (1 byte)  — empty, initial handshake
	extBody := []byte{0xff, 0x01, 0x00, 0x01, 0x00}
	// extensions total length prefix (2 bytes)
	b.Write([]byte{0x00, byte(len(extBody))})
	b.Write(extBody)

	return tlsPSKHandshakeMsg(tlsPSKHsClientHello, b.Bytes())
}

func tlsPSKParseServerHello(body []byte) (serverRandom []byte, err error) {
	if len(body) < 35 {
		return nil, errors.New("tlspsk: ServerHello too short")
	}
	serverRandom = append([]byte{}, body[2:34]...)
	sidLen := int(body[34])
	off := 35 + sidLen
	if len(body) < off+3 {
		return nil, errors.New("tlspsk: ServerHello truncated at cipher suite")
	}
	suite := binary.BigEndian.Uint16(body[off : off+2])
	if suite != tlsPSKCipherSuite {
		return nil, fmt.Errorf("tlspsk: server selected unsupported cipher suite 0x%04x", suite)
	}
	return serverRandom, nil
}

func tlsPSKPremasterSecret(psk []byte) []byte {
	n := len(psk)
	pms := make([]byte, 0, 2+n+2+n)
	var l [2]byte
	binary.BigEndian.PutUint16(l[:], uint16(n))
	pms = append(pms, l[:]...)
	pms = append(pms, make([]byte, n)...)
	pms = append(pms, l[:]...)
	pms = append(pms, psk...)
	return pms
}

func (c *tlsPSKConn) setupKeys(master, clientRandom, serverRandom []byte) {
	need := 2*tlsPSKAES256KeyLen + 2*tlsPSKGcmFixedIVLen
	kb := tlsPSKPrf12(master, "key expansion", append(append([]byte{}, serverRandom...), clientRandom...), need)
	clientKey := kb[0:tlsPSKAES256KeyLen]
	serverKey := kb[tlsPSKAES256KeyLen : 2*tlsPSKAES256KeyLen]
	off := 2 * tlsPSKAES256KeyLen
	copy(c.writeIV[:], kb[off:off+tlsPSKGcmFixedIVLen])
	copy(c.readIV[:], kb[off+tlsPSKGcmFixedIVLen:off+2*tlsPSKGcmFixedIVLen])
	c.writeAEAD = tlsPSKNewGCM(clientKey)
	c.readAEAD = tlsPSKNewGCM(serverKey)
}

func tlsPSKNewGCM(key []byte) cipher.AEAD {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return aead
}

func (c *tlsPSKConn) verifyData(master []byte, label string) []byte {
	h := tlsPSKPrfHash()
	h.Write(c.transcript.Bytes())
	return tlsPSKPrf12(master, label, h.Sum(nil), tlsPSKVerifyDataLen)
}

func (c *tlsPSKConn) writeHandshake(msg []byte) error {
	c.transcript.Write(msg)
	return c.writeRecord(tlsPSKRecordHandshake, msg)
}

func (c *tlsPSKConn) writeRecord(contentType uint8, payload []byte) error {
	var fragment []byte
	if c.writeEnc && contentType != tlsPSKRecordChangeCipherSpec {
		var explicit [tlsPSKGcmExplicitNonce]byte
		binary.BigEndian.PutUint64(explicit[:], c.writeSeq)
		nonce := append(append([]byte{}, c.writeIV[:]...), explicit[:]...)
		aad := tlsPSKMakeAAD(c.writeSeq, contentType, len(payload))
		sealed := c.writeAEAD.Seal(nil, nonce, payload, aad)
		fragment = append(explicit[:], sealed...)
		c.writeSeq++
	} else {
		fragment = payload
	}
	hdr := []byte{
		contentType,
		byte(tlsPSKVersionTLS12 >> 8), byte(tlsPSKVersionTLS12 & 0xff),
		byte(len(fragment) >> 8), byte(len(fragment)),
	}
	if _, err := c.conn.Write(append(hdr, fragment...)); err != nil {
		return err
	}
	return nil
}

func (c *tlsPSKConn) readRecord() (uint8, []byte, error) {
	hdr, err := c.readN(5)
	if err != nil {
		return 0, nil, err
	}
	contentType := hdr[0]
	length := int(binary.BigEndian.Uint16(hdr[3:5]))
	if length > tlsPSKMaxRecordPayload+2048 {
		return 0, nil, fmt.Errorf("tlspsk: oversized record %d", length)
	}
	fragment, err := c.readN(length)
	if err != nil {
		return 0, nil, err
	}
	if c.readEnc && contentType != tlsPSKRecordChangeCipherSpec {
		if len(fragment) < tlsPSKGcmExplicitNonce+tlsPSKGcmTagSize {
			return 0, nil, errors.New("tlspsk: short encrypted record")
		}
		explicit := fragment[:tlsPSKGcmExplicitNonce]
		nonce := append(append([]byte{}, c.readIV[:]...), explicit...)
		ciphertext := fragment[tlsPSKGcmExplicitNonce:]
		aad := tlsPSKMakeAAD(c.readSeq, contentType, len(ciphertext)-tlsPSKGcmTagSize)
		plain, err := c.readAEAD.Open(nil, nonce, ciphertext, aad)
		if err != nil {
			return 0, nil, fmt.Errorf("tlspsk: record decrypt failed: %w", err)
		}
		c.readSeq++
		fragment = plain
	}
	if contentType == tlsPSKRecordAlert {
		if len(fragment) >= 2 {
			return 0, nil, fmt.Errorf("tlspsk: received alert level=%d description=%d", fragment[0], fragment[1])
		}
		return 0, nil, errors.New("tlspsk: received alert")
	}
	return contentType, fragment, nil
}

func (c *tlsPSKConn) readHandshakeMsg() (uint8, []byte, error) {
	for c.hsBuf.Len() < 4 {
		if err := c.fillHandshakeBuf(); err != nil {
			return 0, nil, err
		}
	}
	header := c.hsBuf.Bytes()[:4]
	msgType := header[0]
	bodyLen := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	for c.hsBuf.Len() < 4+bodyLen {
		if err := c.fillHandshakeBuf(); err != nil {
			return 0, nil, err
		}
	}
	full := make([]byte, 4+bodyLen)
	_, _ = io.ReadFull(&c.hsBuf, full)
	c.transcript.Write(full)
	return msgType, full[4:], nil
}

func (c *tlsPSKConn) fillHandshakeBuf() error {
	ct, payload, err := c.readRecord()
	if err != nil {
		return err
	}
	if ct != tlsPSKRecordHandshake {
		return fmt.Errorf("tlspsk: expected handshake record, got content type %d", ct)
	}
	c.hsBuf.Write(payload)
	return nil
}

func (c *tlsPSKConn) readN(n int) ([]byte, error) {
	for c.rawBuf.Len() < n {
		tmp := make([]byte, 4096)
		m, err := c.conn.Read(tmp)
		if m > 0 {
			c.rawBuf.Write(tmp[:m])
		}
		if err != nil {
			if c.rawBuf.Len() >= n {
				break
			}
			return nil, err
		}
	}
	out := make([]byte, n)
	_, _ = io.ReadFull(&c.rawBuf, out)
	return out, nil
}

func tlsPSKMakeAAD(seq uint64, contentType uint8, plaintextLen int) []byte {
	aad := make([]byte, 13)
	binary.BigEndian.PutUint64(aad[0:8], seq)
	aad[8] = contentType
	aad[9] = byte(tlsPSKVersionTLS12 >> 8)
	aad[10] = byte(tlsPSKVersionTLS12 & 0xff)
	binary.BigEndian.PutUint16(aad[11:13], uint16(plaintextLen))
	return aad
}

func tlsPSKHandshakeMsg(msgType uint8, body []byte) []byte {
	out := make([]byte, 4+len(body))
	out[0] = msgType
	out[1] = byte(len(body) >> 16)
	out[2] = byte(len(body) >> 8)
	out[3] = byte(len(body))
	copy(out[4:], body)
	return out
}

func tlsPSKPrf12(secret []byte, label string, seed []byte, length int) []byte {
	labelSeed := make([]byte, 0, len(label)+len(seed))
	labelSeed = append(labelSeed, label...)
	labelSeed = append(labelSeed, seed...)
	out := make([]byte, length)
	tlsPSKPHash(out, secret, labelSeed)
	return out
}

func tlsPSKPHash(result, secret, seed []byte) {
	h := hmac.New(sha512.New384, secret)
	h.Write(seed)
	a := h.Sum(nil)
	for len(result) > 0 {
		h.Reset()
		h.Write(a)
		h.Write(seed)
		b := h.Sum(nil)
		n := copy(result, b)
		result = result[n:]
		h.Reset()
		h.Write(a)
		a = h.Sum(nil)
	}
}

func (c *tlsPSKConn) Read(p []byte) (int, error) {
	for c.appBuf.Len() == 0 {
		ct, payload, err := c.readRecord()
		if err != nil {
			return 0, err
		}
		switch ct {
		case tlsPSKRecordApplicationData:
			c.appBuf.Write(payload)
		case tlsPSKRecordHandshake:
			// ignore post-handshake messages
		default:
			return 0, fmt.Errorf("tlspsk: unexpected record type %d during Read", ct)
		}
	}
	return c.appBuf.Read(p)
}

func (c *tlsPSKConn) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > tlsPSKMaxRecordPayload {
			chunk = chunk[:tlsPSKMaxRecordPayload]
		}
		if err := c.writeRecord(tlsPSKRecordApplicationData, chunk); err != nil {
			return written, err
		}
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}

func (c *tlsPSKConn) Close() error                       { return c.conn.Close() }
func (c *tlsPSKConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *tlsPSKConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *tlsPSKConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *tlsPSKConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *tlsPSKConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
