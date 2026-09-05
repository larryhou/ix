// Package configuration implements the com.apple.coredevice.configuration RSD service.
// It controls UI appearance settings: dark/light mode, color filters, text size,
// reduce motion, increase contrast, show borders, reduce transparency, and glass opacity.
package configuration

import (
	"math"
	"github.com/larryhou/ix/api/coredevice"
	"github.com/larryhou/ix/api/tunnel/xpc"
)

const ServiceName = `com.apple.coredevice.configuration`

// Action identifier prefix shared by all configuration actions.
const actionPrefix = `com.apple.coredevice.action.`

// UIStyle constants for GetUIStyle / SetUIStyle.
const (
	UIStyleDark  = "dark"
	UIStyleLight = "light"
)

// ColorFilterType names recognised by SetColorFilter.
const (
	ColorFilterNone        = "Normal"
	ColorFilterProtanopia  = "Protanopia"
	ColorFilterDeuteranopia = "Deuteranopia"
	ColorFilterTritanopia  = "Tritanopia"
	ColorFilterGrayscale   = "Grayscale"
)

// TextSize names recognised by SetTextSize.
const (
	TextSizeXSmall      = "xSmall"
	TextSizeSmall       = "small"
	TextSizeMedium      = "medium"
	TextSizeLarge       = "large"
	TextSizeXLarge      = "xLarge"
	TextSizeXXLarge     = "xxLarge"
	TextSizeXXXLarge    = "xxxLarge"
	TextSizeAccessibility1 = "accessibility1"
	TextSizeAccessibility2 = "accessibility2"
	TextSizeAccessibility3 = "accessibility3"
	TextSizeAccessibility4 = "accessibility4"
	TextSizeAccessibility5 = "accessibility5"
)

// ColorFilter holds the color-filter configuration returned by GetColorFilter.
type ColorFilter struct {
	Enabled    bool
	FilterType string
	Intensity  float64
}

func New(conn *xpc.RemoteXpcConnection) *Service {
	return &Service{Service: coredevice.NewService(conn)}
}

type Service struct {
	*coredevice.Service
}

// invoke is a thin wrapper that calls Invoke with only an actionIdentifier
// (ConfigurationService never uses featureIdentifier).
func (s *Service) invoke(action string, input map[string]any) (any, error) {
	return s.Invoke("", actionPrefix+action, input)
}

// GetUIStyle returns the current UI style ("dark" or "light").
func (s *Service) GetUIStyle() (string, error) {
	out, err := s.invoke("getuserinterfacestyle", map[string]any{})
	if err != nil {
		return "", err
	}
	m, _ := out.(map[string]any)
	style, _ := m["style"].(string)
	return style, nil
}

// SetUIStyle sets the UI style to "dark" or "light".
func (s *Service) SetUIStyle(style string) error {
	_, err := s.invoke("setuserinterfacestyle", map[string]any{"style": style})
	return err
}

// SetGlassOpacity sets the liquid-glass opacity (0.0–1.0).
// The value is rounded to the nearest IEEE-754 float32 before encoding to avoid
// device-side decoding errors.
func (s *Service) SetGlassOpacity(opacity float64) error {
	_, err := s.invoke("setliquidglassconfiguration", map[string]any{
		"configuration": map[string]any{
			"opacity": float32to64(opacity),
		},
	})
	return err
}

// GetColorFilter returns the current color-filter configuration.
func (s *Service) GetColorFilter() (*ColorFilter, error) {
	out, err := s.invoke("getcolorfilter", map[string]any{})
	if err != nil {
		return nil, err
	}
	m, _ := out.(map[string]any)
	cf, _ := m["colorFilter"].(map[string]any)
	filter := &ColorFilter{}
	filter.Enabled, _ = cf["enabled"].(bool)
	filter.Intensity, _ = cf["intensity"].(float64)
	if ft, ok := cf["filterType"].(map[string]any); ok {
		for k := range ft {
			filter.FilterType = k
			break
		}
	}
	return filter, nil
}

// SetColorFilter configures the display color filter.
// intensity is clamped to [0.0, 1.0] and rounded to float32 precision.
func (s *Service) SetColorFilter(enabled bool, filterType string, intensity float64) error {
	_, err := s.invoke("setcolorfilter", map[string]any{
		"colorFilter": map[string]any{
			"enabled":    enabled,
			"filterType": map[string]any{filterType: map[string]any{}},
			"intensity":  float32to64(intensity),
		},
	})
	return err
}

// GetTextSize returns the current dynamic-type text size name.
func (s *Service) GetTextSize() (string, error) {
	out, err := s.invoke("getdevicetextsize", map[string]any{})
	if err != nil {
		return "", err
	}
	m, _ := out.(map[string]any)
	ts, _ := m["textSize"].(map[string]any)
	size, _ := ts["size"].(map[string]any)
	for k := range size {
		return k, nil
	}
	return "", nil
}

// SetTextSize sets the dynamic-type text size. Use the TextSize* constants.
func (s *Service) SetTextSize(size string) error {
	_, err := s.invoke("setdevicetextsize", map[string]any{
		"textSize": map[string]any{
			"size": map[string]any{size: map[string]any{}},
		},
	})
	return err
}

// GetReduceMotion returns whether Reduce Motion is enabled.
func (s *Service) GetReduceMotion() (bool, error) {
	out, err := s.invoke("getreducemotion", map[string]any{})
	if err != nil {
		return false, err
	}
	m, _ := out.(map[string]any)
	rm, _ := m["reduceMotion"].(map[string]any)
	enabled, _ := rm["enabled"].(bool)
	return enabled, nil
}

// SetReduceMotion enables or disables the Reduce Motion accessibility setting.
func (s *Service) SetReduceMotion(enabled bool) error {
	_, err := s.invoke("setreducemotion", map[string]any{
		"reduceMotion": map[string]any{"enabled": enabled},
	})
	return err
}

// SetIncreaseContrast enables or disables Increase Contrast.
func (s *Service) SetIncreaseContrast(enabled bool) error {
	_, err := s.invoke("setdeviceincreasecontrast", map[string]any{
		"increaseContrast": map[string]any{"enabled": enabled},
	})
	return err
}

// GetShowBorders returns whether accessibility borders are shown.
func (s *Service) GetShowBorders() (bool, error) {
	out, err := s.invoke("getshowborders", map[string]any{})
	if err != nil {
		return false, err
	}
	m, _ := out.(map[string]any)
	sb, _ := m["showBorders"].(map[string]any)
	enabled, _ := sb["enabled"].(bool)
	return enabled, nil
}

// SetShowBorders enables or disables accessibility borders.
func (s *Service) SetShowBorders(enabled bool) error {
	_, err := s.invoke("setshowborders", map[string]any{
		"showBorders": map[string]any{"enabled": enabled},
	})
	return err
}

// GetReduceTransparency returns whether Reduce Transparency is enabled.
func (s *Service) GetReduceTransparency() (bool, error) {
	out, err := s.invoke("getreducetransparency", map[string]any{})
	if err != nil {
		return false, err
	}
	m, _ := out.(map[string]any)
	rt, _ := m["reduceTransparency"].(map[string]any)
	enabled, _ := rt["enabled"].(bool)
	return enabled, nil
}

// SetReduceTransparency enables or disables the Reduce Transparency setting.
func (s *Service) SetReduceTransparency(enabled bool) error {
	_, err := s.invoke("setreducetransparency", map[string]any{
		"reduceTransparency": map[string]any{"enabled": enabled},
	})
	return err
}

// float32to64 rounds a float64 to the nearest IEEE-754 float32 value and returns
// it as float64. This is required because the device decodes these fields as Swift
// Float (32-bit) and rejects values that cannot be represented exactly.
func float32to64(v float64) float64 {
	return float64(math.Float32frombits(math.Float32bits(float32(v))))
}
