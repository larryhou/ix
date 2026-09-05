package xpc

import (
	"bytes"
	"fmt"
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

const Version = 0x00000005

// maxMessageSize caps the outer payload size read from the wire.
const maxMessageSize = 512 << 20 // 512 MiB

type (
	Shmem uint64
	Fd    uint32
)

type FileTransfer struct {
	MsgId int64
	File  any
}

type Payload struct {
	Magic   uint32
	Version uint32
	Data    any
}

type Message struct {
	Id   int64
	Flag uint32
	*Payload
}

func (x *Message) HasData() bool {
	return x.Flag&FlagDataPresent != 0
}

func Encode(w io.Writer, msg *Message) error {
	encoder := NewEncoder(w)
	if err := encoder.u32(MagicMessage); err != nil {
		return err
	}
	flag := msg.Flag | FlagAlwaysSet
	if err := encoder.u32(flag); err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	tmp := NewEncoder(buf)
	if err := tmp.u64(0); err != nil { // packet-size placeholder
		return err
	}
	if err := tmp.s64(msg.Id); err != nil {
		return err
	}

	payload := msg.Payload
	if payload == nil {
		// No payload: back-patch size = 0 (already zero) and flush.
		return encoder.put(buf.Bytes())
	}

	// Fill payload header fields locally; do not mutate the caller's struct.
	magic := uint32(MagicPayload)
	version := payload.Version
	if version == 0 {
		version = Version
	}
	if err := tmp.u32(magic); err != nil {
		return err
	}
	if err := tmp.u32(version); err != nil {
		return err
	}

	switch data := payload.Data.(type) {
	case *io.LimitedReader:
		// Streaming path: size = bytes written so far (header) + remaining stream bytes.
		headerSize := int64(buf.Len()) - 16 // subtract size(8) + id(8) fields
		tmp.b.PutUint64(buf.Bytes(), uint64(headerSize+data.N))
		if err := encoder.put(buf.Bytes()); err != nil {
			return err
		}
		_, err := io.Copy(w, data)
		return err
	default:
		if err := tmp.object(payload.Data); err != nil {
			return err
		}
		tmp.b.PutUint64(buf.Bytes(), uint64(buf.Len()-16))
		return encoder.put(buf.Bytes())
	}
}

func Decode(r io.Reader, msg *Message) error {
	decoder := NewDecoder(r)

	magic, err := decoder.u32()
	if err != nil {
		return fmt.Errorf("xpc: read message magic: %w", err)
	}
	if magic != MagicMessage {
		return fmt.Errorf("xpc: invalid message magic: 0x%08x", magic)
	}

	msg.Flag, err = decoder.u32()
	if err != nil {
		return fmt.Errorf("xpc: read flags: %w", err)
	}

	// Payload size covers everything after the 8-byte size field and 8-byte ID.
	payloadSize, err := decoder.u64()
	if err != nil {
		return fmt.Errorf("xpc: read payload size: %w", err)
	}

	msg.Id, err = decoder.s64()
	if err != nil {
		return fmt.Errorf("xpc: read message id: %w", err)
	}

	if payloadSize == 0 {
		return nil // no payload; message is just a header frame
	}

	if payloadSize > maxMessageSize {
		return fmt.Errorf("xpc: payload size %d exceeds limit %d", payloadSize, maxMessageSize)
	}

	payloadMagic, err := decoder.u32()
	if err != nil {
		return fmt.Errorf("xpc: read payload magic: %w", err)
	}
	if payloadMagic != MagicPayload {
		return fmt.Errorf("xpc: invalid payload magic: 0x%08x", payloadMagic)
	}

	if msg.Payload == nil {
		msg.Payload = &Payload{}
	}
	msg.Payload.Magic = payloadMagic

	msg.Payload.Version, err = decoder.u32()
	if err != nil {
		return fmt.Errorf("xpc: read payload version: %w", err)
	}

	// Remaining bytes after the payload header (magic + version = 8 bytes).
	dataSize := int64(payloadSize) - 8

	switch data := msg.Data.(type) {
	case io.Writer:
		_, err = io.Copy(data, io.LimitReader(r, dataSize))
		return err
	case nil:
		if dataSize < 0 {
			return fmt.Errorf("xpc: negative data size: %d", dataSize)
		}
		mem := make([]byte, dataSize)
		if err = decoder.get(mem); err != nil {
			return fmt.Errorf("xpc: read payload data: %w", err)
		}
		tmp := NewDecoder(bytes.NewReader(mem))
		msg.Data, err = tmp.object()
		return err
	default:
		return fmt.Errorf("xpc: unsupported payload data type: %T", msg.Data)
	}
}
