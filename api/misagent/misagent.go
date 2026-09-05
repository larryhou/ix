package misagent

import (
	"fmt"
	"github.com/larryhou/j3idevice/api/j3/plist"
	"net"
)

const (
	ServiceName    = `com.apple.misagent`
	ServiceNameRSD = `com.apple.misagent.shim.remote`
)

const profileType = `Provisioning`

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// checkStatus verifies the Status field in the response is 0 (success).
func checkStatus(rsp map[string]any) error {
	status, ok := rsp["Status"]
	if !ok {
		return nil
	}
	switch v := status.(type) {
	case uint64:
		if v != 0 {
			return fmt.Errorf("misagent: status %d", v)
		}
	case int64:
		if v != 0 {
			return fmt.Errorf("misagent: status %d", v)
		}
	}
	return nil
}

// Install installs a provisioning profile onto the device.
// profile should be the raw .mobileprovision file bytes.
func (x *Service) Install(profile []byte) error {
	rsp := map[string]any{}
	err := x.Get(map[string]any{
		"MessageType": "Install",
		"Profile":     profile,
		"ProfileType": profileType,
	}, &rsp)
	if err != nil {
		return err
	}
	return checkStatus(rsp)
}

// Remove removes the provisioning profile identified by its UUID.
func (x *Service) Remove(profileID string) error {
	rsp := map[string]any{}
	err := x.Get(map[string]any{
		"MessageType": "Remove",
		"ProfileID":   profileID,
		"ProfileType": profileType,
	}, &rsp)
	if err != nil {
		return err
	}
	return checkStatus(rsp)
}

// CopyAll returns the raw bytes of every provisioning profile installed on the device.
// Each element is a DER/CMS-encoded .mobileprovision blob.
func (x *Service) CopyAll() ([][]byte, error) {
	rsp := map[string]any{}
	err := x.Get(map[string]any{
		"MessageType": "CopyAll",
		"ProfileType": profileType,
	}, &rsp)
	if err != nil {
		return nil, err
	}
	if err = checkStatus(rsp); err != nil {
		return nil, err
	}

	payload, ok := rsp["Payload"]
	if !ok {
		return nil, nil
	}

	// The Payload field is a plist array of raw data blobs.
	items, ok := payload.([]any)
	if !ok {
		return nil, fmt.Errorf("misagent: unexpected Payload type %T", payload)
	}

	profiles := make([][]byte, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case []byte:
			profiles = append(profiles, v)
		default:
			return nil, fmt.Errorf("misagent: unexpected profile item type %T", item)
		}
	}
	return profiles, nil
}
