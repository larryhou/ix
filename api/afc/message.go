package afc

import (
	"encoding/json"
	"time"
)

const (
	ServiceName    = `com.apple.afc`
)

const (
	opStatus        = 0x00000001
	opData          = 0x00000002 // Data
	opReadDir       = 0x00000003 // ReadDir
	opReadFile      = 0x00000004 // ReadFile
	opWriteFile     = 0x00000005 // WriteFile
	opWritePart     = 0x00000006 // WritePart
	opTruncate      = 0x00000007 // TruncateFile
	opRemovePath    = 0x00000008 // RemovePath
	opMakeDir       = 0x00000009 // MakeDir
	opGetFileInfo   = 0x0000000A // GetFileInfo
	opGetDevinfo    = 0x0000000B // GetDeviceInfo
	opWriteFileAtom = 0x0000000C // WriteFileAtomic (tmp file+rename)
	opFileOpen      = 0x0000000D // FileRefOpen
	opFileOpenRes   = 0x0000000E // FileRefOpenResult
	opRead          = 0x0000000F // FileRefRead
	opWrite         = 0x00000010 // FileRefWrite
	opFileSeek      = 0x00000011 // FileRefSeek
	opFileTell      = 0x00000012 // FileRefTell
	opFileTellRes   = 0x00000013 // FileRefTellResult
	opFileClose     = 0x00000014 // FileRefClose
	opFileSetSize   = 0x00000015 // FileRefSetFileSize (ftruncate)
	opGetConInfo    = 0x00000016 // GetConnectionInfo
	opSetConOptions = 0x00000017 // SetConnectionOptions
	opRenamePath    = 0x00000018 // RenamePath
	opSetFsBs       = 0x00000019 // SetFSBlockSize (0x800000)
	opSetSocketBs   = 0x0000001A // SetSocketBlockSize (0x800000)
	opFileLock      = 0x0000001B // FileRefLock
	opMakeLink      = 0x0000001C // MakeLink
	opSetFileTime   = 0x0000001E // set st_mtime
)

type Error int

func (x Error) Error() string {
	return Retcode(x).String()
}

type Retcode int

const (
	retSuccess             Retcode = 0
	retUnknownError        Retcode = 1
	retOpHeaderInvalid     Retcode = 2
	retNoResources         Retcode = 3
	retReadError           Retcode = 4
	retWriteError          Retcode = 5
	retUnknownPacketType   Retcode = 6
	retInvalidArg          Retcode = 7
	retObjectNotFound      Retcode = 8
	retObjectIsDir         Retcode = 9
	retPermDenied          Retcode = 10
	retServiceNotConnected Retcode = 11
	retOpTimeout           Retcode = 12
	retTooMuchData         Retcode = 13
	retEndOfData           Retcode = 14
	retOpNotSupported      Retcode = 15
	retObjectExists        Retcode = 16
	retObjectBusy          Retcode = 17
	retNoSpaceLeft         Retcode = 18
	retOpWouldBlock        Retcode = 19
	retIoError             Retcode = 20
	retOpInterrupted       Retcode = 21
	retOpInProgress        Retcode = 22
	retInternalError       Retcode = 23
	retMuxError            Retcode = 30
	retNoMem               Retcode = 31
	retNotEnoughData       Retcode = 32
	retDirNotEmpty         Retcode = 33
)

func (r Retcode) String() string {
	switch r {
	case retSuccess: return `Success`
	case retUnknownError: return `UnknownError`
	case retOpHeaderInvalid: return `OpHeaderInvalid`
	case retNoResources: return `NoResources`
	case retReadError: return `ReadError`
	case retWriteError: return `WriteError`
	case retUnknownPacketType: return `UnknownPacketType`
	case retInvalidArg: return `InvalidArg`
	case retObjectNotFound: return `ObjectNotFound`
	case retObjectIsDir: return `ObjectIsDir`
	case retPermDenied: return `PermDenied`
	case retServiceNotConnected: return `ServiceNotConnected`
	case retOpTimeout: return `OpTimeout`
	case retTooMuchData: return `TooMuchData`
	case retEndOfData: return `EndOfData`
	case retOpNotSupported: return `OpNotSupported`
	case retObjectExists: return `ObjectExists`
	case retObjectBusy: return `ObjectBusy`
	case retNoSpaceLeft: return `NoSpaceLeft`
	case retOpWouldBlock: return `OpWouldBlock`
	case retIoError: return `IoError`
	case retOpInterrupted: return `OpInterrupted`
	case retOpInProgress: return `OpInProgress`
	case retInternalError: return `InternalError`
	case retMuxError: return `MuxError`
	case retNoMem: return `NoMem`
	case retNotEnoughData: return `NotEnoughData`
	case retDirNotEmpty: return `DirNotEmpty`
	default:
		return `UnknownError`
	}
}

const (
	Hardlink = 1
	Symlink  = 2
)

const (
	permRDONLY   = 0x00000001 // r   O_RDONLY
	permRW       = 0x00000002 // r+  O_RDWR   | O_CREAT
	permWRONLY   = 0x00000003 // w   O_WRONLY | O_CREAT  | O_TRUNC
	permWR       = 0x00000004 // w+  O_RDWR   | O_CREAT  | O_TRUNC
	permAPPEND   = 0x00000005 // a   O_WRONLY | O_APPEND | O_CREAT
	permRDAPPEND = 0x00000006 // a+  O_RDWR   | O_APPEND | O_CREAT
)

const (
	magic            = `CFA6LPAA`
	maximumWriteSize = 1 << 30
	headerSize       = 40
)

const (
	LockSh = 1 | 4 // shared lock
	LockEx = 2 | 4 // exclusive lock
	LockUn = 8 | 4 // unlock
)

type String string

func (x *String) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, (*string)(x))
	}

	*x = String(b)
	return nil
}

type Time time.Time

func (x *Time) String() string {
	return (*time.Time)(x).String()
}

func (x *Time) UnmarshalJSON(b []byte) error {
	val := int64(0)
	err := json.Unmarshal(b, &val)
	if err == nil {
		second := int64(time.Second)
		*x = Time(time.Unix(val/second, val%second))
	}

	return err
}

func (x *Time) MarshalJSON() ([]byte, error) {
	return json.Marshal((*time.Time)(x))
}

type FileStat struct {
	Birthtime *Time  `json:"st_birthtime"`
	Blocks    int    `json:"st_blocks"`
	Ifmt      String `json:"st_ifmt"`
	Mtime     *Time  `json:"st_mtime"`
	Nlink     int    `json:"st_nlink"`
	Size      int64  `json:"st_size"`
	Name      string `json:"st_name"`
}

func (f *FileStat) IsDir() bool {
	return f.Ifmt == `S_IFDIR`
}
