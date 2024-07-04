package remotesvr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/larryhou/iconsole/ns"
)

const (
	magicAux = 0x1F0
	magicDXT = 0x1F3D5B79
)

const (
	auxValueTypeU32  = 3
	auxValueTypeU64 = 6
	auxValueTypeObj = 2
)

type Value struct {
	Type uint32
	Data any
}

type MessageAux struct {
	Values []*Value
}

func (x *MessageAux) AddU32(v uint32) *MessageAux {
	x.Values = append(x.Values, &Value{
		Type: auxValueTypeU32,
		Data: v,
	})
	return x
}

func (x *MessageAux) AddU64(v uint64) *MessageAux {
	x.Values = append(x.Values, &Value{
		Type: auxValueTypeU64,
		Data: v,
	})

	return x
}

func (x *MessageAux) AddObj(v any) *MessageAux {
	x.Values = append(x.Values, &Value{
		Type: auxValueTypeObj,
		Data: v,
	})
	return x
}

func (x *MessageAux) Decode(buf []byte) error {
	endian := binary.LittleEndian
	if endian.Uint64(buf) != magicAux {
		//return errors.New(`bad aux magic ` + hex.EncodeToString(buf[:8]))
	}

	if uint64(len(buf)-16) < endian.Uint64(buf[8:]) {
		return errors.New(`bad aux length`)
	}

	b := buf[8:]
	for len(b) > 0 {
		b = b[4:]
		t := endian.Uint32(b)
		b = b[4:]
		switch t {
		case auxValueTypeU32:
			x.AddU32(endian.Uint32(b))
			b = b[4:]
		case auxValueTypeU64:
			x.AddU64(endian.Uint64(b))
			b = b[8:]
		case auxValueTypeObj:
			num := endian.Uint32(b)
			b = b[4:]
			if len(b) < int(num) {
				return errors.New(`bad object length`)
			}
			nka := ns.NewNSKeyedArchiver()
			obj, err := nka.Unmarshal(b)
			if err != nil {return err}
			x.AddObj(obj)
			b = b[num:]
		}
	}

	return nil
}

func (x *MessageAux) Encode() ([]byte, error){
	endian := binary.LittleEndian
	rsv := make([]byte, 8)

	buf := &bytes.Buffer{}
	buf.Write(rsv[:8])
	buf.Write(rsv[:8])

	for _, v := range x.Values {
		endian.PutUint32(rsv, 0xa)
		buf.Write(rsv[:4])
		endian.PutUint32(rsv, v.Type)
		buf.Write(rsv[:4])
		switch v.Type {
		case auxValueTypeU32:
			endian.PutUint32(rsv, v.Data.(uint32))
			buf.Write(rsv[:4])
		case auxValueTypeU64:
			endian.PutUint64(rsv, v.Data.(uint64))
			buf.Write(rsv[:8])
		case auxValueTypeObj:
			nka := ns.NewNSKeyedArchiver()
			raw, err := nka.Marshal(v.Data)
			if err != nil {return nil, err}
			endian.PutUint32(rsv, uint32(len(raw)))
			buf.Write(rsv[:4])
			buf.Write(raw)
		}
	}

	raw := buf.Bytes()
	endian.PutUint64(raw[0:], magicAux)
	endian.PutUint64(raw[8:], uint64(buf.Len()-16))
	return raw, nil
}

type DTXMessageHeader struct {
	Magic         uint32
	Cb            uint32
	FragmentId    uint16
	FragmentCount uint16
	Length        uint32
	Identifier    uint32
	SessionIndex  uint32
	ChannelCode   int32
	ExpectReply   uint32
}

type DTXPayloadHeader struct {
	Flags           uint32
	AuxiliaryLength uint32
	TotalLength     uint64
}