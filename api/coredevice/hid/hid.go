// Package hid implements the CoreDevice HID injection services.
// Two separate service endpoints are supported:
//
//   - IndigoHID (com.apple.coredevice.hid.indigo): button event injection via
//     pre-defined HID usage page/code pairs. Simpler but limited to button presses.
//
//   - UniversalHID (com.apple.coredevice.hid.universalhidservice): full HID report
//     injection. Supports touch, pointer (gesture), and virtual keyboard. Requires
//     an active video stream to be accepted by the device's backboardd.
package hid

import (
	"encoding/binary"
	"fmt"
	"github.com/larryhou/j3idevice/api/coredevice"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"time"
)

const (
	ServiceNameIndigo    = `com.apple.coredevice.hid.indigo`
	ServiceNameUniversal = `com.apple.coredevice.hid.universalhidservice`
)

const (
	featureIndigoButton    = `com.apple.coredevice.feature.remote.hid.button`
	featureUniversalHID    = `com.apple.coredevice.feature.remote.universalhidservice`
)

// Button state constants for IndigoHID.
const (
	ButtonStateDown     = uint64(1)
	ButtonStateUp       = uint64(2)
	ButtonStateCanceled = uint64(3)
)

// Common HID usage page / usage code pairs for IndigoHID button events.
const (
	UsagePageConsumer = uint64(0x0C)
	UsagePageKeyboard = uint64(0x07)
	UsagePageGeneric  = uint64(0x01)

	// Consumer page buttons
	UsageVolumeUp   = uint64(0xE9)
	UsageVolumeDown = uint64(0xEA)
	UsageMute       = uint64(0xE2)
	UsagePlay       = uint64(0xCD)

	// Generic desktop page
	UsagePower  = uint64(0x30)
	UsageSleep  = uint64(0x82)
	UsageWakeUp = uint64(0x83)
)

// Touch contact state values for touchscreen HID reports.
const (
	TouchContact = byte(0xC2) // finger down
	TouchRelease = byte(0x02) // finger up
)

// ServiceID values used by UniversalHID send_report.
const (
	ServiceIDTouchscreen = uint64(257)
	ServiceIDGesture     = uint64(1281)
)

// IndigoService wraps the Indigo HID button-event endpoint.
type IndigoService struct {
	*coredevice.Service
}

func NewIndigo(conn *xpc.RemoteXpcConnection) *IndigoService {
	return &IndigoService{Service: coredevice.NewService(conn)}
}

// SendButton fires a single button event (fire-and-forget; no response expected).
func (s *IndigoService) SendButton(usagePage, usageCode, state uint64) error {
	return s.Send(map[string]any{
		"messageType":       "IndigoButtonEvent",
		"featureIdentifier": featureIndigoButton,
		"payload": map[string]any{
			"state":     state,
			"usagePage": usagePage,
			"usageCode": usageCode,
		},
	})
}

// Press simulates a full button press (down + up) with a short hold duration.
func (s *IndigoService) Press(usagePage, usageCode uint64) error {
	if err := s.SendButton(usagePage, usageCode, ButtonStateDown); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return s.SendButton(usagePage, usageCode, ButtonStateUp)
}

// VolumeUp simulates pressing the volume-up button.
func (s *IndigoService) VolumeUp() error {
	return s.Press(UsagePageConsumer, UsageVolumeUp)
}

// VolumeDown simulates pressing the volume-down button.
func (s *IndigoService) VolumeDown() error {
	return s.Press(UsagePageConsumer, UsageVolumeDown)
}

// UniversalService wraps the Universal HID report injection endpoint.
type UniversalService struct {
	*coredevice.Service
}

func NewUniversal(conn *xpc.RemoteXpcConnection) *UniversalService {
	return &UniversalService{Service: coredevice.NewService(conn)}
}

// ListConnectedServices queries the available HID surfaces and returns the raw response.
func (s *UniversalService) ListConnectedServices() (any, error) {
	return s.SendRecv(map[string]any{
		"featureIdentifier": featureUniversalHID,
		"messageType":       "Request",
		"payload":           map[string]any{"connectedServices": map[string]any{}},
	})
}

// sendReport delivers a raw HID report to the given service surface ID.
func (s *UniversalService) sendReport(serviceID uint64, report []byte) error {
	return s.Send(map[string]any{
		"featureIdentifier": featureUniversalHID,
		"messageType":       "Request",
		"payload": map[string]any{
			"send": map[string]any{
				"_0": report,
				"_1": serviceID,
			},
		},
	})
}

// Touch sends a touchscreen HID report.
// x and y are in the device's logical coordinate space [0, 65535].
// state is TouchContact (finger down) or TouchRelease (finger up).
func (s *UniversalService) Touch(x, y uint16, state byte) error {
	report := buildTouchReport(x, y, state)
	return s.sendReport(ServiceIDTouchscreen, report)
}

// Tap simulates a finger tap at (x, y): contact then release after a brief delay.
func (s *UniversalService) Tap(x, y uint16) error {
	if err := s.Touch(x, y, TouchContact); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return s.Touch(x, y, TouchRelease)
}

// buildTouchReport constructs a 58-byte touchscreen HID report.
// Layout (all little-endian):
//
//	[0]    report ID  = 0x09
//	[1]    0x01
//	[2]    0x05
//	[3]    state      (TouchContact or TouchRelease)
//	[4:6]  x (uint16 LE)
//	[6:8]  y (uint16 LE)
//	[8:40] padding    (32 zero bytes)
//	[40]   0x02
//	[41:43] 0x00 0x00
//	[43:49] mach absolute time (48-bit LE)
//	[49:58] zero padding
func buildTouchReport(x, y uint16, state byte) []byte {
	b := make([]byte, 58)
	b[0] = 0x09
	b[1] = 0x01
	b[2] = 0x05
	b[3] = state
	binary.LittleEndian.PutUint16(b[4:6], x)
	binary.LittleEndian.PutUint16(b[6:8], y)
	// bytes 8–39 are zero padding
	b[40] = 0x02
	// bytes 41–42 are zero
	putMachTime(b[43:49])
	// bytes 49–57 are zero padding
	return b
}

// putMachTime writes the low 48 bits of the current mach absolute time
// (approximated as nanoseconds since process start) into a 6-byte LE field.
func putMachTime(dst []byte) {
	ns := uint64(time.Now().UnixNano())
	dst[0] = byte(ns)
	dst[1] = byte(ns >> 8)
	dst[2] = byte(ns >> 16)
	dst[3] = byte(ns >> 24)
	dst[4] = byte(ns >> 32)
	dst[5] = byte(ns >> 40)
}

// KeyboardReport holds the 39-byte virtual keyboard HID report state.
// Set/clear usage bits via Press/Release then call Flush to send.
type KeyboardReport struct {
	svc    *UniversalService
	svcID  uint64
	bitmap [30]byte
}

// NewKeyboardReport creates a KeyboardReport bound to a Universal HID service.
// serviceID should be obtained from ListConnectedServices.
func NewKeyboardReport(svc *UniversalService, serviceID uint64) *KeyboardReport {
	return &KeyboardReport{svc: svc, svcID: serviceID}
}

// Press marks the HID usage as pressed in the bitmap.
func (r *KeyboardReport) Press(usage byte) {
	if usage >= 4 {
		byteIdx := 1 + (usage / 8)
		bitIdx := usage % 8
		if int(byteIdx) < len(r.bitmap) {
			r.bitmap[byteIdx] |= 1 << bitIdx
		}
	}
}

// Release clears the HID usage bit.
func (r *KeyboardReport) Release(usage byte) {
	if usage >= 4 {
		byteIdx := 1 + (usage / 8)
		bitIdx := usage % 8
		if int(byteIdx) < len(r.bitmap) {
			r.bitmap[byteIdx] &^= 1 << bitIdx
		}
	}
}

// Flush sends the current bitmap state as a keyboard HID report.
func (r *KeyboardReport) Flush() error {
	b := make([]byte, 39)
	b[0] = 0x01
	copy(b[1:31], r.bitmap[:])
	putMachTime(b[31:37])
	return r.svc.sendReport(r.svcID, b)
}

// TypeKey presses and releases a single key usage.
func (r *KeyboardReport) TypeKey(usage byte) error {
	r.Press(usage)
	if err := r.Flush(); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	r.Release(usage)
	return r.Flush()
}

// HID keyboard usage codes for Latin letters and digits.
const (
	KeyA = byte(0x04)
	KeyB = byte(0x05)
	KeyC = byte(0x06)
	KeyD = byte(0x07)
	KeyE = byte(0x08)
	KeyF = byte(0x09)
	KeyG = byte(0x0A)
	KeyH = byte(0x0B)
	KeyI = byte(0x0C)
	KeyJ = byte(0x0D)
	KeyK = byte(0x0E)
	KeyL = byte(0x0F)
	KeyM = byte(0x10)
	KeyN = byte(0x11)
	KeyO = byte(0x12)
	KeyP = byte(0x13)
	KeyQ = byte(0x14)
	KeyR = byte(0x15)
	KeyS = byte(0x16)
	KeyT = byte(0x17)
	KeyU = byte(0x18)
	KeyV = byte(0x19)
	KeyW = byte(0x1A)
	KeyX = byte(0x1B)
	KeyY = byte(0x1C)
	KeyZ = byte(0x1D)
	Key1 = byte(0x1E)
	Key2 = byte(0x1F)
	Key3 = byte(0x20)
	Key4 = byte(0x21)
	Key5 = byte(0x22)
	Key6 = byte(0x23)
	Key7 = byte(0x24)
	Key8 = byte(0x25)
	Key9 = byte(0x26)
	Key0 = byte(0x27)

	KeyReturn    = byte(0x28)
	KeyEscape    = byte(0x29)
	KeyBackspace = byte(0x2A)
	KeyTab       = byte(0x2B)
	KeySpace     = byte(0x2C)

	// Modifier usages (0xE0–0xE7)
	KeyLeftCtrl  = byte(0xE0)
	KeyLeftShift = byte(0xE1)
	KeyLeftAlt   = byte(0xE2)
	KeyLeftMeta  = byte(0xE3)
	KeyRightCtrl = byte(0xE4)
	KeyRightShift = byte(0xE5)
	KeyRightAlt  = byte(0xE6)
	KeyRightMeta = byte(0xE7)
)

// unused keeps the fmt import satisfied if no format calls remain.
var _ = fmt.Sprintf
