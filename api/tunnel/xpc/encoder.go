package xpc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"io"
	"math"
	"time"
)

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, b: binary.LittleEndian}
}

type Encoder struct {
	w   io.Writer
	b   binary.ByteOrder
	buf [8]byte // scratch buffer — avoids per-call heap allocation
}

func (x *Encoder) Encode(v any) error {
	return x.object(v)
}

func (x *Encoder) boolean(v bool) error {
	var b uint32
	if v {
		b = 1
	}
	return x.u32(b)
}

func (x *Encoder) put(v []byte) error {
	n := len(v)
	for t := 0; t < n; {
		k, err := x.w.Write(v[t:])
		if err != nil {
			return err
		}
		t += k
	}
	return nil
}

func (x *Encoder) s32(v int32) error {
	return x.u32(uint32(v))
}

func (x *Encoder) u32(v uint32) error {
	x.b.PutUint32(x.buf[:4], v)
	return x.put(x.buf[:4])
}

func (x *Encoder) s64(v int64) error {
	return x.u64(uint64(v))
}

func (x *Encoder) u64(v uint64) error {
	x.b.PutUint64(x.buf[:8], v)
	return x.put(x.buf[:8])
}

func (x *Encoder) double(v float64) error {
	x.b.PutUint64(x.buf[:8], math.Float64bits(v))
	return x.put(x.buf[:8])
}

func (x *Encoder) data(v []byte) error {
	num := len(v)
	err := x.u32(uint32(num))
	if err == nil {
		err = x.put(v)
	}
	if err == nil {
		err = x.align(num)
	}
	return err
}

func (x *Encoder) string(v string) error {
	num := len(v) + 1
	err := x.u32(uint32(num))
	if err == nil {
		err = x.cstring(v)
	}
	return err
}

func (x *Encoder) align(n int) error {
	pad := ((n + 3) & ^3) - n
	if pad == 0 {
		return nil
	}
	var zeros [4]byte
	return x.put(zeros[:pad])
}

func (x *Encoder) cstring(v string) error {
	num := len(v) + 1
	buf := make([]byte, num)
	copy(buf, v)
	err := x.put(buf)
	if err == nil {
		err = x.align(num)
	}
	return err
}

func (x *Encoder) uuid(v uuid.UUID) error {
	return x.put(v[:])
}

func (x *Encoder) fd(v Fd) error {
	return x.u32(uint32(v))
}

func (x *Encoder) shmem(v Shmem) error {
	return x.u64(uint64(v))
}

func (x *Encoder) fileTransfer(v FileTransfer) error {
	err := x.s64(v.MsgId)
	if err == nil {
		err = x.object(v.File)
	}
	return err
}

func (x *Encoder) array(v []any) error {
	err := x.u32(uint32(len(v)))
	if err == nil {
		for _, item := range v {
			err = x.object(item)
			if err != nil {
				return err
			}
		}
	}
	return err // was `return nil` — bug: swallowed u32 write error
}

func (x *Encoder) dictionary(v map[string]any) error {
	err := x.u32(uint32(len(v)))
	if err == nil {
		for k, val := range v {
			err = x.cstring(k)
			if err == nil {
				err = x.object(val)
			}
			if err != nil {
				return err
			}
		}
	}
	return err // was `return nil` — bug: swallowed u32 write error
}

func (x *Encoder) object(v any) (err error) {
	switch t := v.(type) {
	case nil:
		err = x.u32(TypeNull)

	case int:
		if err = x.u32(TypeInt64); err == nil {
			err = x.s64(int64(t))
		}

	case int64:
		if err = x.u32(TypeInt64); err == nil {
			err = x.s64(t)
		}

	case uint64:
		if err = x.u32(TypeUint64); err == nil {
			err = x.u64(t)
		}

	case float64:
		if err = x.u32(TypeDouble); err == nil {
			err = x.double(t)
		}

	case bool:
		if err = x.u32(TypeBool); err == nil {
			err = x.boolean(t)
		}

	case string:
		if err = x.u32(TypeString); err == nil {
			err = x.string(t)
		}

	case Fd:
		if err = x.u32(TypeFd); err == nil {
			err = x.fd(t)
		}

	case Shmem:
		if err = x.u32(TypeShmem); err == nil {
			err = x.shmem(t)
		}

	case uuid.UUID:
		if err = x.u32(TypeUuid); err == nil {
			err = x.uuid(t)
		}

	case FileTransfer:
		if err = x.u32(TypeFileTransfer); err == nil {
			err = x.fileTransfer(t)
		}

	case map[string]any:
		if err = x.u32(TypeDictionary); err == nil {
			buf := &bytes.Buffer{}
			sub := &Encoder{w: buf, b: x.b}
			sub.u32(0) // size placeholder — back-patched below
			if err = sub.dictionary(t); err == nil {
				x.b.PutUint32(buf.Bytes(), uint32(buf.Len()-4))
				err = x.put(buf.Bytes())
			}
		}

	case []any:
		if err = x.u32(TypeArray); err == nil {
			buf := &bytes.Buffer{}
			sub := &Encoder{w: buf, b: x.b}
			sub.u32(0) // size placeholder — back-patched below
			if err = sub.array(t); err == nil {
				x.b.PutUint32(buf.Bytes(), uint32(buf.Len()-4))
				err = x.put(buf.Bytes())
			}
		}

	case []byte:
		if err = x.u32(TypeData); err == nil {
			err = x.data(t)
		}

	case time.Time:
		// Decoder interprets wire value as nanoseconds; write UnixNano.
		if err = x.u32(TypeDate); err == nil {
			err = x.s64(t.UnixNano())
		}

	default:
		err = fmt.Errorf(`unsupported type: %T %+v`, t, t)
	}

	return
}
