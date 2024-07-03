package remotepair

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"io"
	"net"
)

type pairConnection interface {
	recvResponse() (any, error)
	sendRequest(msg any) error
	canPair() bool
	tcpAddr() *net.TCPAddr
	bytes(b any) []byte
}

type wirePairConnection struct {
	addr    *net.TCPAddr
	xpcConn *xpc.RemoteXpcConnection
}

func (x *wirePairConnection) bytes(b any) []byte {
	return b.([]byte)
}

func (x *wirePairConnection) tcpAddr() *net.TCPAddr {
	return x.addr
}

func (x *wirePairConnection) recvResponse() (any, error) {
	rsp, err := x.xpcConn.Recv()
	if err != nil {
		return nil, err
	}
	return rsp.
	(map[string]any)[`value`], nil
}

func (x *wirePairConnection) sendRequest(msg any) error {
	return x.xpcConn.Send(map[string]any{
		`mangledTypeName`: `RemotePairing.ControlChannelMessageEnvelope`,
		`value`:           msg,
	})
}

func (x *wirePairConnection) canPair() bool {
	return true
}

const (
	magicRPPairing = `RPPairing`
)

type wifiPairConnection struct {
	addr    *net.TCPAddr
	netConn net.Conn
}

func (x *wifiPairConnection) bytes(b any) []byte {
	data, _ := base64.StdEncoding.DecodeString(b.(string))
	return data
}

func (x *wifiPairConnection) tcpAddr() *net.TCPAddr {
	return x.addr
}

func (x *wifiPairConnection) recvResponse() (any, error) {
	hdr := make([]byte, len(magicRPPairing)+2)
	_, err := io.ReadFull(x.netConn, hdr)
	if err != nil {return nil, err}
	num := binary.BigEndian.Uint16(hdr[len(magicRPPairing):])

	var msg any
	err = json.NewDecoder(io.LimitReader(x.netConn, int64(num))).Decode(&msg)
	return msg, err
}

func (x *wifiPairConnection) sendRequest(msg any) error {
	buf := &bytes.Buffer{}
	buf.WriteString(magicRPPairing)
	buf.WriteByte(0)
	buf.WriteByte(0)
	off := buf.Len()
	err := json.NewEncoder(buf).Encode(msg)
	if err != nil {return err}
	binary.BigEndian.PutUint16(buf.Bytes()[len(magicRPPairing):], uint16(buf.Len()-off))
	_, err = io.Copy(x.netConn, buf)
	return err
}

func (x *wifiPairConnection) canPair() bool {
	return false
}


