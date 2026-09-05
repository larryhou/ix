// Package networkmonitor implements the com.apple.instruments.server.services.networking
// DVT channel. It streams per-connection and per-interface network traffic events.
package networkmonitor

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
)

const channelIdentifier = `com.apple.instruments.server.services.networking`

// EventType classifies the kind of network event received from the device.
type EventType int

const (
	// EventInterface is fired when a new network interface is detected.
	EventInterface EventType = 0
	// EventConnection is fired when a new TCP/UDP connection is established.
	EventConnection EventType = 1
	// EventUpdate carries per-connection traffic counters.
	EventUpdate EventType = 2
)

// InterfaceEvent describes a newly detected network interface.
type InterfaceEvent struct {
	Index int64
	Name  string
}

// ConnectionEvent describes a newly established connection.
type ConnectionEvent struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr
	IfIndex    int64
	PID        int64
	RecvBufSize int64
	RecvBufUsed int64
	Serial     int64
	Kind       int64
}

// UpdateEvent carries traffic counters for an existing connection.
type UpdateEvent struct {
	RxPackets  int64
	RxBytes    int64
	TxPackets  int64
	TxBytes    int64
	RxDups     int64
	Rx000      int64
	TxRetx     int64
	MinRTT     float64
	AvgRTT     float64
	ConnSerial int64
	Timestamp  float64
}

// Event is the union type delivered on the Events channel.
type Event struct {
	Type       EventType
	Interface  *InterfaceEvent
	Connection *ConnectionEvent
	Update     *UpdateEvent
}

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(channelIdentifier)
	if err != nil {
		return nil, err
	}
	return &Service{ch: svr.GetChannel(id)}, nil
}

type Service struct {
	ch *remotesvr.DTXChannel
}

// Start begins streaming network events. Returns a channel of decoded events
// and an error channel. The event channel is closed when monitoring stops.
func (s *Service) Start() (<-chan *Event, <-chan error) {
	evCh := make(chan *Event, 64)
	errCh := make(chan error, 1)

	// Fire-and-forget start; no reply expected.
	if err := s.ch.Send("startMonitoring", new(remotesvr.ArgumentAux), false); err != nil {
		errCh <- err
		close(evCh)
		close(errCh)
		return evCh, errCh
	}

	go func() {
		defer close(evCh)
		defer close(errCh)
		for {
			obj, err := s.ch.Recv(nil)
			if err != nil {
				errCh <- err
				return
			}
			ev, err := decodeEvent(obj)
			if err != nil {
				// Skip unparseable events rather than aborting.
				continue
			}
			evCh <- ev
		}
	}()

	return evCh, errCh
}

// Stop asks the device to stop streaming network events.
func (s *Service) Stop() error {
	return s.ch.Send("stopMonitoring", new(remotesvr.ArgumentAux), true)
}

// decodeEvent converts the raw DTX dispatch payload into a typed Event.
// The device sends an array [message_type, payload_array].
func decodeEvent(raw any) (*Event, error) {
	arr, ok := raw.([]any)
	if !ok || len(arr) < 2 {
		return nil, fmt.Errorf("networkmonitor: unexpected event shape %T", raw)
	}

	msgType, ok := toInt64(arr[0])
	if !ok {
		return nil, fmt.Errorf("networkmonitor: non-integer event type %T", arr[0])
	}

	payload, ok := arr[1].([]any)
	if !ok {
		return nil, fmt.Errorf("networkmonitor: non-array payload %T", arr[1])
	}

	switch EventType(msgType) {
	case EventInterface:
		return decodeInterface(payload)
	case EventConnection:
		return decodeConnection(payload)
	case EventUpdate:
		return decodeUpdate(payload)
	default:
		return nil, fmt.Errorf("networkmonitor: unknown event type %d", msgType)
	}
}

func decodeInterface(p []any) (*Event, error) {
	if len(p) < 2 {
		return nil, fmt.Errorf("networkmonitor: short interface event")
	}
	idx, _ := toInt64(p[0])
	name, _ := p[1].(string)
	return &Event{
		Type:      EventInterface,
		Interface: &InterfaceEvent{Index: idx, Name: name},
	}, nil
}

func decodeConnection(p []any) (*Event, error) {
	if len(p) < 8 {
		return nil, fmt.Errorf("networkmonitor: short connection event")
	}
	local := parseSockAddr(p[0])
	remote := parseSockAddr(p[1])
	ifIdx, _ := toInt64(p[2])
	pid, _ := toInt64(p[3])
	recvBufSize, _ := toInt64(p[4])
	recvBufUsed, _ := toInt64(p[5])
	serial, _ := toInt64(p[6])
	kind, _ := toInt64(p[7])

	return &Event{
		Type: EventConnection,
		Connection: &ConnectionEvent{
			LocalAddr:   local,
			RemoteAddr:  remote,
			IfIndex:     ifIdx,
			PID:         pid,
			RecvBufSize: recvBufSize,
			RecvBufUsed: recvBufUsed,
			Serial:      serial,
			Kind:        kind,
		},
	}, nil
}

func decodeUpdate(p []any) (*Event, error) {
	if len(p) < 11 {
		return nil, fmt.Errorf("networkmonitor: short update event")
	}
	rxPkts, _ := toInt64(p[0])
	rxBytes, _ := toInt64(p[1])
	txPkts, _ := toInt64(p[2])
	txBytes, _ := toInt64(p[3])
	rxDups, _ := toInt64(p[4])
	rx000, _ := toInt64(p[5])
	txRetx, _ := toInt64(p[6])
	minRTT, _ := toFloat64(p[7])
	avgRTT, _ := toFloat64(p[8])
	serial, _ := toInt64(p[9])
	ts, _ := toFloat64(p[10])

	return &Event{
		Type: EventUpdate,
		Update: &UpdateEvent{
			RxPackets:  rxPkts,
			RxBytes:    rxBytes,
			TxPackets:  txPkts,
			TxBytes:    txBytes,
			RxDups:     rxDups,
			Rx000:      rx000,
			TxRetx:     txRetx,
			MinRTT:     minRTT,
			AvgRTT:     avgRTT,
			ConnSerial: serial,
			Timestamp:  ts,
		},
	}, nil
}

// parseSockAddr decodes the binary socket address blob sent by the device.
// IPv4 is 16 bytes; IPv6 is 28 bytes.
func parseSockAddr(v any) net.Addr {
	b, ok := v.([]byte)
	if !ok || len(b) < 8 {
		return nil
	}
	// b[0]: length, b[1]: family, b[2:4]: port BE
	port := binary.BigEndian.Uint16(b[2:4])
	switch b[1] {
	case 2: // AF_INET
		if len(b) < 8 {
			return nil
		}
		ip := net.IP(b[4:8])
		return &net.TCPAddr{IP: ip, Port: int(port)}
	case 30: // AF_INET6
		if len(b) < 24 {
			return nil
		}
		ip := net.IP(b[8:24])
		return &net.TCPAddr{IP: ip, Port: int(port)}
	}
	return nil
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		return int64(n), true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}
