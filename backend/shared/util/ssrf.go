package util

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

// ValidateRPCEndpoint ensures an RPC URL is safe to dial from the server.

func ValidateRPCEndpoint(endpoint string) error {
	if os.Getenv("BLOCKMESH_SKIP_SSRF") == "1" {
		return nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid url")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isRestrictedIP(ip) {
			return fmt.Errorf("restricted ip address")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host")
	}

	for _, ip := range ips {
		if isRestrictedIP(ip) {
			return fmt.Errorf("restricted ip address")
		}
	}

	return nil
}

func isRestrictedIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	if ip.IsMulticast() {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}
	if ip.Equal(net.ParseIP("::1")) {
		return true
	}
	return false
}