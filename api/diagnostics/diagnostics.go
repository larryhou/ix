package diagnostics

import (
	"fmt"
	"github.com/larryhou/ix/api/mux/plist"
	"net"
)

const (
	ServiceName    = `com.apple.mobile.diagnostics_relay`
	ServiceNameRSD = `com.apple.mobile.diagnostics_relay.shim.remote`
)

// Action constants for device power control.
const (
	RequestRestart  = `Restart`
	RequestShutdown = `Shutdown`
	RequestSleep    = `Sleep`
)

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// request sends a single-field Request command and returns the raw response map.
func (x *Service) request(req map[string]any) (map[string]any, error) {
	rsp := map[string]any{}
	if err := x.Get(req, &rsp); err != nil {
		return nil, err
	}
	if status, ok := rsp["Status"].(string); ok && status != "Success" {
		detail, _ := rsp["DetailedError"].(string)
		if detail == "" {
			detail = status
		}
		return nil, fmt.Errorf("diagnostics: %s", detail)
	}
	return rsp, nil
}

// Restart reboots the device immediately.
func (x *Service) Restart() error {
	_, err := x.request(map[string]any{"Request": RequestRestart})
	return err
}

// Shutdown powers off the device.
func (x *Service) Shutdown() error {
	_, err := x.request(map[string]any{"Request": RequestShutdown})
	return err
}

// Sleep puts the device display to sleep.
func (x *Service) Sleep() error {
	_, err := x.request(map[string]any{"Request": RequestSleep})
	return err
}

// MobileGestalt queries one or more MobileGestalt keys from the device.
// Returns a map of key → value. On iOS 17.4+ the service may return
// MobileGestaltDeprecated; this is surfaced as an error.
func (x *Service) MobileGestalt(keys ...string) (map[string]any, error) {
	rsp, err := x.request(map[string]any{
		"Request":          "MobileGestalt",
		"MobileGestaltKeys": keys,
	})
	if err != nil {
		return nil, err
	}
	diag, ok := rsp["Diagnostics"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("diagnostics: missing Diagnostics field")
	}
	mg, ok := diag["MobileGestalt"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("diagnostics: missing MobileGestalt field")
	}
	if status, ok := mg["Status"].(string); ok && status == "MobileGestaltDeprecated" {
		return nil, fmt.Errorf("diagnostics: MobileGestalt deprecated on this iOS version")
	}
	return mg, nil
}

// IORegistryEntry queries a single IORegistry entry by class or entry name.
// plane, entryName, entryClass are all optional (pass empty string to omit).
func (x *Service) IORegistryEntry(plane, entryName, entryClass string) (map[string]any, error) {
	req := map[string]any{"Request": "IORegistry"}
	if plane != "" {
		req["CurrentPlane"] = plane
	}
	if entryName != "" {
		req["EntryName"] = entryName
	}
	if entryClass != "" {
		req["EntryClass"] = entryClass
	}

	rsp, err := x.request(req)
	if err != nil {
		return nil, err
	}
	diag, ok := rsp["Diagnostics"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("diagnostics: missing Diagnostics field")
	}
	entry, ok := diag["IORegistry"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("diagnostics: missing IORegistry field")
	}
	return entry, nil
}

// All queries all available diagnostics information from the device.
func (x *Service) All() (map[string]any, error) {
	rsp, err := x.request(map[string]any{"Request": "All"})
	if err != nil {
		return nil, err
	}
	diag, ok := rsp["Diagnostics"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("diagnostics: missing Diagnostics field")
	}
	return diag, nil
}
