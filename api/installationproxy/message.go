package installationproxy

import (
	"github.com/larryhou/ix/api/mux"
)


type Application struct {
	AVInitialRouteSharingPolicy                      string                       `plist:"AVInitialRouteSharingPolicy,omitempty"`
	ApplicationDSID                                  int64                        `plist:"ApplicationDSID"`
	ApplicationType                                  string                       `plist:"ApplicationType"`
	BuildMachineOSBuild                              string                       `plist:"BuildMachineOSBuild"`
	CFBundleDevelopmentRegion                        string                       `plist:"CFBundleDevelopmentRegion"`
	CFBundleDisplayName                              string                       `plist:"CFBundleDisplayName"`
	CFBundleDocumentTypes                            []*CFBundleDocumentType      `plist:"CFBundleDocumentTypes"`
	CFBundleExecutable                               string                       `plist:"CFBundleExecutable"`
	CFBundleIcons                                    *CFBundleIcons               `plist:"CFBundleIcons"`
	CFBundleIdentifier                               string                       `plist:"CFBundleIdentifier"`
	CFBundleInfoDictionaryVersion                    string                       `plist:"CFBundleInfoDictionaryVersion"`
	CFBundleName                                     string                       `plist:"CFBundleName"`
	CFBundleNumericVersion                           int64                        `plist:"CFBundleNumericVersion"`
	CFBundlePackageType                              string                       `plist:"CFBundlePackageType"`
	CFBundleShortVersionString                       string                       `plist:"CFBundleShortVersionString"`
	CFBundleSignature                                string                       `plist:"CFBundleSignature"`
	CFBundleSupportedPlatforms                       []string                     `plist:"CFBundleSupportedPlatforms"`
	CFBundleURLTypes                                 []*CFBundleURLType           `plist:"CFBundleURLTypes"`
	CFBundleVersion                                  string                       `plist:"CFBundleVersion"`
	Container                                        string                       `plist:"Container"`
	DTAppStoreToolsBuild                             string                       `plist:"DTAppStoreToolsBuild"`
	DTCompiler                                       string                       `plist:"DTCompiler"`
	DTPlatformBuild                                  string                       `plist:"DTPlatformBuild"`
	DTPlatformName                                   string                       `plist:"DTPlatformName"`
	DTPlatformVersion                                string                       `plist:"DTPlatformVersion"`
	DTSDKBuild                                       string                       `plist:"DTSDKBuild"`
	DTSDKName                                        string                       `plist:"DTSDKName"`
	DTXcode                                          string                       `plist:"DTXcode"`
	DTXcodeBuild                                     string                       `plist:"DTXcodeBuild"`
	Entitlements                                     *Entitlements                `plist:"Entitlements"`
	EnvironmentVariables                             *EnvironmentVariables        `plist:"EnvironmentVariables"`
	GroupContainers                                  map[string]string            `plist:"GroupContainers"`
	INAlternativeAppNames                            []*INAlternativeAppName      `plist:"INAlternativeAppNames,omitempty"`
	ITSDRMScheme                                     string                       `plist:"ITSDRMScheme"`
	LSApplicationQueriesSchemes                      []string                     `plist:"LSApplicationQueriesSchemes"`
	LSHasLocalizedDisplayName                        bool                         `plist:"LSHasLocalizedDisplayName,omitempty"`
	LSRequiresIPhoneOS                               bool                         `plist:"LSRequiresIPhoneOS"`
	LSSupportsOpeningDocumentsInPlace                bool                         `plist:"LSSupportsOpeningDocumentsInPlace"`
	MinimumOSVersion                                 string                       `plist:"MinimumOSVersion"`
	NSAppTransportSecurity                           *NSAppTransportSecurity      `plist:"NSAppTransportSecurity"`
	NSAppleMusicUsageDescription                     string                       `plist:"NSAppleMusicUsageDescription,omitempty"`
	NSBluetoothAlwaysUsageDescription                string                       `plist:"NSBluetoothAlwaysUsageDescription"`
	NSBluetoothPeripheralUsageDescription            string                       `plist:"NSBluetoothPeripheralUsageDescription"`
	NSBonjourServices                                []string                     `plist:"NSBonjourServices,omitempty"`
	NSCalendarsFullAccessUsageDescription            string                       `plist:"NSCalendarsFullAccessUsageDescription,omitempty"`
	NSCalendarsUsageDescription                      string                       `plist:"NSCalendarsUsageDescription"`
	NSCalendarsWriteOnlyAccessUsageDescription       string                       `plist:"NSCalendarsWriteOnlyAccessUsageDescription,omitempty"`
	NSCameraUsageDescription                         string                       `plist:"NSCameraUsageDescription"`
	NSFaceIDUsageDescription                         string                       `plist:"NSFaceIDUsageDescription"`
	NSLocalNetworkUsageDescription                   string                       `plist:"NSLocalNetworkUsageDescription"`
	NSLocationDefaultAccuracyReduced                 any                          `plist:"NSLocationDefaultAccuracyReduced,omitempty"` // bool or string
	NSLocationWhenInUseUsageDescription              string                       `plist:"NSLocationWhenInUseUsageDescription"`
	NSMicrophoneUsageDescription                     string                       `plist:"NSMicrophoneUsageDescription"`
	NSMotionUsageDescription                         string                       `plist:"NSMotionUsageDescription"`
	NSPhotoLibraryAddUsageDescription                string                       `plist:"NSPhotoLibraryAddUsageDescription"`
	NSPhotoLibraryUsageDescription                   string                       `plist:"NSPhotoLibraryUsageDescription"`
	NSSiriUsageDescription                           string                       `plist:"NSSiriUsageDescription,omitempty"`
	NSUserActivityTypes                              any                          `plist:"NSUserActivityTypes"`
	NSUserTrackingUsageDescription                   string                       `plist:"NSUserTrackingUsageDescription,omitempty"`
	PHPhotoLibraryPreventAutomaticLimitedAccessAlert bool                         `plist:"PHPhotoLibraryPreventAutomaticLimitedAccessAlert"`
	UIAppFonts                                       []string                     `plist:"UIAppFonts,omitempty"`
	UIApplicationShortcutItems                       []*UIApplicationShortcutItem `plist:"UIApplicationShortcutItems,omitempty"`
	UIBackgroundModes                                []string                     `plist:"UIBackgroundModes"`
	UIDeviceFamily                                   []int64                      `plist:"UIDeviceFamily"`
	UILaunchStoryboardName                           string                       `plist:"UILaunchStoryboardName"`
	UIRequiredDeviceCapabilities                     any                          `plist:"UIRequiredDeviceCapabilities,omitempty"`
	UIRequiresFullScreen                             any                          `plist:"UIRequiresFullScreen,omitempty"`
	UISearchFonts                                    []string                     `plist:"UISearchFonts,omitempty"`
	UIStatusBarHidden                                bool                         `plist:"UIStatusBarHidden"`
	UISupportedDevices                               []string                     `plist:"UISupportedDevices"`
	UISupportedInterfaceOrientations                 []string                     `plist:"UISupportedInterfaceOrientations"`
	UTExportedTypeDeclarations                       []*UTExportedTypeDeclaration `plist:"UTExportedTypeDeclarations,omitempty"`
	BGTaskSchedulerPermittedIdentifiers              []string                     `plist:"BGTaskSchedulerPermittedIdentifiers,omitempty"`
	CADisableMinimumFrameDurationOnPhone             bool                         `plist:"CADisableMinimumFrameDurationOnPhone,omitempty"`
	FLTEnableImpeller                                bool                         `plist:"FLTEnableImpeller,omitempty"`
	FLTEnableSkParagraph                             bool                         `plist:"FLTEnableSkParagraph,omitempty"`
	FLTLeakDartVM                                    bool                         `plist:"FLTLeakDartVM,omitempty"`
	NSContactsUsageDescription                       string                       `plist:"NSContactsUsageDescription,omitempty"`
	NSHealthShareUsageDescription                    string                       `plist:"NSHealthShareUsageDescription,omitempty"`
	NSHealthUpdateUsageDescription                   string                       `plist:"NSHealthUpdateUsageDescription,omitempty"`
	UIStatusBarStyle                                 string                       `plist:"UIStatusBarStyle,omitempty"`
	UIViewControllerBasedStatusBarAppearance         any                          `plist:"UIViewControllerBasedStatusBarAppearance,omitempty"`
	FLTEnableWideGamut                               bool                         `plist:"FLTEnableWideGamut,omitempty"`
	NSFocusStatusUsageDescription                    string                       `plist:"NSFocusStatusUsageDescription,omitempty"`
	NSLocationAlwaysUsageDescription                 string                       `plist:"NSLocationAlwaysUsageDescription,omitempty"`
	NSLocationTemporaryUsageDescriptionDictionary    map[string]string            `plist:"NSLocationTemporaryUsageDescriptionDictionary,omitempty"`
	NSLocationUsageDescription                       *string                      `plist:"NSLocationUsageDescription,omitempty"`
	SKAdNetworkItems                                 []*SKAdNetworkItem           `plist:"SKAdNetworkItems,omitempty"`
	UIApplicationSceneManifest                       *UIApplicationSceneManifest  `plist:"UIApplicationSceneManifest,omitempty"`
	UISupportsDocumentBrowser                        bool                         `plist:"UISupportsDocumentBrowser,omitempty"`
}

type CFBundleDocumentType struct {
	CFBundleTypeName      string   `plist:"CFBundleTypeName"`
	LSHandlerRank         string   `plist:"LSHandlerRank"`
	LSItemContentTypes    []string `plist:"LSItemContentTypes"`
	CFBundleTypeIconFiles []string `plist:"CFBundleTypeIconFiles,omitempty"`
}

type CFBundleIcons struct {
	CFBundlePrimaryIcon    *CFBundleIcon `plist:"CFBundlePrimaryIcon"`
	CFBundleAlternateIcons *CFBundleIcon `plist:"CFBundleAlternateIcons,omitempty"`
}

type CFBundleIcon struct {
	CFBundleIconFiles []string `plist:"CFBundleIconFiles"`
	CFBundleIconName  string   `plist:"CFBundleIconName"`
}

type CFBundleURLType struct {
	CFBundleTypeRole   string   `plist:"CFBundleTypeRole"`
	CFBundleURLName    string   `plist:"CFBundleURLName,omitempty"`
	CFBundleURLSchemes []string `plist:"CFBundleURLSchemes"`
}

type Entitlements struct {
	ApplicationIdentifier                                    string   `plist:"application-identifier"`
	ApsEnvironment                                           string   `plist:"aps-environment"`
	//COMAppleDeveloperAssociatedDomains                       []string `plist:"com.apple.developer.associated-domains"`
	COMAppleDeveloperNetworkingMulticast                     bool     `plist:"com.apple.developer.networking.multicast,omitempty"`
	COMAppleDeveloperSiri                                    bool     `plist:"com.apple.developer.siri,omitempty"`
	COMAppleDeveloperUsernotificationsCommunication          bool     `plist:"com.apple.developer.usernotifications.communication"`
	COMAppleSecurityApplicationGroups                        []string `plist:"com.apple.security.application-groups"`
	KeychainAccessGroups                                     []string `plist:"keychain-access-groups,omitempty"`
	COMAppleDeveloperHealthkit                               bool     `plist:"com.apple.developer.healthkit,omitempty"`
	COMAppleDeveloperHealthkitAccess                         []any    `plist:"com.apple.developer.healthkit.access,omitempty"`
	COMAppleDeveloperKernelExtendedVirtualAddressing         bool     `plist:"com.apple.developer.kernel.extended-virtual-addressing,omitempty"`
	COMAppleDeveloperKernelIncreasedMemoryLimit              bool     `plist:"com.apple.developer.kernel.increased-memory-limit,omitempty"`
	COMAppleDeveloperNetworkingHotspotConfiguration          bool     `plist:"com.apple.developer.networking.HotspotConfiguration,omitempty"`
	COMAppleDeveloperNetworkingHotspotHelper                 bool     `plist:"com.apple.developer.networking.HotspotHelper,omitempty"`
	COMAppleDeveloperNetworkingWifiInfo                      bool     `plist:"com.apple.developer.networking.wifi-info,omitempty"`
	COMAppleDeveloperPushkitUnrestrictedVoipRegulatory       bool     `plist:"com.apple.developer.pushkit.unrestricted-voip-regulatory,omitempty"`
	COMAppleDeveloperApplesignin                             []string `plist:"com.apple.developer.applesignin,omitempty"`
	COMAppleDeveloperAvfoundationMultitaskingCameraAccess    bool     `plist:"com.apple.developer.avfoundation.multitasking-camera-access,omitempty"`
	COMAppleDeveloperCarplayCommunication                    bool     `plist:"com.apple.developer.carplay-communication,omitempty"`
	COMAppleDeveloperDeviceInformationUserAssignedDeviceName bool     `plist:"com.apple.developer.device-information.user-assigned-device-name,omitempty"`
	COMAppleDeveloperNetworkingNetworkextension              []string `plist:"com.apple.developer.networking.networkextension,omitempty"`
	COMAppleDeveloperPassTypeIdentifiers                     []string `plist:"com.apple.developer.pass-type-identifiers,omitempty"`
	COMAppleDeveloperPaymentPassProvisioning                 bool     `plist:"com.apple.developer.payment-pass-provisioning,omitempty"`
	COMAppleDeveloperTeamIdentifier                          string   `plist:"com.apple.developer.team-identifier,omitempty"`
}

type EnvironmentVariables struct {
	CffixedUserHome string `plist:"CFFIXED_USER_HOME"`
	Home            string `plist:"HOME"`
	Tmpdir          string `plist:"TMPDIR"`
}

type INAlternativeAppName struct {
	INAlternativeAppName string `plist:"INAlternativeAppName"`
}

type NSAppTransportSecurity struct {
	NSAllowsArbitraryLoads bool `plist:"NSAllowsArbitraryLoads"`
}

type SKAdNetworkItem struct {
	SKAdNetworkIdentifier string `plist:"SKAdNetworkIdentifier"`
}

type UIApplicationSceneManifest struct {
	UIApplicationSupportsMultipleScenes bool `plist:"UIApplicationSupportsMultipleScenes"`
	UISceneConfigurations               any  `plist:"UISceneConfigurations"`
}

type UIApplicationShortcutItem struct {
	UIApplicationShortcutItemIconType string `plist:"UIApplicationShortcutItemIconType,omitempty"`
	UIApplicationShortcutItemTitle    string `plist:"UIApplicationShortcutItemTitle"`
	UIApplicationShortcutItemType     string `plist:"UIApplicationShortcutItemType"`
	UIApplicationShortcutItemIconFile string `plist:"UIApplicationShortcutItemIconFile,omitempty"`
}

type UTExportedTypeDeclaration struct {
	UTTypeConformsTo       any            `plist:"UTTypeConformsTo"`
	UTTypeIdentifier       string         `plist:"UTTypeIdentifier"`
	UTTypeDescription      string         `plist:"UTTypeDescription"`
	UTTypeTagSpecification map[string]any `plist:"UTTypeTagSpecification"`
}

type ClientOptions struct {
	ApplicationType       string `plist:"ApplicationType,omitempty"`
	ApplicationIdentifier string `plist:"ApplicationIdentifier,omitempty"`
}

type Request struct {
	*ClientOptions `plist:"ClientOptions"`
	Command        string `plist:"Command"`
}

type ListRequest Request
type ListResponse struct {
	mux.Response
	LookupResult map[string]*Application `plist:"LookupResult"`
	Status       string                  `plist:"Status"`
}

type ProgressResponse struct {
	mux.Response
	PercentComplete int    `plist:"PercentComplete"`
	Status          string `plist:"Status"`
}
