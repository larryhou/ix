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

type Encoder struct {
	Writer io.Writer
	binary.ByteOrder
}

func (x *Encoder) Encode(v any) error {
	return x.object(v)
}

func (x *Encoder) boolean(v bool) error {
	return x.u32(uint32(*(*byte)(unsafe.Pointer(&v))))
}

func (x *Encoder) put(v []byte) error {
	n := len(v)
	for t := 0; t < n; {
		k, err := x.Writer.Write(v[t:])
		if err != nil {return err}
		t += k
	}
	return nil
}

func (x *Encoder) s32(v int32) error {
	return x.u32(*(*uint32)(unsafe.Pointer(&v)))
}

func (x *Encoder) u32(v uint32) error {
	buf := make([]byte, 4)
	x.ByteOrder.PutUint32(buf, v)
	return x.put(buf)
}

func (x *Encoder) s64(v int64) error {
	return x.u64(*(*uint64)(unsafe.Pointer(&v)))
}

func (x *Encoder) u64(v uint64) error {
	buf := make([]byte, 8)
	x.ByteOrder.PutUint64(buf, v)
	return x.put(buf)
}

func (x *Encoder) double(v float64) error {
	buf := make([]byte, 8)
	x.ByteOrder.PutUint64(buf, *(*uint64)(unsafe.Pointer(&v)))
	return x.put(buf)
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
	p := make([]byte, 4)
	return x.put(p[:((n + 3) & ^3)-n])
}

func (x *Encoder) cstring(v string) error {
	num := len(v)+1
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
	err := x.u64(uint64(v.MsgId))
	if err == nil {
		err = x.object(v.Data)
	}

	return err
}

func (x *Encoder) array(v []any) error {
	err := x.u32(uint32(len(v)))
	if err == nil {
		for _, item := range v {
			err = x.object(item)
			if err != nil {return err}
		}
	}

	return nil
}

func (x *Encoder) dictionary(v map[string]any) error {
	err := x.u32(uint32(len(v)))
	if err == nil {
		for k, v := range v {
			err = x.cstring(k)
			if err == nil {
				err = x.object(v)
			}
			if err != nil {return err}
		}
	}

	return nil
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
			rsv := make([]byte, 4)
			buf := &bytes.Buffer{}
			buf.Write(rsv)
			sub := &Encoder{Writer: buf, ByteOrder: x.ByteOrder}
			if err = sub.dictionary(t); err == nil {
				x.ByteOrder.PutUint32(rsv, uint32(buf.Len()-4))
				copy(buf.Bytes(), rsv)
				err = x.put(buf.Bytes())
			}
		}
	case []any:
		if err = x.u32(TypeArray); err == nil {
			rsv := make([]byte, 4)
			buf := &bytes.Buffer{}
			buf.Write(rsv)
			sub := &Encoder{Writer: buf, ByteOrder: x.ByteOrder}
			if err = sub.array(t); err == nil {
				x.ByteOrder.PutUint32(rsv, uint32(buf.Len()-4))
				copy(buf.Bytes(), rsv)
				err = x.put(buf.Bytes())
			}
		}
	case []byte:
		if err = x.u32(TypeData); err == nil {
			err = x.data(t)
		}
	case time.Time:
		if err = x.u32(TypeDate); err == nil {
			err = x.s64(t.Unix())
		}
	default:
		err = fmt.Errorf(`supported type: %+v`, t)
	}

	return
}

