package ipdetect

import (
	"errors"
	"fmt"
	"net"
	"runtime"
)

type Options struct {
	PreferredIface string
	// Method: "" or "auto" (default), "route", "udp", "iface"
	Method string
}

type Detector struct {
	opt Options
}

func NewDetector(opt Options) *Detector {
	if opt.Method == "" {
		opt.Method = "auto"
	}
	return &Detector{opt: opt}
}

func (d *Detector) DetectIPv4() (net.IP, string, error) {
	// If user pins iface, use it first.
	if d.opt.PreferredIface != "" {
		ip, err := ipv4FromIface(d.opt.PreferredIface)
		if err == nil {
			return ip, "iface:" + d.opt.PreferredIface, nil
		}
		if d.opt.Method == "iface" {
			return nil, "", err
		}
	}

	switch d.opt.Method {
	case "auto":
		// Prefer route-based on Linux; otherwise UDP fallback.
		if runtime.GOOS == "linux" {
			if ip, ifname, err := ipv4FromDefaultRouteLinux(); err == nil {
				return ip, "route:" + ifname, nil
			}
		}
		if ip, err := ipv4FromUDP(); err == nil {
			return ip, "udp", nil
		}
		if ip, ifname, err := ipv4FromAnyNonLoopback(); err == nil {
			return ip, "any:" + ifname, nil
		}
		return nil, "", errors.New("failed to detect IPv4")
	case "route":
		if runtime.GOOS != "linux" {
			return nil, "", fmt.Errorf("method=route requires linux (current %s)", runtime.GOOS)
		}
		ip, ifname, err := ipv4FromDefaultRouteLinux()
		if err != nil {
			return nil, "", err
		}
		return ip, "route:" + ifname, nil
	case "udp":
		ip, err := ipv4FromUDP()
		if err != nil {
			return nil, "", err
		}
		return ip, "udp", nil
	case "iface":
		if d.opt.PreferredIface == "" {
			return nil, "", errors.New("method=iface requires IP_PREFERRED_IFACE")
		}
		ip, err := ipv4FromIface(d.opt.PreferredIface)
		if err != nil {
			return nil, "", err
		}
		return ip, "iface:" + d.opt.PreferredIface, nil
	default:
		return nil, "", fmt.Errorf("unknown IP_DETECT_METHOD: %q", d.opt.Method)
	}
}

func ipv4FromIface(ifname string) (net.IP, error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		ip := addrToIPv4(a)
		if ip == nil {
			continue
		}
		if isUsableIPv4(ip) {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("no usable IPv4 found on iface %s", ifname)
}

func ipv4FromAnyNonLoopback() (net.IP, string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := addrToIPv4(a)
			if ip == nil {
				continue
			}
			if isUsableIPv4(ip) {
				return ip, iface.Name, nil
			}
		}
	}
	return nil, "", errors.New("no usable IPv4 found")
}

func addrToIPv4(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPNet:
		return v.IP.To4()
	case *net.IPAddr:
		return v.IP.To4()
	default:
		return nil
	}
}

func isUsableIPv4(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return false
	}
	// Filter link-local 169.254.0.0/16
	if ip[0] == 169 && ip[1] == 254 {
		return false
	}
	return ip.IsGlobalUnicast()
}
