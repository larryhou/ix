package pcapd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/larryhou/ix/api/j3/plist"
	"io"
	"net"
	"time"
)

const (
	ServiceName    = `com.apple.pcapd`
	ServiceNameRSD = `com.apple.pcapd.shim.remote`
)

// Interface type identifiers reported in the pcapd packet header.
const (
	InterfaceWIFI      = 1
	InterfaceCellular  = 2
	InterfaceLoopback  = 3
	InterfaceEthernet  = 5
	InterfaceOtherIP   = 8
)

// fakeEthernetHeader is prepended to raw IP packets so that the resulting
// capture data has a valid Ethernet II frame header expected by libpcap tools.
// Destination and source MAC are set to the broadcast address 0xBEEFBEEFBEEF;
// EtherType 0x0800 signals IPv4.
var fakeEthernetHeader = []byte{
	0xBE, 0xEF, 0xBE, 0xEF, 0xBE, 0xEF, // dst MAC
	0xBE, 0xEF, 0xBE, 0xEF, 0xBE, 0xEF, // src MAC
	0x08, 0x00,                           // EtherType: IPv4
}

// Packet holds a single captured network packet and its metadata.
type Packet struct {
	Timestamp      time.Time
	InterfaceName  string
	InterfaceType  uint8
	PID            uint32
	ProcessName    string
	EPID           uint32
	EProcessName   string
	ProtocolFamily uint32
	IO             uint8 // 0 = inbound, 1 = outbound
	Data           []byte
}

// pcapdHeader mirrors the binary structure sent by the pcapd daemon.
// All multi-byte fields are big-endian unless noted.
type pcapdHeader struct {
	HeaderLength    uint32
	HeaderVersion   uint8
	PacketLength    uint32
	InterfaceType   uint8
	Unit            uint16
	IO              uint8
	ProtocolFamily  uint32
	FramePreLength  uint32
	FramePostLength uint32
	InterfaceName   [16]byte
	PID             uint32 // little-endian
	Comm            [17]byte
	_               [3]byte // padding
	SVC             uint32
	EPID            uint32 // little-endian
	EComm           [17]byte
	_               [3]byte // padding
	Seconds         uint32
	Microseconds    uint32
}

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// Recv reads and decodes the next captured packet from the device.
// The service begins streaming immediately upon connection with no explicit request.
func (x *Service) Recv() (*Packet, error) {
	// Each frame arrives as a plist <data> blob — recv_plist returns raw bytes.
	var raw []byte
	if err := x.Connection.Recv(&raw); err != nil {
		return nil, err
	}
	return decodePacket(raw)
}

// decodePacket parses the raw binary payload produced by the pcapd daemon.
func decodePacket(raw []byte) (*Packet, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("pcapd: packet too short (%d bytes)", len(raw))
	}

	r := bytes.NewReader(raw)

	// Read the fixed-size header fields individually using the documented layout.
	headerLength := binary.BigEndian.Uint32(raw[0:4])
	if int(headerLength) > len(raw) {
		return nil, fmt.Errorf("pcapd: header length %d exceeds packet size %d", headerLength, len(raw))
	}

	_ = r // use binary.Read for structured access
	hr := bytes.NewReader(raw)

	var hdrLen uint32
	var hdrVer uint8
	var pktLen uint32
	var ifaceType uint8
	var unit uint16
	var ioDir uint8
	var protoFamily uint32
	var framePreLen uint32
	var framePostLen uint32
	var ifaceName [16]byte
	var pid uint32
	var comm [17]byte
	var pad1 [3]byte
	var svc uint32
	var epid uint32
	var ecomm [17]byte
	var pad2 [3]byte
	var seconds uint32
	var microseconds uint32

	be := binary.BigEndian
	le := binary.LittleEndian

	read := func(dst any, order binary.ByteOrder) error {
		return binary.Read(hr, order, dst)
	}

	if err := read(&hdrLen, be); err != nil {
		return nil, err
	}
	if err := read(&hdrVer, be); err != nil {
		return nil, err
	}
	_ = hdrVer
	if err := read(&pktLen, be); err != nil {
		return nil, err
	}
	if err := read(&ifaceType, be); err != nil {
		return nil, err
	}
	if err := read(&unit, be); err != nil {
		return nil, err
	}
	if err := read(&ioDir, be); err != nil {
		return nil, err
	}
	if err := read(&protoFamily, be); err != nil {
		return nil, err
	}
	if err := read(&framePreLen, be); err != nil {
		return nil, err
	}
	if err := read(&framePostLen, be); err != nil {
		return nil, err
	}
	_ = framePostLen
	if err := read(&ifaceName, be); err != nil {
		return nil, err
	}
	// PID and EPID are little-endian.
	if err := read(&pid, le); err != nil {
		return nil, err
	}
	if err := read(&comm, be); err != nil {
		return nil, err
	}
	if err := read(&pad1, be); err != nil {
		return nil, err
	}
	_ = pad1
	if err := read(&svc, be); err != nil {
		return nil, err
	}
	_ = svc
	if err := read(&epid, le); err != nil {
		return nil, err
	}
	if err := read(&ecomm, be); err != nil {
		return nil, err
	}
	if err := read(&pad2, be); err != nil {
		return nil, err
	}
	_ = pad2
	if err := read(&seconds, be); err != nil {
		return nil, err
	}
	if err := read(&microseconds, be); err != nil {
		return nil, err
	}

	// Skip to the end of the header to reach packet data.
	dataOffset := int(headerLength)
	if dataOffset > len(raw) {
		return nil, fmt.Errorf("pcapd: data offset %d out of range", dataOffset)
	}
	data := raw[dataOffset:]
	if int(pktLen) <= len(data) {
		data = data[:pktLen]
	}

	ifaceNameStr := cstring(ifaceName[:])
	commStr := cstring(comm[:])
	ecommStr := cstring(ecomm[:])

	// Prepend a fake Ethernet header for interfaces that carry raw IP frames.
	if framePreLen == 0 || ifaceNameStr == "pdp_ip" {
		if ifaceNameStr == "pdp_ip" && len(data) > 4 {
			// Skip the 4-byte protocol family prefix prepended by pdp_ip.
			data = data[4:]
		}
		data = append(append([]byte(nil), fakeEthernetHeader...), data...)
	}

	_ = headerLength // already used above via hdrLen

	return &Packet{
		Timestamp:      time.Unix(int64(seconds), int64(microseconds)*1000),
		InterfaceName:  ifaceNameStr,
		InterfaceType:  ifaceType,
		PID:            pid,
		ProcessName:    commStr,
		EPID:           epid,
		EProcessName:   ecommStr,
		ProtocolFamily: protoFamily,
		IO:             ioDir,
		Data:           data,
	}, nil
}

// WritePcapGlobalHeader writes a standard libpcap global file header to w.
// linkType should be 1 (Ethernet) when fakeEthernetHeader is prepended.
func WritePcapGlobalHeader(w io.Writer, linkType uint32) error {
	// Magic number, version, GMT offset, accuracy, max packet length, link type.
	hdr := struct {
		MagicNumber  uint32
		VersionMajor uint16
		VersionMinor uint16
		ThisZone     int32
		SigFigs      uint32
		SnapLen      uint32
		LinkType     uint32
	}{
		MagicNumber:  0xA1B2C3D4,
		VersionMajor: 2,
		VersionMinor: 4,
		SnapLen:      65535,
		LinkType:     linkType,
	}
	return binary.Write(w, binary.LittleEndian, hdr)
}

// WritePcapPacket writes a single libpcap packet record to w.
func WritePcapPacket(w io.Writer, pkt *Packet) error {
	sec := uint32(pkt.Timestamp.Unix())
	usec := uint32(pkt.Timestamp.Nanosecond() / 1000)
	length := uint32(len(pkt.Data))
	rec := struct {
		TsSec   uint32
		TsUsec  uint32
		InclLen uint32
		OrigLen uint32
	}{sec, usec, length, length}
	if err := binary.Write(w, binary.LittleEndian, rec); err != nil {
		return err
	}
	_, err := w.Write(pkt.Data)
	return err
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
