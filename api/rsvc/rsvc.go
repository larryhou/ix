package rsvc

//goland:noinspection GoCommentStart
const (
	// AppleInternal
	ComAppleCarkitRemoteIapService               = `com.apple.carkit.remote-iap.service`
	ComAppleDtTestmanagerdRemoteAutomation       = `com.apple.dt.testmanagerd.remote.automation`
	ComAppleInternalDevicecomputeCoreDeviceProxy = `com.apple.internal.devicecompute.CoreDeviceProxy`

	// com.apple.ReportCrash.antenna-access
	ComAppleOsanalyticsLogTransfer = `com.apple.osanalytics.logTransfer`

	// com.apple.corecaptured.remoteservice-access
	ComAppleCorecapturedRemoteservice = `com.apple.corecaptured.remoteservice`

	// com.apple.dt.coredevice.tunnelservice.client
	ComAppleInternalDtCoredeviceUntrustedTunnelservice = `com.apple.internal.dt.coredevice.untrusted.tunnelservice`

	// com.apple.fusion.remote.service
	ComAppleFusionRemoteService = `com.apple.fusion.remote.service`

	// com.apple.mobile.insecure_notification_proxy.remote
	ComAppleMobileInsecureNotificationProxyRemote = `com.apple.mobile.insecure_notification_proxy.remote`

	// com.apple.mobile.lockdown.remote.trusted
	ComAppleGPUToolsMobileServiceShimRemote                  = `com.apple.GPUTools.MobileService.shim.remote`
	ComApplePurpleReverseProxyConnShimRemote                 = `com.apple.PurpleReverseProxy.Conn.shim.remote`
	ComApplePurpleReverseProxyCtrlShimRemote                 = `com.apple.PurpleReverseProxy.Ctrl.shim.remote`
	ComAppleAccessibilityAxAuditDaemonRemoteserverShimRemote = `com.apple.accessibility.axAuditDaemon.remoteserver.shim.remote`
	ComAppleAfcShimRemote                                    = `com.apple.afc.shim.remote`
	ComAppleAmfiLockdownShimRemote                           = `com.apple.amfi.lockdown.shim.remote`
	ComAppleAtcShimRemote                                    = `com.apple.atc.shim.remote`
	ComAppleAtc2ShimRemote                                   = `com.apple.atc2.shim.remote`
	ComAppleBackgroundassetsLockdownserviceShimRemote        = `com.apple.backgroundassets.lockdownservice.shim.remote`
	ComAppleBluetoothBTPacketLoggerShimRemote                = `com.apple.bluetooth.BTPacketLogger.shim.remote`
	ComAppleCarkitServiceShimRemote                          = `com.apple.carkit.service.shim.remote`
	ComAppleCommcenterMobileHelperCbupdateserviceShimRemote  = `com.apple.commcenter.mobile-helper-cbupdateservice.shim.remote`
	ComAppleCompanionProxyShimRemote                         = `com.apple.companion_proxy.shim.remote`
	ComAppleCrashreportcopymobileShimRemote                  = `com.apple.crashreportcopymobile.shim.remote`
	ComAppleCrashreportmoverShimRemote                       = `com.apple.crashreportmover.shim.remote`
	ComAppleDtRemotepairingdevicedLockdownShimRemote         = `com.apple.dt.remotepairingdeviced.lockdown.shim.remote`
	ComAppleIdamdShimRemote                                  = `com.apple.idamd.shim.remote`
	ComAppleInternalDevicecomputeCoreDeviceProxyShimRemote   = `com.apple.internal.devicecompute.CoreDeviceProxy.shim.remote`
	ComAppleIosdiagnosticsRelayShimRemote                    = `com.apple.iosdiagnostics.relay.shim.remote`
	ComAppleMisagentShimRemote                               = `com.apple.misagent.shim.remote`
	ComAppleMobileMCInstallShimRemote                        = `com.apple.mobile.MCInstall.shim.remote`
	ComAppleMobileAssertionAgentShimRemote                   = `com.apple.mobile.assertion_agent.shim.remote`
	ComAppleMobileDiagnosticsRelayShimRemote                 = `com.apple.mobile.diagnostics_relay.shim.remote`
	ComAppleMobileFileRelayShimRemote                        = `com.apple.mobile.file_relay.shim.remote`
	ComAppleMobileHeartbeatShimRemote                        = `com.apple.mobile.heartbeat.shim.remote`
	ComAppleMobileHouseArrestShimRemote                      = `com.apple.mobile.house_arrest.shim.remote`
	ComAppleMobileInstallationProxyShimRemote                = `com.apple.mobile.installation_proxy.shim.remote`
	ComAppleMobileLockdownRemoteTrusted                      = `com.apple.mobile.lockdown.remote.trusted`
	ComAppleMobileMobileImageMounterShimRemote               = `com.apple.mobile.mobile_image_mounter.shim.remote`
	ComAppleMobileNotificationProxyShimRemote                = `com.apple.mobile.notification_proxy.shim.remote`
	ComAppleMobileactivationdShimRemote                      = `com.apple.mobileactivationd.shim.remote`
	ComAppleMobilebackup2ShimRemote                          = `com.apple.mobilebackup2.shim.remote`
	ComAppleMobilesyncShimRemote                             = `com.apple.mobilesync.shim.remote`
	ComAppleOsTraceRelayShimRemote                           = `com.apple.os_trace_relay.shim.remote`
	ComApplePcapdShimRemote                                  = `com.apple.pcapd.shim.remote`
	ComApplePreboardserviceShimRemote                        = `com.apple.preboardservice.shim.remote`
	ComApplePreboardserviceV2ShimRemote                      = `com.apple.preboardservice_v2.shim.remote`
	ComAppleSpringboardservicesShimRemote                    = `com.apple.springboardservices.shim.remote`
	ComAppleStreamingZipConduitShimRemote                    = `com.apple.streaming_zip_conduit.shim.remote`
	ComAppleSyslogRelayShimRemote                            = `com.apple.syslog_relay.shim.remote`
	ComAppleWebinspectorShimRemote                           = `com.apple.webinspector.shim.remote`

	// com.apple.mobile.lockdown.remote.untrusted
	ComAppleMobileInsecureNotificationProxyShimRemote = `com.apple.mobile.insecure_notification_proxy.shim.remote`
	ComAppleMobileLockdownRemoteUntrusted             = `com.apple.mobile.lockdown.remote.untrusted`

	// com.apple.mobile.notification_proxy.remote
	ComAppleMobileNotificationProxyRemote = `com.apple.mobile.notification_proxy.remote`

	// com.apple.private.CoreDevice.canDebugApplicationsOnDevice
	ComAppleInternalDtRemoteDebugproxy = `com.apple.internal.dt.remote.debugproxy`

	// com.apple.private.CoreDevice.canInstallCustomerContent
	ComAppleCoredeviceAppservice      = `com.apple.coredevice.appservice`
	ComAppleCoredeviceOpenstdiosocket = `com.apple.coredevice.openstdiosocket`

	// com.apple.private.CoreDevice.canObtainDiagnostics
	ComAppleCoredeviceDiagnosticsservice = `com.apple.coredevice.diagnosticsservice`

	// com.apple.private.CoreDevice.canRetrieveDeviceInfo
	ComAppleCoredeviceDeviceinfo = `com.apple.coredevice.deviceinfo`

	// com.apple.private.CoreDevice.canTransferFilesToDevice
	ComAppleCoredeviceFileserviceControl = `com.apple.coredevice.fileservice.control`
	ComAppleCoredeviceFileserviceData    = `com.apple.coredevice.fileservice.data`

	// com.apple.private.InstallCoordinationRemote
	ComAppleRemoteInstallcoordinationProxy = `com.apple.remote.installcoordination_proxy`

	// com.apple.private.RestoreRemoteServices.restoreservice.remote
	ComAppleRestoreRemoteServicesRestoreserviced = `com.apple.RestoreRemoteServices.restoreserviced`

	// com.apple.private.dt.ViewHierarchyAgent.client
	ComAppleDtViewHierarchyAgentRemote = `com.apple.dt.ViewHierarchyAgent.remote`

	// com.apple.private.dt.instruments.dtservicehub.client
	ComAppleInstrumentsDtservicehub = `com.apple.instruments.dtservicehub`

	// com.apple.private.dt.remoteFetchSymbols.client
	ComAppleDtRemoteFetchSymbols = `com.apple.dt.remoteFetchSymbols`

	// com.apple.private.dt.testmanagerd.client
	ComAppleDtTestmanagerdRemote = `com.apple.dt.testmanagerd.remote`

	// com.apple.private.gputoolstransportd
	ComAppleGputoolsRemoteAgent = `com.apple.gputools.remote.agent`

	// com.apple.private.mobile_storage.remote.allowedSPI
	ComAppleMobileStorageMounterProxyBridge = `com.apple.mobile.storage_mounter_proxy.bridge`

	// com.apple.private.security.cryptexd.remote
	ComAppleSecurityCryptexdRemote = `com.apple.security.cryptexd.remote`

	// com.apple.private.sysdiagnose.remote
	ComAppleSysdiagnoseRemote = `com.apple.sysdiagnose.remote`

	// com.apple.prviate.sysdiagnose.remote.trusted
	ComAppleSysdiagnoseRemoteTrusted = `com.apple.sysdiagnose.remote.trusted`
)
