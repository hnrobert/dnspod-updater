package ipdetect

import (
	"errors"
	"net"
	"time"
)

func ipv4FromUDP() (net.IP, error) {
	// No packets need to be sent; Dial picks a source IP.
	d := net.Dialer{Timeout: 2 * time.Second}

	// Try a couple of public IPs; either should pick the default egress interface.
	for _, addr := range []string{"8.8.8.8:80", "1.1.1.1:80"} {
		c, err := d.Dial("udp", addr)
		if err != nil {
			continue
		}
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		local := c.LocalAddr()
		_ = c.Close()

		u, ok := local.(*net.UDPAddr)
		if !ok {
			continue
		}
		ip := u.IP.To4()
		if isUsableIPv4(ip) {
			return ip, nil
		}
	}
	return nil, errors.New("udp source-ip detection failed")
}
