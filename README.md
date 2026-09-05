# ix

A pure-Go implementation of iOS device control, inspired by [pymobiledevice3](https://github.com/doronz88/pymobiledevice3).

- **No CGo** — pure Go, including TLS-PSK handshake
- **iOS 17+ first** — full CoreDevice/RSD/XPC tunnel support
- **USB + WiFi** — automatic discovery via usbmuxd and Bonjour

---

## Requirements

- Go 1.24+
- macOS (tunnel interface creation requires Darwin)
- `sudo` for tunneld (TUN interface requires root)
- Device trusted on this Mac (paired via Finder/iTunes)

---

## Quick Start

```bash
# Build and start the tunnel daemon
go build -o /tmp/ixtunneld ./cmd/tunneld/
sudo /tmp/ixtunneld

# Verify a device is connected
curl http://localhost:33333/rsd
curl http://localhost:33333/rsd/<UDID>
```

---

## Package Structure

```
api/
├── device/              Unified device entry point (iOS version routing)
├── tunneld/             Tunnel daemon — WiFi (Bonjour) + USB discovery
│
├── lockdown/            USB lockdown pairing session (iOS < 17)
├── mux/                 usbmuxd + lockdown protocol types
│   ├── plist/           Plist-framed service connection
│   └── usb/             usbmuxd connection
│
├── tunnel/              CDTunnel + TUN interface
│   ├── rsd/             Remote Service Discovery (iOS 17+)
│   ├── xpc/             RemoteXPC over HTTP/2
│   └── h2c/             Bespoke HTTP/2 framing layer
│
├── remotepair/          CoreDevice pairing (SRP + ECDH + PSK)
│   └── tlspsk.go        Pure-Go TLS-PSK (TLS_PSK_WITH_AES_256_GCM_SHA384)
│
├── coredevice/          CoreDevice RSD/XPC services (iOS 17+)
│   ├── appservice/      App list / launch / kill
│   ├── configuration/   Dark mode / accessibility
│   ├── deviceinfo/      Device info / MobileGestalt
│   ├── hid/             Touch & keyboard injection
│   ├── location/        GPS simulation
│   ├── orientation/     Screen rotation
│   ├── pasteboard/      Clipboard read/write
│   └── screencapture/   Screenshot
│
├── dvt/                 Instruments/DVT protocol (iOS < 17)
│   ├── processctrl/     Launch / kill / signal
│   ├── deviceinfo/      Process list / directory read
│   ├── screenshot/      Screenshot via DVT
│   ├── networkmonitor/  Network connection events
│   ├── graphics/        GPU performance sampling
│   ├── conditioninducer/ Network/thermal simulation
│   ├── energy/          Energy monitor
│   ├── location/        GPS simulation
│   ├── notification/    App state notifications
│   ├── systemtap/       CPU/memory telemetry
│   └── applicationlisting/ App listing
│
├── diagnostics/         Reboot / shutdown / MobileGestalt / IORegistry
├── amfi/                Developer Mode toggle
├── installationproxy/   App install / uninstall / list
├── afc/                 Apple File Conduit (file system)
├── housearrest/         App sandbox file access
├── mounter/             DeveloperDiskImage mount/unmount
├── pcapd/               Network packet capture
├── ostrace/             Unified Log stream
├── notificationproxy/   Darwin notification post/observe
├── springboard/         Icon layout / wallpaper
├── misagent/            Provisioning profile management
├── heartbeat/           Connection keepalive
└── syslog/              Legacy device log stream

cmd/
├── tunneld/             tunneld entry point (run with sudo)
├── rsd/                 RSD debug — list services on a device
├── dvt/                 DVT debug
└── test/
    ├── test.go          Screenshot + app listing
    └── devicetool.go    Launch / kill / install / file transfer
```

---

## iOS Version Routing

| iOS | Connection path |
|-----|----------------|
| < 17 | USB lockdown (mTLS pairing) |
| ≥ 17, < 18.2 | tunneld → QUIC or TCP+PSK |
| ≥ 18.2 | tunneld → TCP+PSK only (QUIC removed) |

`device.New()` selects the correct path automatically:

```go
dev, err := device.New(device.Any)                          // any connected device
dev, err := device.New("00008130-001975122140001C")          // specific UDID
dev, err := device.NewFromTunnelD("00008130-001975122140001C") // tunneld only
```

---

## tunneld HTTP API

tunneld listens on `:33333` (HTTP) and `:33334` (pprof).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/rsd` | List all devices with active tunnels |
| GET | `/rsd/{udid}` | Get RSD address for a specific device |

Response example:

```json
{
  "Ret": 0,
  "Msg": "success",
  "Data": [
    {
      "Descriptor": { "ProductType": "iPhone16,2", "ProductVersion": "18.2" },
      "RSD": "[fd27::1]:59381"
    }
  ]
}
```

---

## Device API

```go
// Screenshot
data, err := dev.ScreenShot()
os.WriteFile("screen.png", data, 0644)

// Launch app
pid, err := dev.Launch("com.apple.mobilesafari", processctrl.LaunchContext{})

// Kill process
err = dev.Kill(pid)

// List processes
procs, err := dev.ListProcesses()

// List installed apps
apps, err := dev.ListApplications() // map[bundleID]*Application

// Install IPA
err = dev.Install("/path/to/app.ipa")

// Uninstall
err = dev.Uninstall("com.example.app")

// AFC file access
afcSvc, err := dev.AfcService()
items, err := afcSvc.List("DCIM/", true)

// App sandbox (HouseArrest)
haSvc, err := dev.HouseArrestService()
afcSvc, err = haSvc.AfcService("com.example.app")

// Port forward
err = dev.Forward(localPort, devicePort)

// Live syslog
err = dev.Logcat(os.Stdout)
```

---

## devicetool CLI

```bash
go run cmd/test/devicetool.go -command launch          -bundle com.apple.mobilesafari
go run cmd/test/devicetool.go -command launchAndReturn -bundle com.example.app
go run cmd/test/devicetool.go -command kill            -bundle com.example.app
go run cmd/test/devicetool.go -command install         -path /tmp/app.ipa
go run cmd/test/devicetool.go -command uninstall       -bundle com.example.app
go run cmd/test/devicetool.go -command pull  -bundle com.example.app -path /Documents/file.dat -path /tmp/file.dat
go run cmd/test/devicetool.go -command push  -bundle com.example.app -path /tmp/file.dat       -path /Documents/file.dat
go run cmd/test/devicetool.go -command remove -bundle com.example.app -path /Documents/file.dat
```

---

## Tests

```bash
# Unit tests (no device required)
go test ./api/remotepair/ -v -run TestPSKConn -timeout 20s
go test ./api/tunnel/xpc/ -v

# All unit tests
go test ./api/remotepair/ ./api/tunnel/xpc/

# Verify pure Go (no CGo)
CGO_ENABLED=0 go build ./api/remotepair/ && echo "ok"
```

---

## Pair Records

Pair records are stored at `~/.j3idevice/RP_<UDID>.plist`. Delete to force re-pairing:

```bash
rm ~/.j3idevice/RP_<UDID>.plist
```
