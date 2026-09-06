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

func (x *Service) List(dir string, recursive bool) ([]*FileStat, error) {
	x.gm.Lock()
	defer x.gm.Unlock()

	type command struct {
		code uint64
		name string
	}

	// Pipeline design: send goroutine runs ahead filling the wire;
	// recv loop (this goroutine) consumes responses in order and appends
	// new commands to the shared queue.
	//
	// Invariant: inflight = #sent - #received.
	// Done when inflight == 0 && queue is empty.

	var (
		mu       sync.Mutex
		cond     = sync.NewCond(&mu)
		queue    []command       // pending commands not yet sent
		inflight int             // commands sent but not yet received
		sendDone bool            // send goroutine has exited
		sendErr  error
	)

	push := func(cmd command) {
		mu.Lock()
		queue = append(queue, cmd)
		inflight++
		cond.Signal()
		mu.Unlock()
	}

	// seed
	push(command{code: opReadDir, name: dir})

	// sent is an ordered record of dispatched commands so recv can
	// process responses in the same order without any sn matching.
	sent := make(chan command, 256)

	// send goroutine: drains queue and writes requests onto the wire.
	go func() {
		defer func() {
			mu.Lock()
			sendDone = true
			cond.Broadcast()
			mu.Unlock()
			close(sent)
		}()
		for {
			mu.Lock()
			for len(queue) == 0 && !sendDone {
				// wait until recv adds more work or signals done
				cond.Wait()
			}
			if len(queue) == 0 {
				mu.Unlock()
				return
			}
			cmd := queue[0]
			queue = queue[1:]
			mu.Unlock()

			req := make([]byte, len(cmd.name)+1)
			copy(req, cmd.name)
			if _, err := x.send(cmd.code, &request{Args: req}); err != nil {
				mu.Lock()
				sendErr = err
				sendDone = true
				cond.Broadcast()
				mu.Unlock()
				return
			}
			sent <- cmd
		}
	}()

	var (
		out    []*FileStat
		retErr error
	)

	for cmd := range sent {
		var opcode uint64
		rsp, err := x.recv(&opcode, 0, false)

		mu.Lock()
		inflight--
		allDone := inflight == 0 && len(queue) == 0
		mu.Unlock()

		if err != nil {
			if cmd.code == opGetFileInfo {
				// stat failure (e.g. PermDenied) — skip this entry
				if allDone {
					// signal send goroutine to exit
					mu.Lock()
					sendDone = true
					cond.Broadcast()
					mu.Unlock()
				}
				continue
			}
			retErr = err
			break
		}

		if opcode == opData {
			raw := rsp.(*bytes.Buffer).Bytes()
			switch cmd.code {
			case opGetFileInfo:
				fst := &FileStat{Name: cmd.name}
				if err = x.parse(raw, fst); err != nil {
					break // malformed stat — skip
				}
				if fst.IsDir() && recursive {
					push(command{code: opReadDir, name: cmd.name})
				} else {
					out = append(out, fst)
				}
			case opReadDir:
				p := 0
				for i := range raw {
					if raw[i] == 0 {
						ent := string(raw[p:i])
						if ent != `.` && ent != `..` && ent != `` {
							push(command{
								code: opGetFileInfo,
								name: path.Join(cmd.name, ent),
							})
						}
						p = i + 1
					}
				}
			}
		}

		// When nothing is in-flight and the queue is empty, the send
		// goroutine is blocked waiting — wake it so it can exit.
		mu.Lock()
		allDone = inflight == 0 && len(queue) == 0
		if allDone {
			sendDone = true
			cond.Broadcast()
		}
		mu.Unlock()
		if allDone {
			break
		}
	}

	// drain sent so the send goroutine can unblock if it's stuck on send<-
	for range sent {}

	if retErr == nil {
		mu.Lock()
		retErr = sendErr
		mu.Unlock()
	}
	return out, retErr
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

