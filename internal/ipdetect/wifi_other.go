//go:build !linux

package ipdetect

import "fmt"

func wifiIfaceForSSID(targetSSID string) (string, string, error) {
	return "", "", fmt.Errorf("%w: WIFI_SSID requires linux", ErrWiFiSSIDUnavailable)
}
