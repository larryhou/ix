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
	"sync"
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
	gm   sync.Mutex
}

type request struct {
	Args []byte
	Body int64
}

func (x *Service) send(op uint64, msg *request) (uint64, error) {
	rsv := make([]byte, 8)
	buf := &bytes.Buffer{}
	copy(rsv, magic)
	buf.Write(rsv) // magic

	length := headerSize + int64(len(msg.Args))
	x.bo.PutUint64(rsv, uint64(length + msg.Body))
	buf.Write(rsv) // packet length

	x.bo.PutUint64(rsv, uint64(length))
	buf.Write(rsv) // header length
	
	sn := x.sn
	x.bo.PutUint64(rsv, sn)
	buf.Write(rsv) // packet sn
	x.sn++

	x.bo.PutUint64(rsv, op)
	buf.Write(rsv)      // opcode
	buf.Write(msg.Args) // header options

	_, err := io.Copy(x.conn, buf)
	return sn, err
}

var (
	OutOfOrder = errors.New(`AFC RESPONSE OUT OF ORDER`)
)

func (x *Service) recv(op *uint64, sn uint64, noCopy bool) (r io.Reader, err error) {
	buf := make([]byte, headerSize)
	if _, err := io.ReadFull(x.conn, buf); err != nil {return nil, err}

	if m := string(buf[:8]); m != magic {
		return nil, errors.New(`invalid magic: ` + m)
	}

	length := x.bo.Uint64(buf[ 8:16]) // packet length
	opcode := x.bo.Uint64(buf[32:40]) // opcode
	if op != nil { *op = opcode }
	defer func() {
		if sn != 0 && sn != x.bo.Uint64(buf[24:32]) {
			err = OutOfOrder
		}
	}()

	num := length - headerSize
	switch opcode {
	case opStatus:
		out := make([]byte, num)
		_, err = io.ReadFull(x.conn, out)
		if err == nil {
			status := Retcode(x.bo.Uint64(out))
			if status != retSuccess {
				err = Error(status)
			}
		}

		return
	}

	r = io.LimitReader(x.conn, int64(num))
	if noCopy {
		return
	}

	out := &bytes.Buffer{}
	_, err = io.Copy(out, r)
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

func (x *Service) List(dir string, recursive bool) ([]*FileStat, error) {
	x.gm.Lock()
	defer x.gm.Unlock()

	type command struct {
		code uint64
		name string
		sn   uint64
	}

	ctx := &struct {
		*sync.Cond
		sync.RWMutex
		queue []*command
		p, c  int
	}{}

	ctx.Cond = sync.NewCond(ctx)
	ctx.queue = append(ctx.queue, &command{
		code: opReadDir,
		name: dir,
	})

	event := make(chan *command, 16)
	defer close(event)

	go func() {
		for {
			end := ctx.p >= len(ctx.queue)
			if end {
				ctx.Lock()
				ctx.Wait()
				ctx.Unlock()
				if ctx.p >= len(ctx.queue) {
					return
				}
			}

			cmd := ctx.queue[ctx.p]
			req := make([]byte, len(cmd.name)+1)
			copy(req, cmd.name)

			sn, err := x.send(cmd.code, &request{Args: req})
			if err != nil {continue}
			cmd.sn = sn

			ctx.p++
			event <- cmd
		}
	}()

	var out []*FileStat
	opcode := uint64(0)
	for cmd := range event {
		rsp, err := x.recv(&opcode, cmd.sn, false)
		ctx.c++

		if err == nil && opcode == opData {
			raw := rsp.(*bytes.Buffer).Bytes()
			switch cmd.code {
			case opGetFileInfo:
				fst := &FileStat{Name: cmd.name}
				err = x.parse(raw, fst)
				if err == nil {
					if fst.IsDir() && recursive {
						ctx.queue = append(ctx.queue, &command{
							code: opReadDir,
							name: cmd.name,
						})
						ctx.Signal()
					} else {
						out = append(out, fst)
					}
				}
			case opReadDir:
				p := 0
				for i := range raw {
					if raw[i] == 0 {
						ent := string(raw[p:i])
						switch ent {
						case `.`, `..`:
						default:
							ctx.queue = append(ctx.queue, &command{
								code: opGetFileInfo,
								name: path.Join(cmd.name, string(raw[p:i])),
							})
							ctx.Signal()
						}

						p = i + 1
					}
				}
			}
		}

		if ctx.c == len(ctx.queue) {
			ctx.Signal() // TERMINATE SEND LOOP
			break
		}
	}

	return out, nil
}

func (x *Service) Exists(name string) bool {
	_, err := x.Stat(name)
	return err == nil
}

func (x *Service) Stat(name string) (*FileStat, error) {
	req := make([]byte, len(name)+1)
	copy(req, name)

	msg := &FileStat{Name: name}
	return msg, x.get(opGetFileInfo, req, msg)
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
	x.gm.Lock()
	defer x.gm.Unlock()
	var msg *request
	switch data := req.(type) {
	case []byte: msg = &request{Args: data, Body: 0}
	case *request: msg = data
	default:
		return errors.New(`BAD REQUEST`)
	}

	sn, err := x.send(op, msg)
	if err != nil {
		return err
	}

	noCopy := false
	switch rsp.(type) {
	case *io.Reader: noCopy = true
	}

	var opcode uint64
	r, err := x.recv(&opcode, sn, noCopy)
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

