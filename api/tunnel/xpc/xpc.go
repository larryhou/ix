package xpc

import (
	"bytes"
	"errors"
	"io"
)

const (
	TypeNull            = 0x00001000
	TypeBool            = 0x00002000
	TypeInt64           = 0x00003000
	TypeUint64          = 0x00004000
	TypeDouble          = 0x00005000
	TypePointer         = 0x00006000
	TypeDate            = 0x00007000
	TypeData            = 0x00008000
	TypeString          = 0x00009000
	TypeUuid            = 0x0000a000
	TypeFd              = 0x0000b000
	TypeShmem           = 0x0000c000
	TypeMachSend        = 0x0000d000
	TypeArray           = 0x0000e000
	TypeDictionary      = 0x0000f000
	TypeError           = 0x00010000
	TypeConnection      = 0x00011000
	TypeEndpoint        = 0x00012000
	TypeSERIALIZER      = 0x00013000
	TypePipe            = 0x00014000
	TypeMachRecv        = 0x00015000
	TypeBundle          = 0x00016000
	TypeService         = 0x00017000
	TypeServiceInstance = 0x00018000
	TypeActivity        = 0x00019000
	TypeFileTransfer    = 0x0001a000
)

const (
	FlagAlwaysSet            = 0x00000001
	FlagPing                 = 0x00000002
	FlagDataPresent          = 0x00000100
	FlagWantingReply         = 0x00010000
	FlagReply                = 0x00020000
	FlagFileTxStreamRequest  = 0x00100000
	FlagFileTxStreamResponse = 0x00200000
	FlagInitHandshake        = 0x00400000
)

const (
	MagicPayload = 0x42133742
	MagicMessage = 0x29b00b92
)

const (
	Version = 0x00000005
)

type (
	Shmem uint64
	Fd    uint32
)

type FileTransfer struct {
	MsgId int64
	File  any
}

type Payload struct {
	Magic   int
	Version int
	Data    any
}

type Message struct {
	Id   int64
	Flag int
	*Payload
}

func Encode(w io.Writer, msg *Message) error {
	encoder := NewEncoder(w)
	err := encoder.u32(MagicMessage)
	if err == nil {
		flag := msg.Flag | FlagAlwaysSet
		err = encoder.u32(uint32(flag))
	}

	payload := msg.Payload
	if payload != nil {
		payload.Magic = MagicPayload
		if payload.Version == 0 {
			payload.Version = Version
		}
	}

	if err == nil {
		buf := &bytes.Buffer{}
		tmp := NewEncoder(buf)
		tmp.u64(0) // packet size
		tmp.s64(msg.Id)
		if payload == nil {
			return encoder.put(buf.Bytes())
		}

		tmp.u32(uint32(payload.Magic))
		tmp.u32(uint32(payload.Version))
		switch data := payload.Data.(type) {
		case *io.LimitedReader:
			tmp.b.PutUint64(buf.Bytes(), uint64(int64(buf.Len())-16+data.N))
			err = encoder.put(buf.Bytes())
			if err == nil {
				_, err = io.Copy(w, data)
			}
		default:
			err = tmp.object(payload.Data)
			if err == nil {
				tmp.b.PutUint64(buf.Bytes(), uint64(buf.Len()-16))
				err = encoder.put(buf.Bytes())
			}
		}
	}

	return err
}

func Decode(r io.Reader, msg *Message) error {
	decoder := NewDecoder(r)
	magic, err := decoder.u32()
	if err != nil || magic != MagicMessage {
		return errors.New(`invalid packet magic`)
	}

	msg.Flag, err = decoder.u32()
	if err != nil {return err}

	num, err := decoder.s64()
	if err != nil {return err}

	msg.Id, err = decoder.s64()
	if err != nil || num == 0 {return err}

	magic, err = decoder.u32()
	if err != nil || magic != MagicPayload {
		return errors.New(`invalid payload magic`)
	}

	if msg.Payload == nil {
		msg.Payload = &Payload{}
	}

	msg.Magic = magic
	msg.Version, err = decoder.u32()
	if err != nil {return err}

	num -= 8

	switch data := msg.Data.(type) {
	case io.Writer:
		_, err = io.Copy(data, io.LimitReader(r, num))
	case nil:
		mem := make([]byte, num)
		err = decoder.get(mem)
		if err == nil {
			tmp := NewDecoder(bytes.NewReader(mem))
			msg.Data, err = tmp.object()
		}

	default:
		err = errors.New(`invalid payload data type`)
	}

	return err
}