// Package coredevice provides the shared base layer for CoreDevice RSD/XPC services.
// All iOS 17+ device control services (device info, HID, screen capture, pasteboard, etc.)
// communicate via RemoteXPC over an RSD tunnel. This package implements the
// standard CoreDevice request/response envelope that wraps every feature invocation.
package coredevice

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
	"github.com/google/uuid"
)

// Protocol version advertised in every CoreDevice request envelope.
const DDIProtocolVersion = int64(2)

// coreDeviceVersion is the static version dict included in every request.
var coreDeviceVersion = map[string]any{
	"components":             []any{uint64(629), uint64(3)},
	"originalComponentsCount": int64(2),
	"stringValue":            "629.3",
}

// Service wraps a RemoteXpcConnection and provides the CoreDevice invoke primitives.
type Service struct {
	conn *xpc.RemoteXpcConnection
}

// NewService creates a Service from an already-connected RemoteXpcConnection.
func NewService(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{conn: conn}
}

// Close closes the underlying XPC connection.
func (s *Service) Close() error {
	return s.conn.Close()
}

// Invoke sends a standard CoreDevice feature request and returns the
// "CoreDevice.output" field from the response.
//
// featureIdentifier identifies the feature, e.g.
// "com.apple.coredevice.feature.getdeviceinfo".
// actionIdentifier may be empty; when non-empty it is included as
// "CoreDevice.actionIdentifier" and "CoreDevice.featureIdentifier" is omitted
// (used by ConfigurationService).
// input is the feature-specific parameter dict sent as "CoreDevice.input".
func (s *Service) Invoke(featureIdentifier, actionIdentifier string, input map[string]any) (any, error) {
	req := map[string]any{
		"CoreDevice.CoreDeviceDDIProtocolVersion": DDIProtocolVersion,
		"CoreDevice.coreDeviceVersion":           coreDeviceVersion,
		"CoreDevice.deviceIdentifier":            uuid.New().String(),
		"CoreDevice.invocationIdentifier":        uuid.New().String(),
		"CoreDevice.action":                      map[string]any{},
		"CoreDevice.input":                       input,
	}

	if actionIdentifier != "" {
		req["CoreDevice.actionIdentifier"] = actionIdentifier
	}
	if featureIdentifier != "" {
		req["CoreDevice.featureIdentifier"] = featureIdentifier
	}

	if err := s.conn.Send(req); err != nil {
		return nil, err
	}

	rsp, err := s.conn.Recv()
	if err != nil {
		return nil, err
	}

	return extractOutput(rsp)
}

// extractOutput pulls "CoreDevice.output" from the response dict.
func extractOutput(rsp any) (any, error) {
	m, ok := rsp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("coredevice: unexpected response type %T", rsp)
	}
	if errVal, ok := m["CoreDevice.error"]; ok {
		return nil, fmt.Errorf("coredevice: %v", errVal)
	}
	return m["CoreDevice.output"], nil
}

// Send sends a raw XPC dict to the service without the CoreDevice envelope.
// Used by services that speak a direct message protocol (Pasteboard, Orientation, HID).
func (s *Service) Send(msg any) error {
	return s.conn.Send(msg)
}

// Recv reads the next XPC message from the service.
func (s *Service) Recv() (any, error) {
	return s.conn.Recv()
}

// SendRecv sends a message and reads one response.
func (s *Service) SendRecv(msg any) (any, error) {
	if err := s.conn.Send(msg); err != nil {
		return nil, err
	}
	return s.conn.Recv()
}
