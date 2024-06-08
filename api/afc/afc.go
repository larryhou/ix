package afc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
)

func New(service *usbmux.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*usbmux.Service

	idx uint64
}

func (x *Service) Send(op uint64, msg []byte) error {
	rsv := make([]byte, 8)
	buf := &bytes.Buffer{}
	copy(rsv, Magic)
	buf.Write(rsv) // magic

	length := HeaderSize + len(msg)
	x.PutUint64(rsv, uint64(length))
	buf.Write(rsv) // entire length
	buf.Write(rsv) // packet length

	x.PutUint64(rsv, x.idx)
	buf.Write(rsv) // packet num
	x.idx++

	x.PutUint64(rsv, op)
	buf.Write(rsv) // opcode
	buf.Write(msg) // data
	_, err := x.Write(buf.Bytes())
	return err
}

func (x *Service) Recv(op *uint64) ([]byte, error) {
	buf := make([]byte, HeaderSize)
	if _, err := x.Read(buf); err != nil {return nil, err}

	if m := string(buf[:8]); m != Magic {
		return nil, errors.New(`invalid magic: ` + m)
	}

	length := x.Uint64(buf[ 8:16]) // entire length
	opcode := x.Uint64(buf[32:40]) // opcode
	if op != nil { *op = opcode }

	data := make([]byte, length - HeaderSize)
	_, err := x.Read(data)
	if err == nil {
		switch opcode {
		case OpStatus:
			if len(data) == 8 {
				status := x.Uint64(data)
				if status != RetSuccess {
					err = fmt.Errorf(`ERROR/%d`, status)
				}
			}
		}
	}

	return data, err
}