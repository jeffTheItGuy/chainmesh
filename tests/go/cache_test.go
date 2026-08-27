//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCacheHitHeader(t *testing.T) {
	apiKey := createTestTenant(t)

	// The cache is shared per-network (not per-tenant), and earlier test
	// files have already warmed common methods like eth_chainId. To get a
	// guaranteed-cold key we use a cacheable method (eth_getBalance, 30s TTL)
	// with a unique random address.
	addr := fmt.Sprintf("0x%040x", time.Now().UnixNano())
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_getBalance",
		"params":  []any{addr, "latest"},
		"id":      1,
	}

	// First call — must be a cache miss
	resp1 := callGatewayRaw(t, apiKey, payload)
	status1 := resp1.StatusCode
	cache1 := resp1.Header.Get("X-Cache")
	resp1.Body.Close()
	if status1 != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d", status1)
	}
	if cache1 != "MISS" {
		t.Fatalf("expected X-Cache: MISS on first call, got %q", cache1)
	}

	// Second call (same params) — must be a cache hit
	resp2 := callGatewayRaw(t, apiKey, payload)
	status2 := resp2.StatusCode
	cache2 := resp2.Header.Get("X-Cache")
	resp2.Body.Close()
	if status2 != http.StatusOK {
		t.Fatalf("second call: expected 200, got %d", status2)
	}
	if cache2 != "HIT" {
		t.Fatalf("expected X-Cache: HIT on second call, got %q", cache2)
	}
}

func TestCacheNonCacheableMethod(t *testing.T) {
	apiKey := createTestTenant(t)

	// eth_blockNumber may not be cached (or may be)
	resp1 := callGateway(t, apiKey, "eth_blockNumber")
	cache1 := resp1.Header.Get("X-Cache")
	resp1.Body.Close()

	resp2 := callGateway(t, apiKey, "eth_blockNumber")
	cache2 := resp2.Header.Get("X-Cache")
	resp2.Body.Close()

	// We just verify the gateway responds; caching behavior varies by method
	t.Logf("first call X-Cache: %s, second call X-Cache: %s", cache1, cache2)
}

func TestCacheDoesNotInterfereWithAuth(t *testing.T) {
	apiKey1 := createTestTenant(t)
	apiKey2 := createTestTenant(t)

	// Call with first key
	resp1 := callGateway(t, apiKey1, "eth_chainId")
	status1 := resp1.StatusCode
	resp1.Body.Close()
	if status1 != http.StatusOK {
		t.Fatalf("first key: expected 200, got %d", status1)
	}

	// Call with second key — should not get first key's cached response
	resp2 := callGateway(t, apiKey2, "eth_chainId")
	status2 := resp2.StatusCode
	resp2.Body.Close()
	if status2 != http.StatusOK {
		t.Fatalf("second key: expected 200, got %d", status2)
	}
}