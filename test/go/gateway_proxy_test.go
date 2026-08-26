//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGatewayHealth(t *testing.T) {
	resp, err := http.Get(gatewayURL())
	if err != nil {
		t.Fatalf("gateway health request failed: %v", err)
	}
	defer resp.Body.Close()

	// Gateway root may return 404 or a health response depending on config
	// We just verify it responds
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestGatewayRPCWithAuth(t *testing.T) {
	apiKey := createTestTenant(t)

	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_chainId",
		"params":  []any{},
		"id":      1,
	}

	resp := callGatewayRaw(t, apiKey, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["result"] == nil && result["error"] == nil {
		t.Fatal("expected result or error in JSON-RPC response")
	}
}

func TestGatewayRPCWithoutAuth(t *testing.T) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_chainId",
		"params":  []any{},
		"id":      1,
	}

	resp := callGatewayRaw(t, "", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}
}

func TestGatewayRPCInvalidKey(t *testing.T) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_chainId",
		"params":  []any{},
		"id":      1,
	}

	resp := callGatewayRaw(t, "bm_live_INVALIDKEY0000000000000000", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid key, got %d", resp.StatusCode)
	}
}

func TestGatewayRPCMethods(t *testing.T) {
	apiKey := createTestTenant(t)

	methods := []string{
		"eth_chainId",
		"eth_blockNumber",
		"net_version",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			resp := callGateway(t, apiKey, method)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", method, resp.StatusCode)
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode %s response: %v", method, err)
			}

			if result["jsonrpc"] != "2.0" {
				t.Fatalf("expected jsonrpc 2.0, got %v", result["jsonrpc"])
			}
		})
	}
}

func TestGatewayBatchRequest(t *testing.T) {
	apiKey := createTestTenant(t)

	batch := []map[string]any{
		{"jsonrpc": "2.0", "method": "eth_chainId", "params": []any{}, "id": 1},
		{"jsonrpc": "2.0", "method": "net_version", "params": []any{}, "id": 2},
	}

	resp := callGatewayRaw(t, apiKey, batch)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var results []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		// Some gateways return a single object for batch
		var single map[string]any
		if err2 := json.NewDecoder(resp.Body).Decode(&single); err2 != nil {
			t.Fatalf("decode batch response: %v", err)
		}
		return
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 responses in batch, got %d", len(results))
	}
}

func TestGatewayRateLimitHeaders(t *testing.T) {
	apiKey := createTestTenant(t)

	resp := callGateway(t, apiKey, "eth_chainId")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Check for rate limit headers
	if resp.Header.Get("X-RateLimit-Remaining-Minute") == "" &&
		resp.Header.Get("X-RateLimit-Remaining") == "" {
		t.Fatal("expected rate limit headers in response")
	}
}
