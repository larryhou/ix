package remotepair

const (
	TypeMethod           = 0x00
	TypeIdentifier       = 0x01
	TypeSalt             = 0x02
	TypePublicKey        = 0x03
	TypeProof            = 0x04
	TypeEncryptedData    = 0x05
	TypeState            = 0x06
	TypeError            = 0x07
	TypeRetryDelay       = 0x08
	TypeCertificate      = 0x09
	TypeSignature        = 0x0a
	TypePermissions      = 0x0b
	TypeFragmentData     = 0x0c
	TypeFragmentLast     = 0x0d
	TypeSessionId        = 0x0e
	TypeTTL              = 0x0f
	TypeExtraData        = 0x10
	TypeInfo             = 0x11
	TypeACL              = 0x12
	TypeFlags            = 0x13
	TypeValidationData   = 0x14
	TypeMfiAuthToken     = 0x15
	TypeMfiProductType   = 0x16
	TypeSerialNumber     = 0x17
	TypeMfiAuthTokenUuid = 0x18
	TypeAppFlags         = 0x19
	TypeOwnershipProof   = 0x1a
	TypeSetupCodeType    = 0x1b
	TypeProductionData   = 0x1c
	TypeAppInfo          = 0x1d
	TypeSeparator        = 0xff
)

type PairingTLV struct {
	Type byte
	Data []byte
}

type DeviceInfo struct {
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

type Descriptor struct {
	DeviceOptions                       *DeviceOptions `json:"deviceOptions"`
	MinimumSupportedWireProtocolVersion int            `json:"minimumSupportedWireProtocolVersion"`
	PeerDeviceInfo                      *DeviceInfo    `json:"peerDeviceInfo"`
	WireProtocolVersion                 int            `json:"wireProtocolVersion"`
}
