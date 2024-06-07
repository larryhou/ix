package lockdown

import (
	"bytes"
	"github.com/larryhou/gomobiledevice3/api/usbmux"
	"howett.net/plist"
)

type ReadPairRecordRequest struct {
	ClientVersionString string `plist:"ClientVersionString"`
	MessageType         string `plist:"MessageType"`
	PairRecordID        string `plist:"PairRecordID"`
	ProgName            string `plist:"ProgName"`
	KLibUSBMuxVersion   int    `plist:"kLibUSBMuxVersion"`
}

type ReadPairRecordResponse struct {
	usbmux.Response
	PairRecordData []byte `plist:"PairRecordData"`
}

func (x *ReadPairRecordResponse) PairRecord() (*PairRecord, error) {
	record := &PairRecord{}
	return record, plist.NewDecoder(bytes.NewReader(x.PairRecordData)).Decode(record)
}

type PairRecord struct {
	DeviceCertificate []byte `plist:"DeviceCertificate"`
	DevicePublicKey   []byte `plist:"DevicePublicKey"`
	EscrowBag         []byte `plist:"EscrowBag,omitempty"`
	HostCertificate   []byte `plist:"HostCertificate"`
	HostID            string `plist:"HostID"`
	HostPrivateKey    []byte `plist:"HostPrivateKey,omitempty"`
	RootCertificate   []byte `plist:"RootCertificate"`
	RootPrivateKey    []byte `plist:"RootPrivateKey"`
	SystemBUID        string `plist:"SystemBUID"`
	WiFiMACAddress    string `plist:"WiFiMACAddress"`
}

type KeyHashInformation struct {
	AKeyStatus int    `plist:"AKeyStatus"`
	SKeyHash   []byte `plist:"SKeyHash"`
	SKeyStatus int    `plist:"SKeyStatus"`
}

type Lockdown struct {
	BasebandCertId                    int64               `plist:"BasebandCertId"`
	BasebandKeyHashInformation        *KeyHashInformation `plist:"BasebandKeyHashInformation"`
	BasebandSerialNumber              []byte              `plist:"BasebandSerialNumber"`
	BasebandVersion                   string              `plist:"BasebandVersion"`
	BoardId                           int                 `plist:"BoardId"`
	BuildVersion                      string              `plist:"BuildVersion"`
	CPUArchitecture                   string              `plist:"CPUArchitecture"`
	ChipID                            int                 `plist:"ChipID"`
	DeviceClass                       string              `plist:"DeviceClass"`
	DeviceColor                       string              `plist:"DeviceColor"`
	DeviceName                        string              `plist:"DeviceName"`
	DieID                             int64               `plist:"DieID"`
	HardwareModel                     string              `plist:"HardwareModel"`
	HasSiDP                           bool                `plist:"HasSiDP"`
	HumanReadableProductVersionString string              `plist:"HumanReadableProductVersionString"`
	PartitionType                     string              `plist:"PartitionType"`
	ProductName                       string              `plist:"ProductName"`
	ProductType                       string              `plist:"ProductType"`
	ProductVersion                    string              `plist:"ProductVersion"`
	ProductionSOC                     bool                `plist:"ProductionSOC"`
	ProtocolVersion                   string              `plist:"ProtocolVersion"`
	SupportedDeviceFamilies           []int               `plist:"SupportedDeviceFamilies"`
	TelephonyCapability               bool                `plist:"TelephonyCapability"`
	UniqueChipID                      int64               `plist:"UniqueChipID"`
	UniqueDeviceID                    string              `plist:"UniqueDeviceID"`
	WiFiAddress                       string              `plist:"WiFiAddress"`
}

type PairRequest struct {

}

type PairResponse struct {
	usbmux.Response

}

type UnpairRequest struct {

}

type UnpairResponse struct {
	usbmux.Response

}

type ResetPairRequest struct {

}

type ResetPairResponse struct {
	usbmux.Response

}

type CarrierBundleInfo struct {
	CFBundleIdentifier                    string `json:"CFBundleIdentifier"`
	CFBundleVersion                       string `json:"CFBundleVersion"`
	IntegratedCircuitCardIdentity         string `json:"IntegratedCircuitCardIdentity"`
	InternationalMobileSubscriberIdentity string `json:"InternationalMobileSubscriberIdentity"`
	MCC                                   string `json:"MCC"`
	MNC                                   string `json:"MNC"`
	Slot                                  string `json:"Slot"`
	KCTPostponementInfoAvailable          string `json:"kCTPostponementInfoAvailable"`
	GID1                                  string `json:"GID1,omitempty"`
	GID2                                  string `json:"GID2,omitempty"`
	SIMGID1                               string `json:"SIMGID1,omitempty"`
	SIMGID2                               string `json:"SIMGID2,omitempty"`
}

type NonVolatileRAM struct {
	StartupMute              string `json:"StartupMute"`
	AutoBoot                 string `json:"auto-boot"`
	BacklightLevel           string `json:"backlight-level"`
	BacklightNits            string `json:"backlight-nits"`
	Bootdelay                string `json:"bootdelay"`
	DisplayCrossbar0         []byte `json:"display-crossbar0"`
	FmAccountMasked          []byte `json:"fm-account-masked"`
	FmActivationLocked       string `json:"fm-activation-locked"`
	FmSpkeys                 []byte `json:"fm-spkeys"`
	FmSpstatus               string `json:"fm-spstatus"`
	OtaControllerVersion     string `json:"ota-controllerVersion"`
	OtaOriginalBaseOsVersion string `json:"ota-original-base-os-version"`
	UsbcVersionRid0          []byte `json:"usbc,version,rid0"`
}

type Descriptor struct {
	ActivationState                               string               `json:"ActivationState"`
	ActivationStateAcknowledged                   bool                 `json:"ActivationStateAcknowledged"`
	BasebandActivationTicketVersion               string               `json:"BasebandActivationTicketVersion"`
	BasebandCertId                                int64                `json:"BasebandCertId"`
	BasebandChipID                                int                  `json:"BasebandChipID"`
	BasebandKeyHashInformation                    *KeyHashInformation  `json:"BasebandKeyHashInformation"`
	BasebandMasterKeyHash                         string               `json:"BasebandMasterKeyHash"`
	BasebandRegionSKU                             []byte               `json:"BasebandRegionSKU"`
	BasebandSerialNumber                          []byte               `json:"BasebandSerialNumber"`
	BasebandStatus                                string               `json:"BasebandStatus"`
	BasebandVersion                               string               `json:"BasebandVersion"`
	BluetoothAddress                              string               `json:"BluetoothAddress"`
	BoardId                                       int                  `json:"BoardId"`
	BootSessionID                                 string               `json:"BootSessionID"`
	BrickState                                    bool                 `json:"BrickState"`
	BuildVersion                                  string               `json:"BuildVersion"`
	CPUArchitecture                               string               `json:"CPUArchitecture"`
	CarrierBundleInfoArray                        []*CarrierBundleInfo `json:"CarrierBundleInfoArray"`
	CertID                                        int64                `json:"CertID"`
	ChipID                                        int                  `json:"ChipID"`
	ChipSerialNo                                  []byte               `json:"ChipSerialNo"`
	DeviceClass                                   string               `json:"DeviceClass"`
	DeviceColor                                   string               `json:"DeviceColor"`
	DeviceName                                    string               `json:"DeviceName"`
	DieID                                         int64                `json:"DieID"`
	EthernetAddress                               string               `json:"EthernetAddress"`
	FirmwareVersion                               string               `json:"FirmwareVersion"`
	FusingStatus                                  int                  `json:"FusingStatus"`
	HardwareModel                                 string               `json:"HardwareModel"`
	HardwarePlatform                              string               `json:"HardwarePlatform"`
	HasSiDP                                       bool                 `json:"HasSiDP"`
	HostAttached                                  bool                 `json:"HostAttached"`
	HumanReadableProductVersionString             string               `json:"HumanReadableProductVersionString"`
	IntegratedCircuitCardIdentity                 string               `json:"IntegratedCircuitCardIdentity"`
	IntegratedCircuitCardIdentity2                string               `json:"IntegratedCircuitCardIdentity2"`
	InternationalMobileEquipmentIdentity          string               `json:"InternationalMobileEquipmentIdentity"`
	InternationalMobileEquipmentIdentity2         string               `json:"InternationalMobileEquipmentIdentity2"`
	InternationalMobileSubscriberIdentity         string               `json:"InternationalMobileSubscriberIdentity"`
	InternationalMobileSubscriberIdentity2        string               `json:"InternationalMobileSubscriberIdentity2"`
	InternationalMobileSubscriberIdentityOverride bool                 `json:"InternationalMobileSubscriberIdentityOverride"`
	MLBSerialNumber                               string               `json:"MLBSerialNumber"`
	MobileSubscriberCountryCode                   string               `json:"MobileSubscriberCountryCode"`
	MobileSubscriberNetworkCode                   string               `json:"MobileSubscriberNetworkCode"`
	ModelNumber                                   string               `json:"ModelNumber"`
	NonVolatileRAM                                *NonVolatileRAM      `json:"NonVolatileRAM"`
	PRIVersionMajor                               int                  `json:"PRIVersion_Major"`
	PRIVersionMinor                               int                  `json:"PRIVersion_Minor"`
	PRIVersionReleaseNo                           int                  `json:"PRIVersion_ReleaseNo"`
	PairRecordProtectionClass                     int                  `json:"PairRecordProtectionClass"`
	PartitionType                                 string               `json:"PartitionType"`
	PasswordProtected                             bool                 `json:"PasswordProtected"`
	PhoneNumber                                   string               `json:"PhoneNumber"`
	PkHash                                        []byte               `json:"PkHash"`
	ProductName                                   string               `json:"ProductName"`
	ProductType                                   string               `json:"ProductType"`
	ProductVersion                                string               `json:"ProductVersion"`
	ProductionSOC                                 bool                 `json:"ProductionSOC"`
	ProtocolVersion                               string               `json:"ProtocolVersion"`
	RegionInfo                                    string               `json:"RegionInfo"`
	SIM1IsEmbedded                                bool                 `json:"SIM1IsEmbedded"`
	SIM2GID1                                      []byte               `json:"SIM2GID1"`
	SIM2GID2                                      []byte               `json:"SIM2GID2"`
	SIM2IsEmbedded                                bool                 `json:"SIM2IsEmbedded"`
	SIMStatus                                     string               `json:"SIMStatus"`
	SIMTrayStatus                                 string               `json:"SIMTrayStatus"`
	SerialNumber                                  string               `json:"SerialNumber"`
	SoftwareBehavior                              []byte               `json:"SoftwareBehavior"`
	SoftwareBundleVersion                         string               `json:"SoftwareBundleVersion"`
	SupportedDeviceFamilies                       []int                `json:"SupportedDeviceFamilies"`
	TelephonyCapability                           bool                 `json:"TelephonyCapability"`
	TimeIntervalSince1970                         float64              `json:"TimeIntervalSince1970"`
	TimeZone                                      string               `json:"TimeZone"`
	TimeZoneOffsetFromUTC                         float64              `json:"TimeZoneOffsetFromUTC"`
	TrustedHostAttached                           bool                 `json:"TrustedHostAttached"`
	UniqueChipID                                  int64                `json:"UniqueChipID"`
	UniqueDeviceID                                string               `json:"UniqueDeviceID"`
	UseRaptorCerts                                bool                 `json:"UseRaptorCerts"`
	Uses24HourClock                               bool                 `json:"Uses24HourClock"`
	WiFiAddress                                   string               `json:"WiFiAddress"`
	WirelessBoardSerialNumber                     string               `json:"WirelessBoardSerialNumber"`
	KCTPostponementInfoPRIVersion                 string               `json:"kCTPostponementInfoPRIVersion"`
	KCTPostponementInfoServiceProvisioningState   bool                 `json:"kCTPostponementInfoServiceProvisioningState"`
	KCTPostponementStatus                         string               `json:"kCTPostponementStatus"`
}

type StartServiceRequest struct {
	usbmux.RequestRequest
	Service string `plist:"Service"`
}

type StartServiceResponse struct {
	usbmux.Response
	Port    int    `plist:"Port"`
	Request string `plist:"Request"`
	Service string `plist:"Service"`
}

