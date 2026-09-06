package syslog

import (
	"bufio"
	"bytes"
	"io"
	"net"
)

const (
	ServiceName = `com.apple.syslog_relay`
)

func New(conn net.Conn) *Service {
	s := &Service{Conn: conn}
	return s
}

type Service struct {
	net.Conn
}

// splitNull splits on the NUL byte (0x00) which syslog_relay uses as
// message delimiter. Each token is one complete log message (may contain
// embedded newlines for multi-line entries).
func splitNull(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0x00); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (x *Service) Streaming(w io.Writer) error {
	s := bufio.NewScanner(x.Conn)
	s.Buffer(make([]byte, 1024*1024), 1024*1024) // some messages can be large
	s.Split(splitNull)
	for s.Scan() {
		msg := s.Bytes()
		// strip leading \n if present (first message has no leading NUL)
		msg = bytes.TrimLeft(msg, "\n")
		if len(msg) == 0 {
			continue
		}
		// deliver each message as a single Write call (content + newline)
		// so consumers can treat one Write = one complete log entry
		w.Write(append(msg, '\n'))
	}
	return s.Err()
}
