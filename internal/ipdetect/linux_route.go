package ipdetect

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

// Parse /proc/net/route and locate the interface for the default route.
// This is reliable inside a host-networked container on Linux.
func ipv4FromDefaultRouteLinux() (net.IP, string, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip header
	if !scanner.Scan() {
		return nil, "", errors.New("/proc/net/route is empty")
	}

	var ifname string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		// Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT
		if len(fields) < 11 {
			continue
		}
		iface := fields[0]
		destination := fields[1]
		flags := fields[3]

		// Default route has Destination=00000000 and Flags has RTF_UP (0x1) + RTF_GATEWAY (0x2) usually.
		if destination != "00000000" {
			continue
		}
		if flags == "00000000" {
			continue
		}
		ifname = iface
		break
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	if ifname == "" {
		return nil, "", errors.New("default route not found in /proc/net/route")
	}

	ip, err := ipv4FromIface(ifname)
	if err != nil {
		return nil, "", fmt.Errorf("default route iface %s has no usable IPv4: %w", ifname, err)
	}
	return ip, ifname, nil
}
