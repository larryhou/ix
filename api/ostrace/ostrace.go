package ostrace

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/larryhou/ix/api/mux/plist"
	"io"
	"net"
	"time"

	xplist "howett.net/plist"
)

const (
	ServiceName    = `com.apple.os_trace_relay`
	ServiceNameRSD = `com.apple.os_trace_relay.shim.remote`
)

// StreamFlags controls which log entries are included in the activity stream.
// Values may be OR-combined.
const (
	StreamFlagProcessOnly  = 0x01
	StreamFlagSkipDecode   = 0x02
	StreamFlagPayload      = 0x04
	StreamFlagHistorical   = 0x08
	StreamFlagCallstack    = 0x10
	StreamFlagDebug        = 0x20
	StreamFlagNoSensitive  = 0x80
	StreamFlagInfo         = 0x100
	StreamFlagPromiscuous  = 0x200

	// StreamFlagsDefault is the commonly used combination for general log streaming.
	StreamFlagsDefault = StreamFlagPayload | StreamFlagHistorical | StreamFlagCallstack | StreamFlagDebug
)

// Log levels reported in Activity entries.
const (
	LevelNotice     = 0x00
	LevelInfo       = 0x01
	LevelDebug      = 0x02
	LevelUserAction = 0x03
	LevelError      = 0x10
	LevelFault      = 0x11
)

// Activity represents a single unified log entry decoded from the binary stream.
type Activity struct {
	Timestamp    time.Time
	Level        uint8
	PID          uint32
	ThreadID     uint64
	MachTimestamp uint64
	ProcessName  string
	ImageName    string
	Message      string
	Subsystem    string
	Category     string
}

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// PidList returns a map of pid (string) → process name for all running processes.
func (x *Service) PidList() (map[string]string, error) {
	if err := x.Send(map[string]any{"Request": "PidList"}); err != nil {
		return nil, err
	}

	// Discard the 1-byte unknown prefix.
	if _, err := io.ReadFull(x.Conn, make([]byte, 1)); err != nil {
		return nil, fmt.Errorf("ostrace: PidList prefix: %w", err)
	}

	var raw map[string]any
	if err := x.Connection.Recv(&raw); err != nil {
		return nil, err
	}

	payload, ok := raw["Payload"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ostrace: missing Payload in PidList response")
	}

	result := make(map[string]string, len(payload))
	for pidStr, info := range payload {
		if m, ok := info.(map[string]any); ok {
			name, _ := m["ProcessName"].(string)
			result[pidStr] = name
		}
	}
	return result, nil
}

// StartActivity begins streaming live log entries from the device.
// pid may be -1 to receive log entries from all processes.
// flags controls which entry types are included (use StreamFlagsDefault for general use).
// The returned channel delivers decoded Activity entries; it is closed when the
// underlying connection is terminated or an error occurs. The error is sent on errCh.
func (x *Service) StartActivity(pid int, flags int) (<-chan *Activity, <-chan error) {
	ch := make(chan *Activity, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)

		req := map[string]any{
			"Request":       "StartActivity",
			"MessageFilter": flags,
			"Pid":           pid,
			"StreamFlags":   flags,
		}
		if err := x.Send(req); err != nil {
			errCh <- err
			return
		}

		// Read the variable-length framing that precedes the status plist.
		// Format: [uint32 LE: length_length][length_length bytes BE: payload_length]
		var lenLen uint32
		if err := binary.Read(x.Conn, binary.LittleEndian, &lenLen); err != nil {
			errCh <- fmt.Errorf("ostrace: handshake lenLen: %w", err)
			return
		}
		lenBuf := make([]byte, lenLen)
		if _, err := io.ReadFull(x.Conn, lenBuf); err != nil {
			errCh <- fmt.Errorf("ostrace: handshake lenBuf: %w", err)
			return
		}
		// lenBuf is a big-endian integer of lenLen bytes (reversed order per Python impl).
		var payloadLen uint64
		for _, b := range lenBuf {
			payloadLen = payloadLen<<8 | uint64(b)
		}
		statusBuf := make([]byte, payloadLen)
		if _, err := io.ReadFull(x.Conn, statusBuf); err != nil {
			errCh <- fmt.Errorf("ostrace: handshake status: %w", err)
			return
		}
		// statusBuf is a plist; verify Status == "RequestSuccessful".
		var statusRsp map[string]any
		if _, err := xplist.Unmarshal(statusBuf, &statusRsp); err == nil {
			if s, _ := statusRsp["Status"].(string); s != "RequestSuccessful" {
				errCh <- fmt.Errorf("ostrace: StartActivity rejected: %v", statusRsp)
				return
			}
		}

		// Read log records: each prefixed by 0x02 magic then uint32 LE record length.
		for {
			magic := make([]byte, 1)
			if _, err := io.ReadFull(x.Conn, magic); err != nil {
				errCh <- err
				return
			}
			if magic[0] != 0x02 {
				errCh <- fmt.Errorf("ostrace: unexpected record magic 0x%02x", magic[0])
				return
			}

			var recLen uint32
			if err := binary.Read(x.Conn, binary.LittleEndian, &recLen); err != nil {
				errCh <- err
				return
			}

			recBuf := make([]byte, recLen)
			if _, err := io.ReadFull(x.Conn, recBuf); err != nil {
				errCh <- err
				return
			}

			act, err := decodeActivityRecord(recBuf)
			if err != nil {
				// Skip malformed records rather than aborting the stream.
				continue
			}
			ch <- act
		}
	}()

	return ch, errCh
}

// decodeActivityRecord parses the fixed-layout binary syslog record produced by os_trace_relay.
// The layout is documented in the protocol analysis (byte offsets are 0-based).
func decodeActivityRecord(b []byte) (*Activity, error) {
	if len(b) < 130 {
		return nil, fmt.Errorf("ostrace: record too short (%d bytes)", len(b))
	}

	le := binary.LittleEndian

	pid := le.Uint32(b[9:13])
	// procid at b[13:21] — not used in Activity struct
	// process_image_uuid at b[21:37] — not used
	seconds := le.Uint32(b[55:59])
	microseconds := le.Uint32(b[63:67])
	level := b[68]
	machTS := le.Uint64(b[73:81])
	threadID := le.Uint64(b[87:95])
	// image_uuid at b[95:111] — not used

	imageNameSize := le.Uint16(b[107:109])
	messageSize := le.Uint16(b[109:111])
	// skip 2 bytes at b[111:113]
	// sender_image_offset at b[113:117] — not used
	subsystemSize := le.Uint32(b[117:121])
	categorySize := le.Uint32(b[121:125])
	// skip 4 bytes at b[125:129]

	offset := 129
	// filename: null-terminated
	filenameEnd := bytes.IndexByte(b[offset:], 0)
	if filenameEnd < 0 {
		filenameEnd = len(b) - offset
	}
	// filename itself is not exposed in Activity; advance past it.
	offset += filenameEnd + 1

	imageName := ""
	if int(imageNameSize) > 0 && offset+int(imageNameSize) <= len(b) {
		imageName = cstring(b[offset : offset+int(imageNameSize)])
		offset += int(imageNameSize)
	}

	message := ""
	if int(messageSize) > 0 && offset+int(messageSize) <= len(b) {
		message = cstring(b[offset : offset+int(messageSize)])
		offset += int(messageSize)
	}

	subsystem := ""
	if subsystemSize > 0 && offset+int(subsystemSize) <= len(b) {
		subsystem = cstring(b[offset : offset+int(subsystemSize)])
		offset += int(subsystemSize)
	}

	category := ""
	if categorySize > 0 && offset+int(categorySize) <= len(b) {
		category = cstring(b[offset : offset+int(categorySize)])
	}

	return &Activity{
		Timestamp:     time.Unix(int64(seconds), int64(microseconds)*1000),
		Level:         level,
		PID:           pid,
		ThreadID:      threadID,
		MachTimestamp: machTS,
		ImageName:     imageName,
		Message:       message,
		Subsystem:     subsystem,
		Category:      category,
	}, nil
}

// cstring converts a null-terminated byte slice to a Go string.
func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
