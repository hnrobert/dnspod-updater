//go:build linux

package ipdetect

import (
	"fmt"
	"strings"

	"github.com/mdlayher/wifi"
)

// wifiIfaceForSSID finds a WiFi interface which is currently associated with the
// given SSID. It returns the interface name and the actual SSID.
func wifiIfaceForSSID(targetSSID string) (string, string, error) {
	targetSSID = strings.TrimSpace(targetSSID)
	if targetSSID == "" {
		return "", "", fmt.Errorf("%w: empty ssid", ErrWiFiSSIDUnavailable)
	}

	c, err := wifi.New()
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrWiFiSSIDUnavailable, err)
	}
	defer c.Close()

	ifis, err := c.Interfaces()
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrWiFiSSIDUnavailable, err)
	}

	foundAny := false
	var foundSSIDs []string

	for _, ifi := range ifis {
		bss, err := c.BSS(ifi)
		if err != nil {
			continue
		}
		ssid := strings.TrimSpace(string(bss.SSID))
		if ssid == "" {
			continue
		}
		foundAny = true
		foundSSIDs = append(foundSSIDs, fmt.Sprintf("%s=%q", ifi.Name, ssid))
		if ssid == targetSSID {
			return ifi.Name, ssid, nil
		}
	}

	if !foundAny {
		return "", "", fmt.Errorf("%w: no associated wifi interface found", ErrWiFiSSIDUnavailable)
	}
	return "", "", fmt.Errorf("%w: want %q, found %s", ErrWiFiSSIDNotMatched, targetSSID, strings.Join(foundSSIDs, ", "))
}
