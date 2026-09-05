// Package orientation implements the com.apple.coredevice.devicecontrol RSD service.
// It allows rotating the device screen programmatically.
package orientation

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/coredevice"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.devicecontrol`

const (
	featureOrientation = `com.apple.coredevice.feature.remote.devicecontrol.orientation`
)

// Rotation direction constants.
const (
	// RotateLeft rotates the device 90° counter-clockwise.
	RotateLeft = "left"
	// RotateRight rotates the device 90° clockwise.
	RotateRight = "right"
)

// OrientationState holds the device orientation reported after a rotate call.
type OrientationState struct {
	CurrentDeviceOrientation        string
	CurrentDeviceNonFlatOrientation string
	CurrentDeviceOrientationLocked  bool
}

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// Rotate rotates the device in the given direction ("left" or "right").
// Four consecutive RotateLeft calls cycle through all four orientations.
// Returns the resulting orientation state.
func (s *Service) Rotate(direction string) (*OrientationState, error) {
	rsp, err := s.SendRecv(map[string]any{
		"featureIdentifier": featureOrientation,
		"messageType":       "OrientationRequest",
		"payload": map[string]any{
			"rotate": map[string]any{"_0": direction},
		},
	})
	if err != nil {
		return nil, err
	}

	m, ok := rsp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("orientation: unexpected response type %T", rsp)
	}

	state := &OrientationState{}
	state.CurrentDeviceOrientation, _ = m["currentDeviceOrientation"].(string)
	state.CurrentDeviceNonFlatOrientation, _ = m["currentDeviceNonFlatOrientation"].(string)
	state.CurrentDeviceOrientationLocked, _ = m["currentDeviceOrientationLocked"].(bool)
	return state, nil
}

// RotateLeft rotates the screen 90° counter-clockwise.
func (s *Service) RotateLeft() (*OrientationState, error) {
	return s.Rotate(RotateLeft)
}

// RotateRight rotates the screen 90° clockwise.
func (s *Service) RotateRight() (*OrientationState, error) {
	return s.Rotate(RotateRight)
}
