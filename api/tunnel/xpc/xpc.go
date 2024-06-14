package xpc


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


type (
	Shmem uint64
	Fd    uint32
)

type FileTransfer struct {
	MsgId int
	Data  any
}
