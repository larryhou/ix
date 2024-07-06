package rsd

//goland:noinspection GoCommentStart
const (
	// AppleInternal
	ComAppleCarkitRemoteIapService = `com.apple.carkit.remote-iap.service`  // xpc:true
	ComAppleDtTestmanagerdRemoteAutomation = `com.apple.dt.testmanagerd.remote.automation`  // xpc:false
	ComAppleInternalDevicecomputeCoreDeviceProxy = `com.apple.internal.devicecompute.CoreDeviceProxy`  // xpc:false

	// com.apple.ReportCrash.antenna-access
	ComAppleOsanalyticsLogTransfer = `com.apple.osanalytics.logTransfer`  // xpc:true

	// com.apple.corecaptured.remoteservice-access
	ComAppleCorecapturedRemoteservice = `com.apple.corecaptured.remoteservice`  // xpc:true

	// com.apple.dt.coredevice.tunnelservice.client
	ComAppleInternalDtCoredeviceUntrustedTunnelservice = `com.apple.internal.dt.coredevice.untrusted.tunnelservice`  // xpc:true

	// com.apple.fusion.remote.service
	ComAppleFusionRemoteService = `com.apple.fusion.remote.service`  // xpc:true

	// com.apple.mobile.insecure_notification_proxy.remote
	ComAppleMobileInsecureNotificationProxyRemote = `com.apple.mobile.insecure_notification_proxy.remote`  // xpc:true

	// com.apple.mobile.lockdown.remote.trusted
	ComAppleGPUToolsMobileServiceShimRemote = `com.apple.GPUTools.MobileService.shim.remote`  // xpc:false
	ComApplePurpleReverseProxyConnShimRemote = `com.apple.PurpleReverseProxy.Conn.shim.remote`  // xpc:false
	ComApplePurpleReverseProxyCtrlShimRemote = `com.apple.PurpleReverseProxy.Ctrl.shim.remote`  // xpc:false
	ComAppleAccessibilityAxAuditDaemonRemoteserverShimRemote = `com.apple.accessibility.axAuditDaemon.remoteserver.shim.remote`  // xpc:false
	ComAppleAfcShimRemote = `com.apple.afc.shim.remote`  // xpc:false
	ComAppleAmfiLockdownShimRemote = `com.apple.amfi.lockdown.shim.remote`  // xpc:false
	ComAppleAtcShimRemote = `com.apple.atc.shim.remote`  // xpc:false
	ComAppleAtc2ShimRemote = `com.apple.atc2.shim.remote`  // xpc:false
	ComAppleBackgroundassetsLockdownserviceShimRemote = `com.apple.backgroundassets.lockdownservice.shim.remote`  // xpc:false
	ComAppleBluetoothBTPacketLoggerShimRemote = `com.apple.bluetooth.BTPacketLogger.shim.remote`  // xpc:false
	ComAppleCarkitServiceShimRemote = `com.apple.carkit.service.shim.remote`  // xpc:false
	ComAppleCommcenterMobileHelperCbupdateserviceShimRemote = `com.apple.commcenter.mobile-helper-cbupdateservice.shim.remote`  // xpc:false
	ComAppleCompanionProxyShimRemote = `com.apple.companion_proxy.shim.remote`  // xpc:false
	ComAppleCrashreportcopymobileShimRemote = `com.apple.crashreportcopymobile.shim.remote`  // xpc:false
	ComAppleCrashreportmoverShimRemote = `com.apple.crashreportmover.shim.remote`  // xpc:false
	ComAppleDtRemotepairingdevicedLockdownShimRemote = `com.apple.dt.remotepairingdeviced.lockdown.shim.remote`  // xpc:false
	ComAppleIdamdShimRemote = `com.apple.idamd.shim.remote`  // xpc:false
	ComAppleInternalDevicecomputeCoreDeviceProxyShimRemote = `com.apple.internal.devicecompute.CoreDeviceProxy.shim.remote`  // xpc:false
	ComAppleIosdiagnosticsRelayShimRemote = `com.apple.iosdiagnostics.relay.shim.remote`  // xpc:false
	ComAppleMisagentShimRemote = `com.apple.misagent.shim.remote`  // xpc:false
	ComAppleMobileMCInstallShimRemote = `com.apple.mobile.MCInstall.shim.remote`  // xpc:false
	ComAppleMobileAssertionAgentShimRemote = `com.apple.mobile.assertion_agent.shim.remote`  // xpc:false
	ComAppleMobileDiagnosticsRelayShimRemote = `com.apple.mobile.diagnostics_relay.shim.remote`  // xpc:false
	ComAppleMobileFileRelayShimRemote = `com.apple.mobile.file_relay.shim.remote`  // xpc:false
	ComAppleMobileHeartbeatShimRemote = `com.apple.mobile.heartbeat.shim.remote`  // xpc:false
	ComAppleMobileHouseArrestShimRemote = `com.apple.mobile.house_arrest.shim.remote`  // xpc:false
	ComAppleMobileInstallationProxyShimRemote = `com.apple.mobile.installation_proxy.shim.remote`  // xpc:false
	ComAppleMobileLockdownRemoteTrusted = `com.apple.mobile.lockdown.remote.trusted`  // xpc:false
	ComAppleMobileMobileImageMounterShimRemote = `com.apple.mobile.mobile_image_mounter.shim.remote`  // xpc:false
	ComAppleMobileNotificationProxyShimRemote = `com.apple.mobile.notification_proxy.shim.remote`  // xpc:false
	ComAppleMobileactivationdShimRemote = `com.apple.mobileactivationd.shim.remote`  // xpc:false
	ComAppleMobilebackup2ShimRemote = `com.apple.mobilebackup2.shim.remote`  // xpc:false
	ComAppleMobilesyncShimRemote = `com.apple.mobilesync.shim.remote`  // xpc:false
	ComAppleOsTraceRelayShimRemote = `com.apple.os_trace_relay.shim.remote`  // xpc:false
	ComApplePcapdShimRemote = `com.apple.pcapd.shim.remote`  // xpc:false
	ComApplePreboardserviceShimRemote = `com.apple.preboardservice.shim.remote`  // xpc:false
	ComApplePreboardserviceV2ShimRemote = `com.apple.preboardservice_v2.shim.remote`  // xpc:false
	ComAppleSpringboardservicesShimRemote = `com.apple.springboardservices.shim.remote`  // xpc:false
	ComAppleStreamingZipConduitShimRemote = `com.apple.streaming_zip_conduit.shim.remote`  // xpc:false
	ComAppleSyslogRelayShimRemote = `com.apple.syslog_relay.shim.remote`  // xpc:false
	ComAppleWebinspectorShimRemote = `com.apple.webinspector.shim.remote`  // xpc:false

	// com.apple.mobile.lockdown.remote.untrusted
	ComAppleMobileInsecureNotificationProxyShimRemote = `com.apple.mobile.insecure_notification_proxy.shim.remote`  // xpc:false
	ComAppleMobileLockdownRemoteUntrusted = `com.apple.mobile.lockdown.remote.untrusted`  // xpc:false

	// com.apple.mobile.notification_proxy.remote
	ComAppleMobileNotificationProxyRemote = `com.apple.mobile.notification_proxy.remote`  // xpc:true

	// com.apple.private.CoreDevice.canDebugApplicationsOnDevice
	ComAppleInternalDtRemoteDebugproxy = `com.apple.internal.dt.remote.debugproxy`  // xpc:true

	// com.apple.private.CoreDevice.canInstallCustomerContent
	ComAppleCoredeviceAppservice = `com.apple.coredevice.appservice`  // xpc:true
	ComAppleCoredeviceOpenstdiosocket = `com.apple.coredevice.openstdiosocket`  // xpc:true

	// com.apple.private.CoreDevice.canObtainDiagnostics
	ComAppleCoredeviceDiagnosticsservice = `com.apple.coredevice.diagnosticsservice`  // xpc:true

	// com.apple.private.CoreDevice.canRetrieveDeviceInfo
	ComAppleCoredeviceDeviceinfo = `com.apple.coredevice.deviceinfo`  // xpc:true

	// com.apple.private.CoreDevice.canTransferFilesToDevice
	ComAppleCoredeviceFileserviceControl = `com.apple.coredevice.fileservice.control`  // xpc:true
	ComAppleCoredeviceFileserviceData = `com.apple.coredevice.fileservice.data`  // xpc:true

	// com.apple.private.InstallCoordinationRemote
	ComAppleRemoteInstallcoordinationProxy = `com.apple.remote.installcoordination_proxy`  // xpc:true

	// com.apple.private.RestoreRemoteServices.restoreservice.remote
	ComAppleRestoreRemoteServicesRestoreserviced = `com.apple.RestoreRemoteServices.restoreserviced`  // xpc:true

	// com.apple.private.dt.ViewHierarchyAgent.client
	ComAppleDtViewHierarchyAgentRemote = `com.apple.dt.ViewHierarchyAgent.remote`  // xpc:true

	// com.apple.private.dt.instruments.dtservicehub.client
	ComAppleInstrumentsDtservicehub = `com.apple.instruments.dtservicehub`  // xpc:false

	// com.apple.private.dt.remoteFetchSymbols.client
	ComAppleDtRemoteFetchSymbols = `com.apple.dt.remoteFetchSymbols`  // xpc:true

	// com.apple.private.dt.testmanagerd.client
	ComAppleDtTestmanagerdRemote = `com.apple.dt.testmanagerd.remote`  // xpc:false

	// com.apple.private.gputoolstransportd
	ComAppleGputoolsRemoteAgent = `com.apple.gputools.remote.agent`  // xpc:true

	// com.apple.private.mobile_storage.remote.allowedSPI
	ComAppleMobileStorageMounterProxyBridge = `com.apple.mobile.storage_mounter_proxy.bridge`  // xpc:true

	// com.apple.private.security.cryptexd.remote
	ComAppleSecurityCryptexdRemote = `com.apple.security.cryptexd.remote`  // xpc:true

	// com.apple.private.sysdiagnose.remote
	ComAppleSysdiagnoseRemote = `com.apple.sysdiagnose.remote`  // xpc:true

	// com.apple.prviate.sysdiagnose.remote.trusted
	ComAppleSysdiagnoseRemoteTrusted = `com.apple.sysdiagnose.remote.trusted`  // xpc:true
)
