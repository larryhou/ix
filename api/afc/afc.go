package afc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"path"
	"reflect"
)

func New(conn net.Conn) *Service {
	s := &Service{conn: conn}
	s.bo = binary.LittleEndian
	return s
}

type Service struct {
	conn net.Conn
	bo   binary.ByteOrder
	sn   uint64
}

type request struct {
	Args []byte
	Body int64
}

func (x *Service) send(op uint64, msg *request) error {
	rsv := make([]byte, 8)
	buf := &bytes.Buffer{}
	copy(rsv, magic)
	buf.Write(rsv) // magic

	length := headerSize + int64(len(msg.Args))
	x.bo.PutUint64(rsv, uint64(length + msg.Body))
	buf.Write(rsv) // packet length

	x.bo.PutUint64(rsv, uint64(length))
	buf.Write(rsv) // header length

	x.bo.PutUint64(rsv, x.sn)
	buf.Write(rsv) // packet num
	x.sn++

	x.bo.PutUint64(rsv, op)
	buf.Write(rsv)      // opcode
	buf.Write(msg.Args) // header options

	_, err := io.Copy(x.conn, buf)
	return err
}

func (x *Service) recv(op *uint64, noCopy bool) (io.Reader, error) {
	buf := make([]byte, headerSize)
	if _, err := io.ReadFull(x.conn, buf); err != nil {return nil, err}

	if m := string(buf[:8]); m != magic {
		return nil, errors.New(`invalid magic: ` + m)
	}

	length := x.bo.Uint64(buf[ 8:16]) // packet length
	opcode := x.bo.Uint64(buf[32:40]) // opcode
	if op != nil { *op = opcode }

	num := length - headerSize
	switch opcode {
	case opStatus:
		out := make([]byte, num)
		_, err := io.ReadFull(x.conn, out)
		if err == nil {
			status := Retcode(x.bo.Uint64(out))
			if status != retSuccess {
				err = Error(status)
			}
		}

		return nil, err
	}

	r := io.LimitReader(x.conn, int64(num))
	if noCopy {
		return r, nil
	}

	out := &bytes.Buffer{}
	_, err := io.Copy(out, r)
	return out, err
}

func (x *Service) Remove(name string) error {
	req := make([]byte, len(name)+1)
	copy(req, name)
	return x.get(opRemovePath, req, nil)
}

func (x *Service) Rename(name string, new string) error {
	req := make([]byte, len(name)+1+len(new)+1)
	copy(req, name)
	copy(req[len(name)+1:], new)
	return x.get(opRenamePath, req, nil)
}

func (x *Service) MkDir(name string) error {
	req := make([]byte, len(name)+1)
	copy(req, name)
	return x.get(opMakeDir, req, nil)
}

func (x *Service) FindAll(dir string) ([]*FileStat, error) {
	var data []*FileStat

	pending := []string{dir}
	for len(pending) > 0 {
		dir := pending[0]
		pending = pending[1:]
		entries, err := x.List(dir)
		if err != nil {return nil, err}
		for _, ent := range entries {
			name := path.Join(dir, ent)
			info, err := x.Stat(name)
			if err != nil {return nil, err}
			if info.IsDir() {
				pending = append(pending, name)
			} else {
				info.Name = name
				data = append(data, info)
			}
		}
	}

	return data, nil
}

func (x *Service) Exists(name string) bool {
	_, err := x.Stat(name)
	return err == nil
}

func (x *Service) Stat(name string) (*FileStat, error) {
	req := make([]byte, len(name)+1)
	copy(req, name)

	msg := &FileStat{}
	return msg, x.get(opGetFileInfo, req, msg)
}

func (x *Service) List(name string) ([]string, error) {
	req := make([]byte, len(name)+1)
	copy(req, name)
	var raw []byte
	err := x.get(opReadDir, req, &raw)
	var out []string
	if p := 0; err == nil {
		for i := range raw {
			if raw[i] == 0 {
				ent := string(raw[p:i])
				switch ent {
				case `.`, `..`:
				default:
					out = append(out, string(raw[p:i]))
				}

				p = i + 1
			}
		}
	}

	return out, err
}

func (x *Service) Open(name string, mode string) (*FileHandle, error) {
	perm := uint64(0)
	switch mode {
	case `r` : perm = permRDONLY
	case `r+`: perm = permRW
	case `w` : perm = permWRONLY
	case `w+`: perm = permWR
	case `a` : perm = permAPPEND
	case `a+`: perm = permRDAPPEND
	default:
		return nil, errors.New(`BAD MODE: ` + mode)
	}

	req := make([]byte, len(name) + 8 + 1)
	x.bo.PutUint64(req, perm)
	copy(req[8:], name)

	var h []byte
	if err := x.get(opFileOpen, req, &h); err == nil {
		return &FileHandle{
			name: name,
			fd:   x.bo.Uint64(h),
			sv:   x,
		}, nil
	} else {
		return nil, err
	}
}

func (x *Service) get(op uint64, req any, rsp any) error {
	var msg *request
	switch data := req.(type) {
	case []byte: msg = &request{Args: data, Body: 0}
	case *request: msg = data
	default:
		return errors.New(`BAD REQUEST`)
	}

	if err := x.send(op, msg); err != nil {
		return err
	}

	noCopy := false
	switch rsp.(type) {
	case *io.Reader: noCopy = true
	}

	var opcode uint64
	r, err := x.recv(&opcode, noCopy)
	if err == nil && rsp != nil {
		switch out := rsp.(type) {
		case *io.Reader:
			*out = r
		case *[]byte:
			*out = r.(*bytes.Buffer).Bytes()
		default:
			switch opcode {
			case opData:
				err = x.parse(r.(*bytes.Buffer).Bytes(), rsp)
			default:
				log.Printf(`OPCODE %d`, opcode)
			}
		}
	}
	
	return err
}

func (x *Service) parse(b []byte, rsp any) (err error) {
	rv := reflect.ValueOf(rsp).Elem()
	rt := rv.Type()

	m := make(map[string]int)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		m[f.Name] = i
		if tag, ok := f.Tag.Lookup(`json`); ok {
			m[tag] = i
		}
	}

	p := 0
	k := ``
	for i := range b {
		if b[i] == 0 {
			if len(k) == 0 {
				k = string(b[p:i])
			} else {
				if idx, ok := m[k]; ok {
					f := rv.Field(idx).Addr().Interface()
					switch v := f.(type) {
					case json.Unmarshaler:
						err = v.UnmarshalJSON(b[p:i])
					default:
						err = json.Unmarshal(b[p:i], v)
					}

					if err != nil {return err}
				}

				k = ``
			}

			p = i + 1
		}
	}

	return
}

