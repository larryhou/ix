// Package screencapture implements the com.apple.coredevice.screencaptureservice RSD service.
package screencapture

import (
	"fmt"
	"github.com/larryhou/ix/api/coredevice"
	"github.com/larryhou/ix/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.screencaptureservice`

const (
	featureCaptureScreenshot = `com.apple.coredevice.feature.capturescreenshot`
	actionCaptureScreenshot  = `com.apple.coredevice.action.capturescreenshot`
)

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// Screenshot captures a PNG screenshot from the device.
// displayUniqueID selects which display to capture; pass an empty string to use the default.
// Returns the raw PNG bytes and the display unique ID that was captured.
func (s *Service) Screenshot(displayUniqueID string) ([]byte, string, error) {
	input := map[string]any{
		"requestedFormat": "png",
	}
	if displayUniqueID != "" {
		input["displayUniqueID"] = displayUniqueID
	} else {
		input["displayUniqueID"] = nil
	}

	out, err := s.Invoke(featureCaptureScreenshot, actionCaptureScreenshot, input)
	if err != nil {
		return nil, "", err
	}

	m, ok := out.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("screencapture: unexpected output type %T", out)
	}

	imgData, _ := m["image"].([]byte)
	displayID, _ := m["displayUniqueID"].(string)
	return imgData, displayID, nil
}
