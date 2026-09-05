// Package appservice implements the com.apple.coredevice.appservice RSD service.
// It provides application listing, launch, process management, and signal delivery
// via the CoreDevice protocol (iOS 17+).
package appservice

import (
	"fmt"
	"github.com/larryhou/ix/api/coredevice"
	"github.com/larryhou/ix/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.appservice`

const (
	featureListApps                  = `com.apple.coredevice.feature.listapps`
	featureLaunchApplication         = `com.apple.coredevice.feature.launchapplication`
	featureListProcesses             = `com.apple.coredevice.feature.listprocesses`
	featureUninstallApp              = `com.apple.coredevice.feature.uninstallapp`
	featureSendSignalToProcess       = `com.apple.coredevice.feature.sendsignaltoprocess`
	featureMonitorProcessTermination = `com.apple.coredevice.feature.monitorprocesstermination`
	featureSpawnExecutable           = `com.apple.coredevice.feature.spawnexecutable`
	featureListRoots                 = `com.apple.coredevice.feature.listroots`
)

// AppInfo holds basic information about an installed application.
type AppInfo struct {
	BundleID    string
	Name        string
	Version     string
	IsSystem    bool
	IsRemovable bool
	Raw         map[string]any
}

// ProcessInfo holds information about a running process.
type ProcessInfo struct {
	PID  int64
	Name string
	Raw  map[string]any
}

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// ListApps returns all installed applications.
// When includeSystem is true, built-in system apps are included.
func (s *Service) ListApps(includeSystem bool) ([]*AppInfo, error) {
	out, err := s.Invoke(featureListApps, "", map[string]any{
		"includeAppClips":            true,
		"includeRemovableApps":       true,
		"includeHiddenApps":          true,
		"includeInternalApps":        includeSystem,
		"includeDefaultApps":         includeSystem,
		"requireContainerAccess":     false,
		"includeAppGroupIdentifiers": false,
		"includeContainerPaths":      false,
	})
	if err != nil {
		return nil, err
	}

	items, ok := out.([]any)
	if !ok {
		return nil, fmt.Errorf("appservice: unexpected ListApps output type %T", out)
	}

	apps := make([]*AppInfo, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		app := &AppInfo{Raw: m}
		app.BundleID, _ = m["bundleIdentifier"].(string)
		app.Name, _ = m["name"].(string)
		app.Version, _ = m["version"].(string)
		app.IsSystem, _ = m["isInternalApp"].(bool)
		app.IsRemovable, _ = m["isRemovable"].(bool)
		apps = append(apps, app)
	}
	return apps, nil
}

// LaunchOptions configures how an application is launched.
type LaunchOptions struct {
	// Arguments are the command-line arguments passed to the app.
	Arguments []string
	// Env is the set of environment variables to set for the process.
	Env map[string]string
	// TerminateExisting kills any existing instance before launching.
	TerminateExisting bool
	// StartStopped launches the process in a suspended state (for debugger attach).
	StartStopped bool
}

// Launch starts the application identified by bundleID.
// Returns the launched process information.
func (s *Service) Launch(bundleID string, opts *LaunchOptions) ([]any, error) {
	if opts == nil {
		opts = &LaunchOptions{TerminateExisting: true}
	}

	args := make([]any, len(opts.Arguments))
	for i, a := range opts.Arguments {
		args[i] = a
	}

	env := map[string]any{}
	for k, v := range opts.Env {
		env[k] = v
	}

	// platformSpecificOptions must be a binary plist-encoded empty dict.
	// Use a pre-encoded constant rather than importing plistlib.
	// Binary plist for {} : bplist00T$top\xd0\x08\x00\x00\x00\x00\x00\x00\x00\x01\x00...
	// The simplest correct encoding: use a nil/empty bytes value; the device accepts it.
	bplistEmptyDict := []byte{
		0x62, 0x70, 0x6c, 0x69, 0x73, 0x74, 0x30, 0x30, // bplist00
		0xd0,                                             // empty dict (0 pairs)
		0x08,                                             // offset table offset
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,
	}

	out, err := s.Invoke(featureLaunchApplication, "", map[string]any{
		"applicationSpecifier": map[string]any{
			"bundleIdentifier": map[string]any{"_0": bundleID},
		},
		"options": map[string]any{
			"arguments":                    args,
			"environmentVariables":         env,
			"standardIOUsesPseudoterminals": true,
			"startStopped":                 opts.StartStopped,
			"terminateExisting":            opts.TerminateExisting,
			"user":                         map[string]any{"shortName": "mobile"},
			"platformSpecificOptions":      bplistEmptyDict,
		},
		"standardIOIdentifiers": map[string]any{},
	})
	if err != nil {
		return nil, err
	}

	if items, ok := out.([]any); ok {
		return items, nil
	}
	return nil, nil
}

// ListProcesses returns all running processes on the device.
func (s *Service) ListProcesses() ([]*ProcessInfo, error) {
	out, err := s.Invoke(featureListProcesses, "", map[string]any{})
	if err != nil {
		return nil, err
	}

	m, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("appservice: unexpected ListProcesses output type %T", out)
	}

	tokens, ok := m["processTokens"].([]any)
	if !ok {
		return nil, nil
	}

	procs := make([]*ProcessInfo, 0, len(tokens))
	for _, t := range tokens {
		pm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		proc := &ProcessInfo{Raw: pm}
		switch pid := pm["processIdentifier"].(type) {
		case int64:
			proc.PID = pid
		case uint64:
			proc.PID = int64(pid)
		}
		proc.Name, _ = pm["name"].(string)
		procs = append(procs, proc)
	}
	return procs, nil
}

// Uninstall removes the application with the given bundle identifier.
func (s *Service) Uninstall(bundleID string) error {
	_, err := s.Invoke(featureUninstallApp, "", map[string]any{
		"bundleIdentifier": bundleID,
	})
	return err
}

// SendSignal sends a Unix signal to the process identified by pid.
// Use standard signal numbers (e.g. syscall.SIGKILL = 9).
func (s *Service) SendSignal(pid int64, signal int) error {
	_, err := s.Invoke(featureSendSignalToProcess, "", map[string]any{
		"process": map[string]any{"processIdentifier": pid},
		"signal":  int64(signal),
	})
	return err
}

// Kill sends SIGKILL (9) to the process identified by pid.
func (s *Service) Kill(pid int64) error {
	return s.SendSignal(pid, 9)
}
