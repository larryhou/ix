package lockdown

const (
	Name = `com.apple.mobile.lockdown`
	Port = 32498
)


type BasebandKeyHashInformation struct {
	AKeyStatus int    `plist:"AKeyStatus"`
	SKeyHash   string `plist:"SKeyHash"`
	SKeyStatus int    `plist:"SKeyStatus"`
}

type Lockdown struct {
	BasebandCertId                    int64 `plist:"BasebandCertId"`
	*BasebandKeyHashInformation       `plist:"BasebandKeyHashInformation"`
	BasebandSerialNumber              string `plist:"BasebandSerialNumber"`
	BasebandVersion                   string `plist:"BasebandVersion"`
	BoardId                           int    `plist:"BoardId"`
	BuildVersion                      string `plist:"BuildVersion"`
	CPUArchitecture                   string `plist:"CPUArchitecture"`
	ChipID                            int    `plist:"ChipID"`
	DeviceClass                       string `plist:"DeviceClass"`
	DeviceColor                       string `plist:"DeviceColor"`
	DeviceName                        string `plist:"DeviceName"`
	DieID                             int64  `plist:"DieID"`
	HardwareModel                     string `plist:"HardwareModel"`
	HasSiDP                           bool   `plist:"HasSiDP"`
	HumanReadableProductVersionString string `plist:"HumanReadableProductVersionString"`
	PartitionType                     string `plist:"PartitionType"`
	ProductName                       string `plist:"ProductName"`
	ProductType                       string `plist:"ProductType"`
	ProductVersion                    string `plist:"ProductVersion"`
	ProductionSOC                     bool   `plist:"ProductionSOC"`
	ProtocolVersion                   string `plist:"ProtocolVersion"`
	SupportedDeviceFamilies           []int  `plist:"SupportedDeviceFamilies"`
	TelephonyCapability               bool   `plist:"TelephonyCapability"`
	UniqueChipID                      int64  `plist:"UniqueChipID"`
	UniqueDeviceID                    string `plist:"UniqueDeviceID"`
	WiFiAddress                       string `plist:"WiFiAddress"`
}