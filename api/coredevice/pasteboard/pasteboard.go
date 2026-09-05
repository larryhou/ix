// Package pasteboard implements the com.apple.coredevice.pasteboardservice RSD service.
// This service uses a direct XPC message protocol (no CoreDevice envelope).
package pasteboard

import (
	"fmt"
	"github.com/larryhou/ix/api/coredevice"
	"github.com/larryhou/ix/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.pasteboardservice`

// PasteboardName constants.
const (
	PasteboardGeneral = `general`
)

// UTI type identifiers for common pasteboard item types.
const (
	UTIPlainText = `public.utf8-plain-text`
	UTIHTML      = `public.html`
	UTIURL       = `public.url`
	UTIImage     = `public.image`
)

// Item represents a single pasteboard item with one or more typed data representations.
type Item struct {
	// Types lists the UTI types available for this item.
	Types []string
	// Data maps each UTI type to its raw bytes.
	Data map[string][]byte
}

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// Pull reads all items from the named pasteboard.
// pasteboardName is typically PasteboardGeneral ("general").
func (s *Service) Pull(pasteboardName string) ([]*Item, error) {
	rsp, err := s.SendRecv(map[string]any{
		"command":        "PULL",
		"pasteboardName": pasteboardName,
		"dataPolicy":     map[string]any{"allResolved": map[string]any{}},
	})
	if err != nil {
		return nil, err
	}

	m, ok := rsp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("pasteboard: unexpected response type %T", rsp)
	}
	if cmd, _ := m["command"].(string); cmd != "PULL_REPLY" {
		return nil, fmt.Errorf("pasteboard: unexpected command %q", cmd)
	}

	pb, ok := m["pasteboard"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("pasteboard: missing pasteboard field")
	}

	return decodePasteboardItems(pb)
}

// Set writes one or more items to the named pasteboard, replacing its current contents.
func (s *Service) Set(pasteboardName string, items []*Item) error {
	encoded := encodeItems(items)
	rsp, err := s.SendRecv(map[string]any{
		"command":        "SET",
		"pasteboardName": pasteboardName,
		"items":          encoded,
		"sourceMetadata": nil,
	})
	if err != nil {
		return err
	}
	if m, ok := rsp.(map[string]any); ok {
		if cmd, _ := m["command"].(string); cmd != "SET_REPLY" {
			return fmt.Errorf("pasteboard: unexpected SET response command %q", cmd)
		}
	}
	return nil
}

// SetText is a convenience wrapper that writes a single plain-text item.
func (s *Service) SetText(text string) error {
	return s.Set(PasteboardGeneral, []*Item{
		{
			Types: []string{UTIPlainText},
			Data:  map[string][]byte{UTIPlainText: []byte(text)},
		},
	})
}

// decodePasteboardItems converts the XPC pasteboard dict into a slice of Items.
func decodePasteboardItems(pb map[string]any) ([]*Item, error) {
	rawItems, ok := pb["items"].([]any)
	if !ok {
		return nil, nil
	}

	items := make([]*Item, 0, len(rawItems))
	for _, ri := range rawItems {
		rm, ok := ri.(map[string]any)
		if !ok {
			continue
		}

		item := &Item{Data: map[string][]byte{}}

		// Collect type list.
		if ts, ok := rm["types"].([]any); ok {
			for _, t := range ts {
				if s, ok := t.(string); ok {
					item.Types = append(item.Types, s)
				}
			}
		}

		// Decode per-type data blobs.
		if dataMap, ok := rm["data"].(map[string]any); ok {
			for uti, v := range dataMap {
				switch vt := v.(type) {
				case map[string]any:
					// Immediate data: {"data": <bytes>}
					if b, ok := vt["data"].([]byte); ok {
						item.Data[uti] = b
					}
				case []byte:
					item.Data[uti] = vt
				}
			}
		}

		items = append(items, item)
	}
	return items, nil
}

// encodeItems converts a slice of Items into the XPC wire format expected by SET.
func encodeItems(items []*Item) []any {
	encoded := make([]any, 0, len(items))
	for _, item := range items {
		types := make([]any, 0, len(item.Types))
		for _, t := range item.Types {
			types = append(types, t)
		}

		dataMap := map[string]any{}
		for uti, b := range item.Data {
			dataMap[uti] = map[string]any{"data": b}
		}

		encoded = append(encoded, map[string]any{
			"types": types,
			"data":  dataMap,
		})
	}
	return encoded
}
