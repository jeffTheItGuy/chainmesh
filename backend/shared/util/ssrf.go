package util

import (
	"fmt"
	"net"
	"net/url"
)

// ValidateRPCEndpoint ensures an RPC URL is safe to dial from the server.
// It blocks non-http(s) schemes and resolves hostnames to confirm they do
// not point to restricted IP ranges.
func ValidateRPCEndpoint(endpoint string) error {
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

	// If the host is already an IP literal, check it directly.
	if ip := net.ParseIP(host); ip != nil {
		if isRestrictedIP(ip) {
			return fmt.Errorf("restricted ip address")
		}
		return nil
	}

	// Resolve hostname and validate all returned IPs.
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
