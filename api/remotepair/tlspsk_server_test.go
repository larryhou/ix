package remotepair

// tlsPSKServer is a minimal TLS 1.2 PSK *server* used only in tests.
// It mirrors the client handshake in tlspsk.go so that TestPSKConn_SelfPipe
// can run entirely in-process without openssl.

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
	"io"
	"net"
)

type tlsPSKServer struct {
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

func (s *tlsPSKServer) handshake() error {
	// 1. Read ClientHello
	msgType, body, err := s.readHandshakeMsg()
	if err != nil {
		return fmt.Errorf("server: read ClientHello: %w", err)
	}
	if msgType != tlsPSKHsClientHello {
		return fmt.Errorf("server: expected ClientHello, got %d", msgType)
	}
	// body: version(2) random(32) sid_len(1) cipher_suites_len(2) suites(n) …
	if len(body) < 35 {
		return errors.New("server: ClientHello too short")
	}
	clientRandom := make([]byte, 32)
	copy(clientRandom, body[2:34])

	// 2. Send ServerHello with our cipher suite
	serverRandom := make([]byte, 32)
	if _, err := rand.Read(serverRandom); err != nil {
		return err
	}
	var sh bytes.Buffer
	sh.Write([]byte{byte(tlsPSKVersionTLS12 >> 8), byte(tlsPSKVersionTLS12 & 0xff)})
	sh.Write(serverRandom)
	sh.WriteByte(0) // session_id length = 0
	sh.Write([]byte{byte(tlsPSKCipherSuite >> 8), byte(tlsPSKCipherSuite & 0xff)})
	sh.WriteByte(0) // compression = null
	// renegotiation_info extension (RFC 5746): type=0xff01, len=1, ri_len=0.
	// OpenSSL 3.x clients reject ServerHellos that omit this extension.
	sh.Write([]byte{0x00, 0x05, 0xff, 0x01, 0x00, 0x01, 0x00})
	if err := s.writeHandshake(tlsPSKHandshakeMsg(tlsPSKHsServerHello, sh.Bytes())); err != nil {
		return fmt.Errorf("server: write ServerHello: %w", err)
	}

	// 3. Send ServerHelloDone (no Certificate, no ServerKeyExchange for plain PSK)
	if err := s.writeHandshake(tlsPSKHandshakeMsg(tlsPSKHsServerHelloDone, []byte{})); err != nil {
		return fmt.Errorf("server: write ServerHelloDone: %w", err)
	}

	// 4. Read ClientKeyExchange
	msgType, _, err = s.readHandshakeMsg()
	if err != nil {
		return fmt.Errorf("server: read ClientKeyExchange: %w", err)
	}
	if msgType != tlsPSKHsClientKeyExchange {
		return fmt.Errorf("server: expected ClientKeyExchange, got %d", msgType)
	}

	// Derive keys
	pms := tlsPSKPremasterSecret(s.psk)
	master := tlsPSKPrf12(pms, "master secret", append(append([]byte{}, clientRandom...), serverRandom...), tlsPSKMasterSecretLen)
	s.setupKeys(master, clientRandom, serverRandom)

	// 5. Read client ChangeCipherSpec
	ct, _, err := s.readRecord()
	if err != nil {
		return err
	}
	if ct != tlsPSKRecordChangeCipherSpec {
		return fmt.Errorf("server: expected client ChangeCipherSpec, got %d", ct)
	}
	s.readEnc = true

	// 6. Read client Finished
	expected := s.verifyData(master, "client finished")
	msgType, body, err = s.readHandshakeMsg()
	if err != nil {
		return fmt.Errorf("server: read client Finished: %w", err)
	}
	if msgType != tlsPSKHsFinished {
		return fmt.Errorf("server: expected client Finished, got %d", msgType)
	}
	if !hmac.Equal(body, expected) {
		return errors.New("server: client Finished verify_data mismatch")
	}

	// 7. Send server ChangeCipherSpec + Finished
	if err := s.writeRecord(tlsPSKRecordChangeCipherSpec, []byte{0x01}); err != nil {
		return err
	}
	s.writeEnc = true
	serverFinished := tlsPSKHandshakeMsg(tlsPSKHsFinished, s.verifyData(master, "server finished"))
	if err := s.writeHandshake(serverFinished); err != nil {
		return fmt.Errorf("server: write server Finished: %w", err)
	}

	return nil
}

func (s *tlsPSKServer) setupKeys(master, clientRandom, serverRandom []byte) {
	need := 2*tlsPSKAES256KeyLen + 2*tlsPSKGcmFixedIVLen
	kb := tlsPSKPrf12(master, "key expansion", append(append([]byte{}, serverRandom...), clientRandom...), need)
	clientKey := kb[0:tlsPSKAES256KeyLen]
	serverKey := kb[tlsPSKAES256KeyLen : 2*tlsPSKAES256KeyLen]
	off := 2 * tlsPSKAES256KeyLen
	// Server reads client's data → server readAEAD uses clientKey
	// Server writes → server writeAEAD uses serverKey
	copy(s.readIV[:], kb[off:off+tlsPSKGcmFixedIVLen])
	copy(s.writeIV[:], kb[off+tlsPSKGcmFixedIVLen:off+2*tlsPSKGcmFixedIVLen])
	s.readAEAD = srvNewGCM(clientKey)
	s.writeAEAD = srvNewGCM(serverKey)
}

func srvNewGCM(key []byte) cipher.AEAD {
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return aead
}

func (s *tlsPSKServer) verifyData(master []byte, label string) []byte {
	h := sha512.New384()
	h.Write(s.transcript.Bytes())
	return tlsPSKPrf12(master, label, h.Sum(nil), tlsPSKVerifyDataLen)
}

func (s *tlsPSKServer) writeHandshake(msg []byte) error {
	s.transcript.Write(msg)
	return s.writeRecord(tlsPSKRecordHandshake, msg)
}

func (s *tlsPSKServer) writeRecord(contentType uint8, payload []byte) error {
	var fragment []byte
	if s.writeEnc && contentType != tlsPSKRecordChangeCipherSpec {
		var explicit [tlsPSKGcmExplicitNonce]byte
		binary.BigEndian.PutUint64(explicit[:], s.writeSeq)
		nonce := append(append([]byte{}, s.writeIV[:]...), explicit[:]...)
		aad := tlsPSKMakeAAD(s.writeSeq, contentType, len(payload))
		sealed := s.writeAEAD.Seal(nil, nonce, payload, aad)
		fragment = append(explicit[:], sealed...)
		s.writeSeq++
	} else {
		fragment = payload
	}
	hdr := []byte{
		contentType,
		byte(tlsPSKVersionTLS12 >> 8), byte(tlsPSKVersionTLS12 & 0xff),
		byte(len(fragment) >> 8), byte(len(fragment)),
	}
	_, err := s.conn.Write(append(hdr, fragment...))
	return err
}

func (s *tlsPSKServer) readRecord() (uint8, []byte, error) {
	hdr, err := s.readN(5)
	if err != nil {
		return 0, nil, err
	}
	contentType := hdr[0]
	length := int(binary.BigEndian.Uint16(hdr[3:5]))
	fragment, err := s.readN(length)
	if err != nil {
		return 0, nil, err
	}
	if s.readEnc && contentType != tlsPSKRecordChangeCipherSpec {
		explicit := fragment[:tlsPSKGcmExplicitNonce]
		nonce := append(append([]byte{}, s.readIV[:]...), explicit...)
		ciphertext := fragment[tlsPSKGcmExplicitNonce:]
		aad := tlsPSKMakeAAD(s.readSeq, contentType, len(ciphertext)-tlsPSKGcmTagSize)
		plain, err := s.readAEAD.Open(nil, nonce, ciphertext, aad)
		if err != nil {
			return 0, nil, fmt.Errorf("server: decrypt failed: %w", err)
		}
		s.readSeq++
		fragment = plain
	}
	if contentType == tlsPSKRecordAlert {
		return 0, nil, fmt.Errorf("server: received alert %v", fragment)
	}
	return contentType, fragment, nil
}

func (s *tlsPSKServer) readHandshakeMsg() (uint8, []byte, error) {
	for s.hsBuf.Len() < 4 {
		ct, payload, err := s.readRecord()
		if err != nil {
			return 0, nil, err
		}
		if ct != tlsPSKRecordHandshake {
			return 0, nil, fmt.Errorf("server: expected handshake, got %d", ct)
		}
		s.hsBuf.Write(payload)
	}
	header := s.hsBuf.Bytes()[:4]
	msgType := header[0]
	bodyLen := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	for s.hsBuf.Len() < 4+bodyLen {
		ct, payload, err := s.readRecord()
		if err != nil {
			return 0, nil, err
		}
		if ct != tlsPSKRecordHandshake {
			return 0, nil, fmt.Errorf("server: expected handshake, got %d", ct)
		}
		s.hsBuf.Write(payload)
	}
	full := make([]byte, 4+bodyLen)
	io.ReadFull(&s.hsBuf, full)
	s.transcript.Write(full)
	return msgType, full[4:], nil
}

func (s *tlsPSKServer) readN(n int) ([]byte, error) {
	for s.rawBuf.Len() < n {
		tmp := make([]byte, 4096)
		m, err := s.conn.Read(tmp)
		if m > 0 {
			s.rawBuf.Write(tmp[:m])
		}
		if err != nil {
			if s.rawBuf.Len() >= n {
				break
			}
			return nil, err
		}
	}
	out := make([]byte, n)
	io.ReadFull(&s.rawBuf, out)
	return out, nil
}

func (s *tlsPSKServer) Read(p []byte) (int, error) {
	for s.appBuf.Len() == 0 {
		ct, payload, err := s.readRecord()
		if err != nil {
			return 0, err
		}
		if ct == tlsPSKRecordApplicationData {
			s.appBuf.Write(payload)
		}
	}
	return s.appBuf.Read(p)
}

func (s *tlsPSKServer) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > tlsPSKMaxRecordPayload {
			chunk = chunk[:tlsPSKMaxRecordPayload]
		}
		if err := s.writeRecord(tlsPSKRecordApplicationData, chunk); err != nil {
			return written, err
		}
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}
