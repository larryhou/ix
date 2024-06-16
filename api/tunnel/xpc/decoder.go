package xpc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"io"
	"time"
	"unsafe"
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r, b: binary.LittleEndian}
}

type Decoder struct {
	r io.Reader
	b binary.ByteOrder
}

func (x *Decoder) Decode(v any) error {
	out, err := x.object()
	if err == nil {
		switch data := v.(type) {
		case *any: *data = out
		}
	}

	return err
}

func (x *Decoder) boolean() (bool, error) {
	v, err := x.u32()
	return v != 0, err
}

func (x *Decoder) get(v []byte) error {
	n := len(v)
	for t := 0; t < n; {
		k, err := x.r.Read(v[t:])
		if err != nil {return err}
		t += k
	}
	return nil
}

func (x *Decoder) s32() (int, error) {
	v, err := x.u32()
	if err == nil {
		return int(*(*int32)(unsafe.Pointer(&v))), nil
	}
	return 0, err
}

func (x *Decoder) u32() (int, error) {
	buf := make([]byte, 4)
	err := x.get(buf)
	if err == nil {
		return int(x.b.Uint32(buf)), nil
	}

	return 0, err
}

func (x *Decoder) s64() (int, error) {
	v, err := x.u64()
	if err == nil {
		return int(*(*int64)(unsafe.Pointer(&v))), nil
	}
	return 0, err
}

func (x *Decoder) u64() (int, error) {
	buf := make([]byte, 8)
	err := x.get(buf)
	if err == nil {
		return int(x.b.Uint64(buf)), nil
	}

	return 0, err
}

func (x *Decoder) double() (float64, error) {
	buf := make([]byte, 8)
	err := x.get(buf)
	if err == nil {
		v := x.b.Uint64(buf)
		return *(*float64)(unsafe.Pointer(&v)), nil
	}

	return 0, err
}

func (x *Decoder) data() ([]byte, error) {
	var buf []byte
	num, err := x.u32()
	if err == nil {
		buf = make([]byte, num)
		err = x.get(buf)
	}

	if err == nil {
		err = x.align(num)
	}

	return buf, err
}

func (x *Decoder) string() (string, error) {
	var buf []byte
	num, err := x.u32()
	if err == nil {
		buf = make([]byte, num)
		err = x.get(buf)
	}

	if err == nil {
		err = x.align(num)
	}

	if len(buf) > 0 {
		return string(buf[:len(buf)-1]), nil
	}

	return ``, err
}

func (x *Decoder) align(n int) error {
	p := make([]byte, 4)
	return x.get(p[:((n + 3) & ^3)-n])
}

func (x *Decoder) cstring() (string, error) {
	sip := make([]byte, 4)
	buf := &bytes.Buffer{}
	for {
		err := x.get(sip)
		if err != nil {
			return ``, err
		}

		buf.Write(sip)
		if sip[3] == 0 {
			break
		}
	}

	raw := buf.Bytes()[:buf.Len()-1]
	for k := 0; k < 4; k++ {
		if raw[len(raw)-1] != 0 { break }
		raw = raw[:len(raw)-1]
	}

	return string(raw), nil
}

func (x *Decoder) uuid() (uuid.UUID, error) {
	u := uuid.UUID{}
	return u, x.get(u[:])
}

func (x *Decoder) fd() (Fd, error) {
	v, err := x.u32()
	return Fd(v), err
}

func (x *Decoder) shmem() (Shmem, error) {
	v, err := x.u64()
	return Shmem(v), err
}

func (x *Decoder) fileTransfer() (ft FileTransfer, err error) {
	id, err := x.u64()
	if err != nil {return}
	ft.MsgId = id

	obj, err := x.object()
	if err != nil {return}

	ft.Data = obj
	return
}

func (x *Decoder) array() ([]any, error) {
	var out []any
	num, err := x.u32()
	if err != nil {return nil, err}
	for i := 0; i < num; i++ {
		v, err := x.object()
		if err != nil {return nil, err}
		out = append(out, v)
	}

	return out, nil
}

func (x *Decoder) dictionary() (map[string]any, error) {
	out := make(map[string]any)
	num, err := x.u32()
	if err != nil {return nil, err}
	for i := 0; i < num; i++ {
		k, err := x.cstring()
		if err != nil {return nil, err}

		v, err := x.object()
		if err != nil {return nil, err}

		out[k] = v
	}

	return out, nil
}

func (x *Decoder) object() (any, error) {
	t, err := x.u32()
	if err != nil {return nil, err}

	num := make([]byte, 4)

	switch t {
	case TypeNull:
	case TypeInt64:
		return x.s64()

	case TypeUint64:
		return x.u64()

	case TypeDouble:
		return x.double()

	case TypeBool:
		return x.boolean()

	case TypeString:
		return x.string()

	case TypeFd:
		return x.fd()

	case TypeShmem:
		return x.shmem()

	case TypeUuid:
		return x.uuid()

	case TypeFileTransfer:
		return x.fileTransfer()

	case TypeDictionary:
		err = x.get(num)
		if err == nil {
			buf := make([]byte, x.b.Uint32(num))
			if err = x.get(buf); err == nil {
				sub := &Decoder{r: bytes.NewReader(buf), b: x.b}
				return sub.dictionary()
			}
		}

	case TypeArray:
		err = x.get(num)
		if err == nil {
			buf := make([]byte, x.b.Uint32(num))
			if err = x.get(buf); err == nil {
				sub := &Decoder{r: bytes.NewReader(buf), b: x.b}
				return sub.array()
			}
		}

	case TypeData:
		return x.data()

	case TypeDate:
		v, err := x.s64()
		if v := int64(v); err == nil {
			return time.Unix(v/int64(time.Second), v % int64(time.Second)), nil
		}
		return nil, err
		
	default:
		err = fmt.Errorf(`supported type: %+v`, t)
	}

	return nil, err
}

