package mounter

import (
	"fmt"
	"github.com/larryhou/ix/api/mux/plist"
	"io"
	"net"
)

const (
	ServiceName    = `com.apple.mobile.mobile_image_mounter`
	ServiceNameRSD = `com.apple.mobile.mobile_image_mounter.shim.remote`
)

// Image type strings used in mount/lookup requests.
const (
	ImageTypeDeveloper   = `Developer`
	ImageTypePersonalized = `Personalized`
)

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// checkResponse inspects the Status field and returns an error when it is not "Complete" or empty.
func checkResponse(rsp map[string]any) error {
	status, _ := rsp["Status"].(string)
	if status != "" && status != "Complete" && status != "ReceiveBytesAck" {
		detail, _ := rsp["DetailedError"].(string)
		if detail == "" {
			detail = status
		}
		return fmt.Errorf("mounter: %s", detail)
	}
	return nil
}

// CopyDevices returns the list of currently mounted developer images.
func (x *Service) CopyDevices() ([]map[string]any, error) {
	rsp := map[string]any{}
	if err := x.Get(map[string]any{"Command": "CopyDevices"}, &rsp); err != nil {
		return nil, err
	}
	raw, ok := rsp["EntryList"].([]any)
	if !ok {
		return nil, nil
	}
	entries := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			entries = append(entries, m)
		}
	}
	return entries, nil
}

// LookupImage checks whether a developer image of the given type is already mounted.
func (x *Service) LookupImage(imageType string) (present bool, signature []byte, err error) {
	rsp := map[string]any{}
	if err = x.Get(map[string]any{
		"Command":   "LookupImage",
		"ImageType": imageType,
	}, &rsp); err != nil {
		return
	}
	present, _ = rsp["ImagePresent"].(bool)
	switch v := rsp["ImageSignature"].(type) {
	case []byte:
		signature = v
	case []any:
		// Some devices return an array of signature blobs; return the first one.
		if len(v) > 0 {
			signature, _ = v[0].([]byte)
		}
	}
	return
}

// UploadImage transfers the raw DMG bytes to the device using the three-step
// ReceiveBytes protocol and then issues a MountImage command.
//
//   - imageType: ImageTypeDeveloper or ImageTypePersonalized
//   - imageSize: exact byte length of the DMG
//   - signature: SHA-1 (Developer) or SHA-384 (Personalized) signature blob
//   - r:         source of the raw DMG bytes (exactly imageSize bytes will be read)
func (x *Service) UploadImage(imageType string, imageSize int64, signature []byte, r io.Reader) error {
	// Step 1: announce upload.
	req := map[string]any{
		"Command":        "ReceiveBytes",
		"ImageType":      imageType,
		"ImageSize":      imageSize,
		"ImageSignature": signature,
	}
	rsp := map[string]any{}
	if err := x.Get(req, &rsp); err != nil {
		return err
	}
	if err := checkResponse(rsp); err != nil {
		return err
	}
	if status, _ := rsp["Status"].(string); status != "ReceiveBytesAck" {
		return fmt.Errorf("mounter: expected ReceiveBytesAck, got %q", status)
	}

	// Step 2: stream raw DMG bytes — no length framing.
	if _, err := io.Copy(x.Conn, r); err != nil {
		return fmt.Errorf("mounter: upload failed: %w", err)
	}

	// Step 3: read completion response.
	rsp = map[string]any{}
	if err := x.Connection.Recv(&rsp); err != nil {
		return err
	}
	return checkResponse(rsp)
}

// MountImage mounts a previously uploaded developer image.
func (x *Service) MountImage(imageType string, signature []byte) error {
	rsp := map[string]any{}
	err := x.Get(map[string]any{
		"Command":        "MountImage",
		"ImageType":      imageType,
		"ImageSignature": signature,
	}, &rsp)
	if err != nil {
		return err
	}
	return checkResponse(rsp)
}

// MountPersonalizedImage mounts a personalized (cryptex) developer image.
func (x *Service) MountPersonalizedImage(signature, trustCache, infoPlist []byte) error {
	rsp := map[string]any{}
	err := x.Get(map[string]any{
		"Command":        "MountImage",
		"ImageType":      ImageTypePersonalized,
		"ImageSignature": signature,
		"ImageTrustCache": trustCache,
		"ImageInfoPlist":  infoPlist,
	}, &rsp)
	if err != nil {
		return err
	}
	return checkResponse(rsp)
}

// UnmountImage unmounts the developer image at the given mount path (e.g. "/Developer").
func (x *Service) UnmountImage(mountPath string) error {
	rsp := map[string]any{}
	err := x.Get(map[string]any{
		"Command":    "UnmountImage",
		"MountPath":  mountPath,
	}, &rsp)
	if err != nil {
		return err
	}
	if errMsg, ok := rsp["Error"].(string); ok && errMsg != "" {
		detail, _ := rsp["DetailedError"].(string)
		if detail != "" {
			return fmt.Errorf("mounter: %s: %s", errMsg, detail)
		}
		return fmt.Errorf("mounter: %s", errMsg)
	}
	return nil
}

// QueryDeveloperModeStatus returns whether Developer Mode is currently enabled.
func (x *Service) QueryDeveloperModeStatus() (bool, error) {
	rsp := map[string]any{}
	if err := x.Get(map[string]any{"Command": "QueryDeveloperModeStatus"}, &rsp); err != nil {
		return false, err
	}
	enabled, _ := rsp["DeveloperModeStatus"].(bool)
	return enabled, nil
}

// QueryNonce returns the personalization nonce for the given image type.
// imageType may be empty to use the device default.
func (x *Service) QueryNonce(imageType string) ([]byte, error) {
	req := map[string]any{"Command": "QueryNonce"}
	if imageType != "" {
		req["PersonalizedImageType"] = imageType
	}
	rsp := map[string]any{}
	if err := x.Get(req, &rsp); err != nil {
		return nil, err
	}
	nonce, _ := rsp["PersonalizationNonce"].([]byte)
	return nonce, nil
}

// QueryPersonalizationIdentifiers returns the board/chip identifiers needed
// to request a personalized manifest from Apple's TSS server.
func (x *Service) QueryPersonalizationIdentifiers(imageType string) (map[string]any, error) {
	req := map[string]any{"Command": "QueryPersonalizationIdentifiers"}
	if imageType != "" {
		req["PersonalizedImageType"] = imageType
	}
	rsp := map[string]any{}
	if err := x.Get(req, &rsp); err != nil {
		return nil, err
	}
	ids, ok := rsp["PersonalizationIdentifiers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("mounter: missing PersonalizationIdentifiers")
	}
	return ids, nil
}

// RollPersonalizationNonce asks the device to roll its personalization nonce.
// The device may close the connection after this call.
func (x *Service) RollPersonalizationNonce() error {
	return x.Send(map[string]any{"Command": "RollPersonalizationNonce"})
}

// RollCryptexNonce asks the device to roll its cryptex nonce.
// The device may close the connection after this call.
func (x *Service) RollCryptexNonce() error {
	return x.Send(map[string]any{"Command": "RollCryptexNonce"})
}
