package afc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
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
		return nil, fmt.Errorf(`invalid magic: %s`, m)
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

// Walk traverses dir up to maxDepth levels deep (0 = unlimited) using a
// pipelined send/recv design. fn is called for every entry (both files and
// directories) as it arrives; fn may be nil. Walk returns as soon as all
// responses have been processed.
func (x *Service) Walk(dir string, maxDepth int, fn func(*FileStat)) error {
	x.gm.Lock()
	defer x.gm.Unlock()

	type command struct {
		code  uint64
		name  string
		sn    uint64
		depth int // directory depth of this command
	}

	var (
		mu       sync.Mutex
		cond     = sync.NewCond(&mu)
		workQ    []*command
		inflight int
		done     bool
	)

	sent := make(chan *command, 512)

	addWork := func(cmds ...*command) {
		workQ = append(workQ, cmds...)
		inflight += len(cmds)
		cond.Signal()
	}

	mu.Lock()
	addWork(&command{code: opReadDir, name: dir, depth: 0})
	mu.Unlock()

	sendErrCh := make(chan error, 1)

	go func() {
		defer close(sent)
		for {
			mu.Lock()
			for len(workQ) == 0 && !done {
				cond.Wait()
			}
			if done && len(workQ) == 0 {
				mu.Unlock()
				return
			}
			cmd := workQ[0]
			workQ = workQ[1:]
			mu.Unlock()

			req := make([]byte, len(cmd.name)+1)
			copy(req, cmd.name)
			sn, err := x.send(cmd.code, &request{Args: req})
			if err != nil {
				sendErrCh <- err
				return
			}
			cmd.sn = sn
			sent <- cmd
		}
	}()

	var retErr error
	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		for cmd := range sent {
			var opcode uint64
			rsp, err := x.recv(&opcode, cmd.sn, false)

			mu.Lock()
			inflight--
			if err != nil {
				if cmd.code != opGetFileInfo {
					done = true
					cond.Signal()
					mu.Unlock()
					retErr = err
					return
				}
			} else if opcode == opData {
				raw := rsp.(*bytes.Buffer).Bytes()
				switch cmd.code {
				case opGetFileInfo:
					fst := &FileStat{Name: cmd.name}
					if e := x.parse(raw, fst); e == nil {
						recurse := fst.IsDir() && (maxDepth == 0 || cmd.depth < maxDepth)
						if recurse {
							addWork(&command{code: opReadDir, name: cmd.name, depth: cmd.depth})
						}
						if fn != nil {
							mu.Unlock()
							fn(fst)
							mu.Lock()
						}
					}
				case opReadDir:
					var newCmds []*command
					p := 0
					for i := range raw {
						if raw[i] == 0 {
							ent := string(raw[p:i])
							if ent != `.` && ent != `..` && ent != `` {
								newCmds = append(newCmds, &command{
									code:  opGetFileInfo,
									name:  path.Join(cmd.name, ent),
									depth: cmd.depth + 1,
								})
							}
							p = i + 1
						}
					}
					if len(newCmds) > 0 {
						addWork(newCmds...)
					}
				}
			}
			if inflight == 0 && len(workQ) == 0 {
				done = true
				cond.Signal()
			}
			mu.Unlock()
		}
	}()

	<-recvDone

	if retErr == nil {
		select {
		case err := <-sendErrCh:
			retErr = err
		default:
		}
	}
	return retErr
}

// List collects all results from Walk into a slice.
// maxDepth=0 means unlimited recursion; maxDepth=1 lists only the immediate
// children (non-recursive); higher values limit directory depth.
// List collects all files (not directories) into a slice.
func (x *Service) List(dir string, maxDepth int) ([]*FileStat, error) {
	var out []*FileStat
	err := x.Walk(dir, maxDepth, func(fst *FileStat) {
		if !fst.IsDir() {
			out = append(out, fst)
		}
	})
	return out, err
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
		return nil, fmt.Errorf(`BAD MODE: %s`, mode)
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

