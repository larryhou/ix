package remotepair

type PeerDeviceInfo struct {
	DeviceKVSData                  []byte `json:"deviceKVSData"`
	DeviceKVSIncludesSensitiveInfo bool   `json:"deviceKVSIncludesSensitiveInfo"`
	Ecid                           int64  `json:"ecid"`
	Identifier                     string `json:"identifier"`
	Model                          string `json:"model"`
	Name                           string `json:"name"`
	Udid                           string `json:"udid"`
}

type DeviceOptions struct {
	AllowsIncomingTunnelConnections          bool `json:"allowsIncomingTunnelConnections"`
	AllowsPairSetup                          bool `json:"allowsPairSetup"`
	AllowsPinlessPairing                     bool `json:"allowsPinlessPairing"`
	AllowsPromptlessAutomationPairingUpgrade bool `json:"allowsPromptlessAutomationPairingUpgrade"`
	AllowsSharingSensitiveInfo               bool `json:"allowsSharingSensitiveInfo"`
}

type Handshake struct {
	DeviceOptions                       *DeviceOptions  `json:"deviceOptions"`
	MinimumSupportedWireProtocolVersion int             `json:"minimumSupportedWireProtocolVersion"`
	PeerDeviceInfo                      *PeerDeviceInfo `json:"peerDeviceInfo"`
	WireProtocolVersion                 int             `json:"wireProtocolVersion"`
}
