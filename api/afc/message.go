package afc

import (
	"encoding/json"
	"time"
)

const (
	ServiceName    = `com.apple.afc`
	RSDServiceName = `com.apple.afc.shim.remote`
)

const (
	OpStatus        = 0x00000001
	OpData          = 0x00000002 // Data
	OpReadDir       = 0x00000003 // ReadDir
	OpReadFile      = 0x00000004 // ReadFile
	OpWriteFile     = 0x00000005 // WriteFile
	OpWritePart     = 0x00000006 // WritePart
	OpTruncate      = 0x00000007 // TruncateFile
	OpRemovePath    = 0x00000008 // RemovePath
	OpMakeDir       = 0x00000009 // MakeDir
	OpGetFileInfo   = 0x0000000A // GetFileInfo
	OpGetDevinfo    = 0x0000000B // GetDeviceInfo
	OpWriteFileAtom = 0x0000000C // WriteFileAtomic (tmp file+rename)
	OpFileOpen      = 0x0000000D // FileRefOpen
	OpFileOpenRes   = 0x0000000E // FileRefOpenResult
	OpRead          = 0x0000000F // FileRefRead
	OpWrite         = 0x00000010 // FileRefWrite
	OpFileSeek      = 0x00000011 // FileRefSeek
	OpFileTell      = 0x00000012 // FileRefTell
	OpFileTellRes   = 0x00000013 // FileRefTellResult
	OpFileClose     = 0x00000014 // FileRefClose
	OpFileSetSize   = 0x00000015 // FileRefSetFileSize (ftruncate)
	OpGetConInfo    = 0x00000016 // GetConnectionInfo
	OpSetConOptions = 0x00000017 // SetConnectionOptions
	OpRenamePath    = 0x00000018 // RenamePath
	OpSetFsBs       = 0x00000019 // SetFSBlockSize (0x800000)
	OpSetSocketBs   = 0x0000001A // SetSocketBlockSize (0x800000)
	OpFileLock      = 0x0000001B // FileRefLock
	OpMakeLink      = 0x0000001C // MakeLink
	OpSetFileTime   = 0x0000001E // set st_mtime
)

const (
	RetSuccess             = 0
	RetUnknownError        = 1
	RetOpHeaderInvalid     = 2
	RetNoResources         = 3
	RetReadError           = 4
	RetWriteError          = 5
	RetUnknownPacketType   = 6
	RetInvalidArg          = 7
	RetObjectNotFound      = 8
	RetObjectIsDir         = 9
	RetPermDenied          = 10
	RetServiceNotConnected = 11
	RetOpTimeout           = 12
	RetTooMuchData         = 13
	RetEndOfData           = 14
	RetOpNotSupported      = 15
	RetObjectExists        = 16
	RetObjectBusy          = 17
	RetNoSpaceLeft         = 18
	RetOpWouldBlock        = 19
	RetIoError             = 20
	RetOpInterrupted       = 21
	RetOpInProgress        = 22
	RetInternalError       = 23
	RetMuxError            = 30
	RetNoMem               = 31
	RetNotEnoughData       = 32
	RetDirNotEmpty         = 33
)

const (
	Hardlink = 1
	Symlink  = 2
)

const (
	RDONLY   = 0x00000001 // r   O_RDONLY
	RW       = 0x00000002 // r+  O_RDWR   | O_CREAT
	WRONLY   = 0x00000003 // w   O_WRONLY | O_CREAT  | O_TRUNC
	WR       = 0x00000004 // w+  O_RDWR   | O_CREAT  | O_TRUNC
	APPEND   = 0x00000005 // a   O_WRONLY | O_APPEND | O_CREAT
	RDAPPEND = 0x00000006 // a+  O_RDWR   | O_APPEND | O_CREAT
)

const (
	Magic            = `CFA6LPAA`
	MaximumWriteSize = 1 << 30
	HeaderSize       = 40
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
	Birthtime Time   `json:"st_birthtime"`
	Blocks    int    `json:"st_blocks"`
	Ifmt      String `json:"st_ifmt"`
	Mtime     Time   `json:"st_mtime"`
	Nlink     int    `json:"st_nlink"`
	Size      int64  `json:"st_size"`
	Name      string `json:"st_name"`
}

func (f *FileStat) IsDir() bool {
	return f.Ifmt == `S_IFDIR`
}
