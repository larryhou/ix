package rsd

type DeviceProperties struct {
	AppleInternal                     bool   `json:"AppleInternal"`
	BoardId                           int    `json:"BoardId"`
	BootSessionUUID                   string `json:"BootSessionUUID"`
	BuildVersion                      string `json:"BuildVersion"`
	CPUArchitecture                   string `json:"CPUArchitecture"`
	CertificateProductionStatus       bool   `json:"CertificateProductionStatus"`
	CertificateSecurityMode           bool   `json:"CertificateSecurityMode"`
	ChipID                            int    `json:"ChipID"`
	DeviceClass                       string `json:"DeviceClass"`
	DeviceColor                       string `json:"DeviceColor"`
	DeviceEnclosureColor              string `json:"DeviceEnclosureColor"`
	DeviceSupportsLockdown            bool   `json:"DeviceSupportsLockdown"`
	EffectiveProductionStatusAp       bool   `json:"EffectiveProductionStatusAp"`
	EffectiveProductionStatusSEP      bool   `json:"EffectiveProductionStatusSEP"`
	EffectiveSecurityModeAp           bool   `json:"EffectiveSecurityModeAp"`
	EffectiveSecurityModeSEP          bool   `json:"EffectiveSecurityModeSEP"`
	EthernetMacAddress                string `json:"EthernetMacAddress"`
	HWModel                           string `json:"HWModel"`
	HardwarePlatform                  string `json:"HardwarePlatform"`
	HasSEP                            bool   `json:"HasSEP"`
	HumanReadableProductVersionString string `json:"HumanReadableProductVersionString"`
	Image4CryptoHashMethod            string `json:"Image4CryptoHashMethod"`
	Image4Supported                   bool   `json:"Image4Supported"`
	IsUIBuild                         bool   `json:"IsUIBuild"`
	IsVirtualDevice                   bool   `json:"IsVirtualDevice"`
	MobileDeviceMinimumVersion        string `json:"MobileDeviceMinimumVersion"`
	ModelNumber                       string `json:"ModelNumber"`
	OSInstallEnvironment              bool   `json:"OSInstallEnvironment"`
	OSVersion                         string `json:"OSVersion"`
	ProductName                       string `json:"ProductName"`
	ProductType                       string `json:"ProductType"`
	RegionCode                        string `json:"RegionCode"`
	RegionInfo                        string `json:"RegionInfo"`
	RemoteXPCVersionFlags             int64  `json:"RemoteXPCVersionFlags"`
	RestoreLongVersion                string `json:"RestoreLongVersion"`
	SecurityDomain                    int    `json:"SecurityDomain"`
	SensitivePropertiesVisible        bool   `json:"SensitivePropertiesVisible"`
	SerialNumber                      string `json:"SerialNumber"`
	SigningFuse                       bool   `json:"SigningFuse"`
	StoreDemoMode                     bool   `json:"StoreDemoMode"`
	SupplementalBuildVersion          string `json:"SupplementalBuildVersion"`
	ThinningProductType               string `json:"ThinningProductType"`
	UniqueChipID                      int64  `json:"UniqueChipID"`
	UniqueDeviceID                    string `json:"UniqueDeviceID"`
}

type ServiceProperties struct {
	ServiceVersion int  `json:"ServiceVersion"`
	UsesRemoteXPC  bool `json:"UsesRemoteXPC"`
}

type RemoteService struct {
	Entitlement string             `json:"Entitlement"`
	Port        string             `json:"Port"`
	Properties  *ServiceProperties `json:"Properties"`
}

type Handshake struct {
	MessageType              string                    `json:"MessageType"`
	MessagingProtocolVersion int                       `json:"MessagingProtocolVersion"`
	Properties               *DeviceProperties         `json:"Properties"`
	Services                 map[string]*RemoteService `json:"Services"`
	UUID                     string                    `json:"UUID"`
}

type T struct {
	MangledTypeName string `json:"mangledTypeName"`
	Value           struct {
		Message struct {
			Plain struct {
				Field1 struct {
					Response struct {
						Field1 struct {
							Handshake struct {
								Field1 struct {
									DeviceOptions struct {
										AllowsIncomingTunnelConnections          bool `json:"allowsIncomingTunnelConnections"`
										AllowsPairSetup                          bool `json:"allowsPairSetup"`
										AllowsPinlessPairing                     bool `json:"allowsPinlessPairing"`
										AllowsPromptlessAutomationPairingUpgrade bool `json:"allowsPromptlessAutomationPairingUpgrade"`
										AllowsSharingSensitiveInfo               bool `json:"allowsSharingSensitiveInfo"`
									} `json:"deviceOptions"`
									MinimumSupportedWireProtocolVersion int `json:"minimumSupportedWireProtocolVersion"`
									PeerDeviceInfo                      struct {
										DeviceKVSData                  string `json:"deviceKVSData"`
										DeviceKVSIncludesSensitiveInfo bool   `json:"deviceKVSIncludesSensitiveInfo"`
										Ecid                           int64  `json:"ecid"`
										Identifier                     string `json:"identifier"`
										Model                          string `json:"model"`
										Name                           string `json:"name"`
										Udid                           string `json:"udid"`
									} `json:"peerDeviceInfo"`
									WireProtocolVersion int `json:"wireProtocolVersion"`
								} `json:"_0"`
							} `json:"handshake"`
						} `json:"_1"`
						ForRequestIdentifier int `json:"forRequestIdentifier"`
					} `json:"response"`
				} `json:"_0"`
			} `json:"plain"`
		} `json:"message"`
		OriginatedBy   string `json:"originatedBy"`
		SequenceNumber int    `json:"sequenceNumber"`
	} `json:"value"`
}
