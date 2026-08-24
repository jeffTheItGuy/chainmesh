package util

import (
	"testing"
)

func TestValidateRPCEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{"public_https", "https://ethereum-rpc.publicnode.com", false},
		{"public_http_port", "http://1.1.1.1:8545", false},
		{"loopback_ipv4", "http://127.0.0.1:8545", true},
		{"loopback_ipv6", "http://[::1]:8545", true},
		{"private_10", "http://10.0.0.1:8545", true},
		{"private_172", "http://172.16.0.1:8545", true},
		{"private_192", "http://192.168.1.1:8545", true},
		{"link_local", "http://169.254.1.1:8545", true},
		{"multicast", "http://224.0.0.1:8545", true},
		{"unspecified", "http://0.0.0.0:8545", true},
		{"localhost", "http://localhost:8545", true},
		{"localhost_ipv6_long", "http://[0:0:0:0:0:0:0:1]:8545", true},
		{"bad_scheme_ftp", "ftp://example.com", true},
		{"bad_scheme_file", "file:///etc/passwd", true},
		{"missing_host", "http://", true},
		{"url_encoded_dot", "http://127.0.0.1%2e1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRPCEndpoint(tt.endpoint)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateRPCEndpoint(%q) expected error, got nil", tt.endpoint)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateRPCEndpoint(%q) unexpected error: %v", tt.endpoint, err)
			}
		})
	}
}
