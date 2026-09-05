package springboard

import (
	"fmt"
	"github.com/larryhou/ix/api/j3/plist"
	"net"
)

const (
	ServiceName    = `com.apple.springboardservices`
	ServiceNameRSD = `com.apple.springboardservices.shim.remote`
)

// Orientation constants returned by GetInterfaceOrientation.
const (
	OrientationPortrait           = 1
	OrientationPortraitUpsideDown = 2
	OrientationLandscapeRight     = 3 // home button to the right
	OrientationLandscapeLeft      = 4 // home button to the left
)

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// cmd sends a springboard command and returns the raw response map.
// Note: SpringBoard uses lowercase "command" key (unlike most other services).
func (x *Service) cmd(req map[string]any) (map[string]any, error) {
	rsp := map[string]any{}
	if err := x.Get(req, &rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

// GetIconPNGData returns the PNG icon data for the app with the given bundle identifier.
func (x *Service) GetIconPNGData(bundleID string) ([]byte, error) {
	rsp, err := x.cmd(map[string]any{
		"command":  "getIconPNGData",
		"bundleId": bundleID,
	})
	if err != nil {
		return nil, err
	}
	data, ok := rsp["pngData"].([]byte)
	if !ok {
		return nil, fmt.Errorf("springboard: missing pngData in response")
	}
	return data, nil
}

// GetInterfaceOrientation returns the current UI orientation.
// Use the Orientation* constants to interpret the value.
func (x *Service) GetInterfaceOrientation() (int, error) {
	rsp, err := x.cmd(map[string]any{"command": "getInterfaceOrientation"})
	if err != nil {
		return 0, err
	}
	// The value may arrive as uint64 or int64 depending on plist encoding.
	switch v := rsp["interfaceOrientation"].(type) {
	case uint64:
		return int(v), nil
	case int64:
		return int(v), nil
	}
	return 0, fmt.Errorf("springboard: missing or invalid interfaceOrientation")
}

// GetHomeScreenWallpaperPNGData returns the PNG data of the home screen wallpaper.
func (x *Service) GetHomeScreenWallpaperPNGData() ([]byte, error) {
	rsp, err := x.cmd(map[string]any{"command": "getHomeScreenWallpaperPNGData"})
	if err != nil {
		return nil, err
	}
	data, ok := rsp["pngData"].([]byte)
	if !ok {
		return nil, fmt.Errorf("springboard: missing pngData in response")
	}
	return data, nil
}

// GetIconState returns the current home screen icon layout as a nested array.
// The outer array represents pages; each page is an array of icon rows or folders.
func (x *Service) GetIconState() (any, error) {
	rsp, err := x.cmd(map[string]any{
		"command":       "getIconState",
		"formatVersion": "2",
	})
	if err != nil {
		return nil, err
	}
	return rsp, nil
}

// SetIconState applies a new icon layout to the home screen.
// layout should be the value previously obtained from GetIconState and optionally modified.
func (x *Service) SetIconState(layout any) error {
	_, err := x.cmd(map[string]any{
		"command":   "setIconState",
		"iconState": layout,
	})
	return err
}
