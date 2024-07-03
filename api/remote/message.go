package remote

import (
	"bytes"
	"encoding/binary"
	"github.com/larryhou/iconsole/ns"
)

const (
	magicAux = 0x1F0
	magicDXT = 0x1F3D5B79
)

const (
	auxValueTypeU32  = 3
	auxValueTypeU64  = 6
	auxValueTypeObjc = 2
)

type Value struct {
	Type uint32
	Data any
}

type MessageAux struct {
	Values []*Value
}

func (x *MessageAux) AddU32(v uint32) {
	x.Values = append(x.Values, &Value{
		Type: auxValueTypeU32,
		Data: v,
	})
}

func (x *MessageAux) AddU64(v uint64) {
	x.Values = append(x.Values, &Value{
		Type: auxValueTypeU64,
		Data: v,
	})
}

func (x *MessageAux) AddObj(v any) {
	x.Values = append(x.Values, &Value{
		Type: auxValueTypeObjc,
		Data: v,
	})
}

func (x *MessageAux) Bytes() ([]byte, error){
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
		case auxValueTypeObjc:
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

type DXTMessageHeader struct {
	Magic         uint32
	Cb            uint32
	FragmentId    uint16
	FragmentCount uint16
	Length        uint32
	Identifier   uint32
	SessionIndex uint32
	ChannelCode  int32
	ExpectReply   uint32
}

type DXTPayloadHeader struct {
	Flags           uint32
	AuxiliaryLength uint32
	TotalLength     uint64
}