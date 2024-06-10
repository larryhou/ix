package afc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"log"
	"reflect"
)

func New(service *usbmux.Service) *Service {
	s := &Service{Service: service}
	s.ByteOrder = binary.LittleEndian
	return s
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

func (x *Service) Stat(name string) (*FileStat, error) {
	req := make([]byte, len(name)+1)
	copy(req, name)

	rsp := &FileStat{}
	return rsp, x.Get(OpGetFileInfo, req, rsp)
}

func (x *Service) Get(op uint64, req []byte, rsp any) error {
	if err := x.Send(op, req); err != nil {
		return err
	}

	var opcode uint64
	raw, err := x.Recv(&opcode)
	if err == nil {
		switch opcode {
		case OpData:
			return x.parse(raw, rsp)
		}
	}
	
	return err
}

func (x *Service) parse(b []byte, rsp any) error {
	log.Printf(`%s`, string(b))
	rv := reflect.ValueOf(rsp).Elem()
	rt := rv.Type()

	m := make(map[string]int)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		m[f.Name] = i
		if tag, ok := f.Tag.Lookup(`json`); ok {
			m[tag] = i
		}
		log.Printf("%s %s\n", f.Name, f.Type.Name())
	}

	p := 0
	out := make(map[string][]byte)
	var k *string
	for i := range b {
		if b[i] == '\x00' {
			if k == nil {
				s := string(b[p:i])
				k = &s
			} else {
				out[*k] = b[p:i]
				if idx, ok := m[*k]; ok {
					err := json.Unmarshal(b[p:i], rv.Field(idx).Addr().Interface())
					if err != nil {return err}
				}
				k = nil
			}

			p = i + 1
		}
	}

	return nil
}

