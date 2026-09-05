// Package deviceinfo implements the com.apple.coredevice.deviceinfo RSD service.
// It provides device information, display info, MobileGestalt queries, and lock state.
package deviceinfo

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/coredevice"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.deviceinfo`

const (
	featureGetDeviceInfo    = `com.apple.coredevice.feature.getdeviceinfo`
	featureGetDisplayInfo   = `com.apple.coredevice.feature.getdisplayinfo`
	featureQueryGestalt     = `com.apple.coredevice.feature.querymobilegestalt`
	featureGetLockState     = `com.apple.coredevice.feature.getlockstate`
)

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// GetDeviceInfo returns a dict of general device information (product type, OS version, etc.).
func (s *Service) GetDeviceInfo() (map[string]any, error) {
	out, err := s.Invoke(featureGetDeviceInfo, "", map[string]any{})
	if err != nil {
		return nil, err
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deviceinfo: unexpected output type %T", out)
	}
	return m, nil
}

// GetDisplayInfo returns display geometry and scale factor information.
func (s *Service) GetDisplayInfo() (map[string]any, error) {
	out, err := s.Invoke(featureGetDisplayInfo, "", map[string]any{})
	if err != nil {
		return nil, err
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deviceinfo: unexpected output type %T", out)
	}
	return m, nil
}

// QueryMobileGestalt queries one or more MobileGestalt keys via CoreDevice.
// Returns a map of key → value.
func (s *Service) QueryMobileGestalt(keys ...string) (map[string]any, error) {
	out, err := s.Invoke(featureQueryGestalt, "", map[string]any{
		"keys": keys,
	})
	if err != nil {
		return nil, err
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deviceinfo: unexpected output type %T", out)
	}
	return m, nil
}

// GetLockState returns passcode and lock state information.
func (s *Service) GetLockState() (map[string]any, error) {
	out, err := s.Invoke(featureGetLockState, "", map[string]any{})
	if err != nil {
		return nil, err
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deviceinfo: unexpected output type %T", out)
	}
	return m, nil
}
