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

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r, b: binary.LittleEndian}
}

type Decoder struct {
	r   io.Reader
	b   binary.ByteOrder
	buf [8]byte // scratch buffer — avoids per-call heap allocation
}

func (x *Decoder) Decode(v any) error {
	out, err := x.object()
	if err == nil {
		switch data := v.(type) {
		case *any:
			*data = out
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
		if err != nil {
			return err
		}
		t += k
	}
	return nil
}

func (x *Decoder) s32() (int32, error) {
	v, err := x.u32()
	return int32(v), err
}

func (x *Decoder) u32() (uint32, error) {
	err := x.get(x.buf[:4])
	if err != nil {
		return 0, err
	}
	return x.b.Uint32(x.buf[:4]), nil
}

func (x *Decoder) s64() (int64, error) {
	v, err := x.u64()
	return int64(v), err
}

func (x *Decoder) u64() (uint64, error) {
	err := x.get(x.buf[:8])
	if err != nil {
		return 0, err
	}
	return x.b.Uint64(x.buf[:8]), nil
}

func (x *Decoder) double() (float64, error) {
	v, err := x.u64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(v), nil
}

func (x *Decoder) data() ([]byte, error) {
	num, err := x.u32()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, num)
	if err = x.get(buf); err != nil {
		return nil, err
	}
	if err = x.align(int(num)); err != nil {
		return nil, err
	}
	return buf, nil
}

func (x *Decoder) string() (string, error) {
	num, err := x.u32()
	if err != nil {
		return "", err
	}
	if num == 0 {
		return "", nil
	}
	buf := make([]byte, num)
	if err = x.get(buf); err != nil {
		return "", err
	}
	if err = x.align(int(num)); err != nil {
		return "", err
	}
	// Strip trailing NUL included in length.
	return string(buf[:num-1]), nil
}

func (x *Decoder) align(n int) error {
	pad := ((n + 3) & ^3) - n
	if pad == 0 {
		return nil
	}
	var tmp [4]byte
	return x.get(tmp[:pad])
}

func (x *Decoder) cstring() (string, error) {
	// Keys are 4-byte aligned NUL-padded C strings; read 4 bytes at a time
	// until the last byte in a chunk is NUL (marks end of aligned block).
	var buf []byte
	chunk := [4]byte{}
	for {
		if err := x.get(chunk[:]); err != nil {
			return "", err
		}
		buf = append(buf, chunk[:]...)
		if chunk[3] == 0 {
			break
		}
	}
	// Trim trailing NUL padding.
	end := len(buf)
	for end > 0 && buf[end-1] == 0 {
		end--
	}
	return string(buf[:end]), nil
}

func (x *Decoder) uuid() (uuid.UUID, error) {
	var u uuid.UUID
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
	id, err := x.s64()
	if err != nil {
		return
	}
	ft.MsgId = id
	ft.File, err = x.object()
	return
}

func (x *Decoder) array() ([]any, error) {
	num, err := x.u32()
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, num)
	for i := uint32(0); i < num; i++ {
		v, err := x.object()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (x *Decoder) dictionary() (map[string]any, error) {
	num, err := x.u32()
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, num)
	for i := uint32(0); i < num; i++ {
		k, err := x.cstring()
		if err != nil {
			return nil, err
		}
		v, err := x.object()
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

// maxPayloadSize caps the per-object allocation to guard against malformed packets.
const maxPayloadSize = 512 << 20 // 512 MiB

func (x *Decoder) object() (any, error) {
	t, err := x.u32()
	if err != nil {
		return nil, err
	}

	switch t {
	case TypeNull:
		return nil, nil

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
		sz, err := x.u32()
		if err != nil {
			return nil, err
		}
		if uint64(sz) > maxPayloadSize {
			return nil, fmt.Errorf("xpc: dictionary payload too large: %d bytes", sz)
		}
		buf := make([]byte, sz)
		if err = x.get(buf); err != nil {
			return nil, err
		}
		sub := &Decoder{r: bytes.NewReader(buf), b: x.b}
		return sub.dictionary()

	case TypeArray:
		sz, err := x.u32()
		if err != nil {
			return nil, err
		}
		if uint64(sz) > maxPayloadSize {
			return nil, fmt.Errorf("xpc: array payload too large: %d bytes", sz)
		}
		buf := make([]byte, sz)
		if err = x.get(buf); err != nil {
			return nil, err
		}
		sub := &Decoder{r: bytes.NewReader(buf), b: x.b}
		return sub.array()

	case TypeData:
		return x.data()

	case TypeDate:
		ns, err := x.s64()
		if err != nil {
			return nil, err
		}
		// Wire value is nanoseconds since Unix epoch.
		return time.Unix(ns/int64(time.Second), ns%int64(time.Second)), nil

	default:
		return nil, fmt.Errorf("xpc: unsupported type: 0x%08x", t)
	}
}
