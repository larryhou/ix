// Package location implements the com.apple.coredevice.locationservice RSD service.
// It allows simulating a GPS location on the device via the CoreDevice protocol.
package location

import (
	"fmt"
	"github.com/larryhou/ix/api/coredevice"
	"github.com/larryhou/ix/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.locationservice`

const (
	featureSimulateLocation         = `com.apple.coredevice.feature.simulatelocation`
	actionAvailableScenarios        = `com.apple.coredevice.action.availablelocationscenarios`
	actionSetSimulatedLocation      = `com.apple.coredevice.action.setsimulatedlocation`
	actionClearSimulatedLocation    = `com.apple.coredevice.action.clearsimulatedlocation`
)

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// AvailableScenarios returns the location simulation scenarios supported by the device.
func (s *Service) AvailableScenarios() (any, error) {
	return s.Invoke(featureSimulateLocation, actionAvailableScenarios, map[string]any{})
}

// SetLocation simulates a GPS position at the given latitude/longitude.
func (s *Service) SetLocation(latitude, longitude float64) error {
	_, err := s.Invoke(featureSimulateLocation, actionSetSimulatedLocation, map[string]any{
		"location": map[string]any{
			"latitude":  latitude,
			"longitude": longitude,
		},
	})
	return err
}

// ClearLocation stops location simulation and reverts to real GPS data.
func (s *Service) ClearLocation() error {
	out, err := s.Invoke(featureSimulateLocation, actionClearSimulatedLocation, map[string]any{})
	if err != nil {
		return err
	}
	_ = out
	return nil
}

// asMap is a helper that asserts the output is a map.
func asMap(out any, context string) (map[string]any, error) {
	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: unexpected output type %T", context, out)
	}
	return m, nil
}
