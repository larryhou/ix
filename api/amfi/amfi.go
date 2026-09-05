package amfi

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/j3/plist"
	"net"
)

const (
	ServiceName = `com.apple.amfi.lockdown`
)

// Action values for the AMFI Developer Mode control protocol.
const (
	// ActionReveal reveals the Developer Mode toggle in Settings without enabling it.
	ActionReveal = 0
	// ActionEnable enables Developer Mode; the device will reboot after success.
	ActionEnable = 1
	// ActionAccept accepts the post-reboot Developer Mode confirmation prompt.
	ActionAccept = 2
)

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// action sends a single action request and checks the response.
// Each AMFI call is expected to open a fresh connection, send one message, then close.
func (x *Service) action(code int) error {
	rsp := map[string]any{}
	if err := x.Get(map[string]any{"action": code}, &rsp); err != nil {
		return err
	}
	if success, ok := rsp["success"].(bool); ok && success {
		return nil
	}
	if errMsg, ok := rsp["Error"].(string); ok && errMsg != "" {
		return fmt.Errorf("amfi: %s", errMsg)
	}
	return fmt.Errorf("amfi: action %d failed: %v", code, rsp)
}

// Reveal makes the Developer Mode toggle visible in the Settings app.
func (x *Service) Reveal() error {
	return x.action(ActionReveal)
}

// Enable turns on Developer Mode. The device will reboot after this call succeeds.
// After the reboot reconnect and call Accept to complete the flow.
func (x *Service) Enable() error {
	return x.action(ActionEnable)
}

// Accept confirms the post-reboot Developer Mode prompt.
// Call this after the device has rebooted following a successful Enable call.
func (x *Service) Accept() error {
	return x.action(ActionAccept)
}
