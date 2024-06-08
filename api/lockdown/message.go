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

type Descriptor struct {
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
	CFBundleIdentifier                    string `plist:"CFBundleIdentifier"`
	CFBundleVersion                       string `plist:"CFBundleVersion"`
	IntegratedCircuitCardIdentity         string `plist:"IntegratedCircuitCardIdentity"`
	InternationalMobileSubscriberIdentity string `plist:"InternationalMobileSubscriberIdentity"`
	MCC                                   string `plist:"MCC"`
	MNC                                   string `plist:"MNC"`
	Slot                                  string `plist:"Slot"`
	KCTPostponementInfoAvailable          string `plist:"kCTPostponementInfoAvailable"`
	GID1                                  string `plist:"GID1,omitempty"`
	GID2                                  string `plist:"GID2,omitempty"`
	SIMGID1                               []byte `plist:"SIMGID1,omitempty"`
	SIMGID2                               []byte `plist:"SIMGID2,omitempty"`
}

type NonVolatileRAM struct {
	StartupMute              []byte `plist:"StartupMute"`
	AutoBoot                 []byte `plist:"auto-boot"`
	BacklightLevel           []byte `plist:"backlight-level"`
	BacklightNits            []byte `plist:"backlight-nits"`
	Bootdelay                []byte `plist:"bootdelay"`
	DisplayCrossbar0         []byte `plist:"display-crossbar0"`
	FmAccountMasked          []byte `plist:"fm-account-masked"`
	FmActivationLocked       []byte `plist:"fm-activation-locked"`
	FmSpkeys                 []byte `plist:"fm-spkeys"`
	FmSpstatus               []byte `plist:"fm-spstatus"`
	OtaControllerVersion     []byte `plist:"ota-controllerVersion"`
	OtaOriginalBaseOsVersion []byte `plist:"ota-original-base-os-version"`
	UsbcVersionRid0          []byte `plist:"usbc,version,rid0"`
}

type Lockdown struct {
	ActivationState                               string               `plist:"ActivationState"`
	ActivationStateAcknowledged                   bool                 `plist:"ActivationStateAcknowledged"`
	BasebandActivationTicketVersion               string               `plist:"BasebandActivationTicketVersion"`
	BasebandCertId                                int64                `plist:"BasebandCertId"`
	BasebandChipID                                int                  `plist:"BasebandChipID"`
	BasebandKeyHashInformation                    *KeyHashInformation  `plist:"BasebandKeyHashInformation"`
	BasebandMasterKeyHash                         string               `plist:"BasebandMasterKeyHash"`
	BasebandRegionSKU                             []byte               `plist:"BasebandRegionSKU"`
	BasebandSerialNumber                          []byte               `plist:"BasebandSerialNumber"`
	BasebandStatus                                string               `plist:"BasebandStatus"`
	BasebandVersion                               string               `plist:"BasebandVersion"`
	BluetoothAddress                              string               `plist:"BluetoothAddress"`
	BoardId                                       int                  `plist:"BoardId"`
	BootSessionID                                 string               `plist:"BootSessionID"`
	BrickState                                    bool                 `plist:"BrickState"`
	BuildVersion                                  string               `plist:"BuildVersion"`
	CPUArchitecture                               string               `plist:"CPUArchitecture"`
	CarrierBundleInfoArray                        []*CarrierBundleInfo `plist:"CarrierBundleInfoArray"`
	CertID                                        int64                `plist:"CertID"`
	ChipID                                        int                  `plist:"ChipID"`
	ChipSerialNo                                  []byte               `plist:"ChipSerialNo"`
	DeviceClass                                   string               `plist:"DeviceClass"`
	DeviceColor                                   string               `plist:"DeviceColor"`
	DeviceName                                    string               `plist:"DeviceName"`
	DieID                                         int64                `plist:"DieID"`
	EthernetAddress                               string               `plist:"EthernetAddress"`
	FirmwareVersion                               string               `plist:"FirmwareVersion"`
	FusingStatus                                  int                  `plist:"FusingStatus"`
	HardwareModel                                 string               `plist:"HardwareModel"`
	HardwarePlatform                              string               `plist:"HardwarePlatform"`
	HasSiDP                                       bool                 `plist:"HasSiDP"`
	HostAttached                                  bool                 `plist:"HostAttached"`
	HumanReadableProductVersionString             string               `plist:"HumanReadableProductVersionString"`
	IntegratedCircuitCardIdentity                 string               `plist:"IntegratedCircuitCardIdentity"`
	IntegratedCircuitCardIdentity2                string               `plist:"IntegratedCircuitCardIdentity2"`
	InternationalMobileEquipmentIdentity          string               `plist:"InternationalMobileEquipmentIdentity"`
	InternationalMobileEquipmentIdentity2         string               `plist:"InternationalMobileEquipmentIdentity2"`
	InternationalMobileSubscriberIdentity         string               `plist:"InternationalMobileSubscriberIdentity"`
	InternationalMobileSubscriberIdentity2        string               `plist:"InternationalMobileSubscriberIdentity2"`
	InternationalMobileSubscriberIdentityOverride bool                 `plist:"InternationalMobileSubscriberIdentityOverride"`
	MLBSerialNumber                               string               `plist:"MLBSerialNumber"`
	MobileSubscriberCountryCode                   string               `plist:"MobileSubscriberCountryCode"`
	MobileSubscriberNetworkCode                   string               `plist:"MobileSubscriberNetworkCode"`
	ModelNumber                                   string               `plist:"ModelNumber"`
	NonVolatileRAM                                *NonVolatileRAM      `plist:"NonVolatileRAM"`
	PRIVersionMajor                               int                  `plist:"PRIVersion_Major"`
	PRIVersionMinor                               int                  `plist:"PRIVersion_Minor"`
	PRIVersionReleaseNo                           int                  `plist:"PRIVersion_ReleaseNo"`
	PairRecordProtectionClass                     int                  `plist:"PairRecordProtectionClass"`
	PartitionType                                 string               `plist:"PartitionType"`
	PasswordProtected                             bool                 `plist:"PasswordProtected"`
	PhoneNumber                                   string               `plist:"PhoneNumber"`
	PkHash                                        []byte               `plist:"PkHash"`
	ProductName                                   string               `plist:"ProductName"`
	ProductType                                   string               `plist:"ProductType"`
	ProductVersion                                string               `plist:"ProductVersion"`
	ProductionSOC                                 bool                 `plist:"ProductionSOC"`
	ProtocolVersion                               string               `plist:"ProtocolVersion"`
	RegionInfo                                    string               `plist:"RegionInfo"`
	SIM1IsEmbedded                                bool                 `plist:"SIM1IsEmbedded"`
	SIM2GID1                                      []byte               `plist:"SIM2GID1"`
	SIM2GID2                                      []byte               `plist:"SIM2GID2"`
	SIM2IsEmbedded                                bool                 `plist:"SIM2IsEmbedded"`
	SIMStatus                                     string               `plist:"SIMStatus"`
	SIMTrayStatus                                 string               `plist:"SIMTrayStatus"`
	SerialNumber                                  string               `plist:"SerialNumber"`
	SoftwareBehavior                              []byte               `plist:"SoftwareBehavior"`
	SoftwareBundleVersion                         string               `plist:"SoftwareBundleVersion"`
	SupportedDeviceFamilies                       []int                `plist:"SupportedDeviceFamilies"`
	TelephonyCapability                           bool                 `plist:"TelephonyCapability"`
	TimeIntervalSince1970                         float64              `plist:"TimeIntervalSince1970"`
	TimeZone                                      string               `plist:"TimeZone"`
	TimeZoneOffsetFromUTC                         float64              `plist:"TimeZoneOffsetFromUTC"`
	TrustedHostAttached                           bool                 `plist:"TrustedHostAttached"`
	UniqueChipID                                  int64                `plist:"UniqueChipID"`
	UniqueDeviceID                                string               `plist:"UniqueDeviceID"`
	UseRaptorCerts                                bool                 `plist:"UseRaptorCerts"`
	Uses24HourClock                               bool                 `plist:"Uses24HourClock"`
	WiFiAddress                                   string               `plist:"WiFiAddress"`
	WirelessBoardSerialNumber                     string               `plist:"WirelessBoardSerialNumber"`
	KCTPostponementInfoPRIVersion                 string               `plist:"kCTPostponementInfoPRIVersion"`
	KCTPostponementInfoServiceProvisioningState   bool                 `plist:"kCTPostponementInfoServiceProvisioningState"`
	KCTPostponementStatus                         string               `plist:"kCTPostponementStatus"`
}

type StartServiceRequest struct {
	usbmux.RequestRequest
	Service string `plist:"Service"`
}

type StartServiceResponse struct {
	usbmux.Response
	Port             int    `plist:"Port"`
	Request          string `plist:"Request"`
	Service          string `plist:"Service"`
	EnableServiceSSL bool   `plist:"EnableServiceSSL"`
}

