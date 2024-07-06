package usbmux

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/larryhou/j3idevice/api/j3"
	"howett.net/plist"
	"io"
	"net"
)

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		ByteOrder: binary.LittleEndian,
		Connection: &j3.Connection{
			Conn: conn,
		},
	}
}

type Connection struct {
	*j3.Connection
	binary.ByteOrder
	
	sn uint32
}

func (x *Connection) nextSeq() uint32 {
	x.sn++
	return x.sn
}

func (x *Connection) Send(msg any) (uint32, error) {
	switch data := msg.(type) {
	case *j3.ConnectRequest:
		data.KLibUSBMuxVersion = j3.MuxVersion
		data.ClientVersionString = j3.VersionName
		data.ProgName = j3.ProgramName
		data.MessageType = j3.TypeConnect
	}

	rsv := make([]byte, 4)
	buf := &bytes.Buffer{}
	buf.Write(rsv)

	x.PutUint32(rsv, VerPlist)
	buf.Write(rsv)

	x.PutUint32(rsv, MsgPlist)
	buf.Write(rsv)

	seq := x.nextSeq()
	x.PutUint32(rsv, seq)
	buf.Write(rsv)

	err := plist.NewEncoder(buf).Encode(msg)
	if err != nil {return seq, err}

	x.PutUint32(rsv, uint32(buf.Len()))
	copy(buf.Bytes(), rsv)

	_, err = io.Copy(x.Conn, buf)
	return seq, err
}

func (x *Connection) Recv(msg any, seq uint32) error {
	rsv := make([]byte, 4)
	if _, err := io.ReadFull(x.Conn, rsv); err != nil {return err}

	num := x.Uint32(rsv)
	buf := make([]byte, num - 4)
	if _, err := io.ReadFull(x.Conn, buf); err != nil {return err}

	if tag := x.Uint32(buf[8:12]); seq > 0 && tag != seq {
		return fmt.Errorf(`seq echo mismatch: %d != %d`, tag, seq)
	}

	err := plist.NewDecoder(bytes.NewReader(buf[12:])).Decode(msg)
	if err == nil {
		if r, ok := msg.(Retcode); ok {
			err = r.Verify()
		}
	}

	return err
}

func (x *Connection) Get(req, rsp any) error {
	if seq, err := x.Send(req); err == nil {
		return x.Recv(rsp, seq)
	} else {
		return err
	}
}

type Retcode interface {
	Verify() error
}
