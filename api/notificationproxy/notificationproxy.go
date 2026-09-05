package notificationproxy

import (
	"github.com/larryhou/ix/api/mux/plist"
	"net"
)

const (
	// ServiceName is the TLS-secured notification proxy (requires pairing).
	ServiceName = `com.apple.mobile.notification_proxy`
	// ServiceNameInsecure is the plain-text variant (no TLS, limited notifications).
	ServiceNameInsecure = `com.apple.mobile.insecure_notification_proxy`
	// ServiceNameRSD is the RSD shim variant over a CoreDevice tunnel.
	ServiceNameRSD = `com.apple.mobile.notification_proxy.shim.remote`
	// ServiceNameInsecureRSD is the insecure RSD shim variant.
	ServiceNameInsecureRSD = `com.apple.mobile.insecure_notification_proxy.shim.remote`
)

// Well-known notification names posted by the device.
const (
	NotificationSyncWillStart         = `com.apple.itunes-mobdev.syncWillStart`
	NotificationSyncDidStart          = `com.apple.itunes-mobdev.syncDidStart`
	NotificationSyncDidFinish         = `com.apple.itunes-mobdev.syncDidFinish`
	NotificationSyncCancelRequest     = `com.apple.itunes-mobdev.syncCancelRequest`
	NotificationSpringBoardIconLayout = `com.apple.springboard.layoutChanged`
	NotificationBackupDomainChanged   = `com.apple.mobile.application_installed`
	NotificationDataClassChanged      = `com.apple.mobile.data_class_changed`
)

func New(conn net.Conn) *Service {
	return &Service{Connection: plist.NewConnection(conn)}
}

type Service struct {
	*plist.Connection
}

// Post sends a Darwin notification to the device. No response is expected.
func (x *Service) Post(name string) error {
	return x.Send(map[string]any{
		"Command": "PostNotification",
		"Name":    name,
	})
}

// Observe registers interest in a named Darwin notification.
// After calling this, incoming notifications can be read with Recv.
// No response is expected from the device.
func (x *Service) Observe(name string) error {
	return x.Send(map[string]any{
		"Command": "ObserveNotification",
		"Name":    name,
	})
}

// Notification holds an incoming notification relayed from the device.
type Notification struct {
	// Name is the Darwin notification name, e.g. "com.apple.springboard.layoutChanged".
	Name string
}

// Recv blocks until the device sends a RelayNotification message.
// Call Observe for every notification name of interest before entering the receive loop.
func (x *Service) Recv() (*Notification, error) {
	msg := map[string]any{}
	if err := x.Connection.Recv(&msg); err != nil {
		return nil, err
	}
	name, _ := msg["Name"].(string)
	return &Notification{Name: name}, nil
}
