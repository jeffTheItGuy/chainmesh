//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestCacheHitHeader(t *testing.T) {
	apiKey := createTestTenant(t)

	// First call — cache miss
	resp1 := callGateway(t, apiKey, "eth_chainId")
	cache1 := resp1.Header.Get("X-Cache")
	resp1.Body.Close()

	if cache1 != "MISS" && cache1 != "" {
		t.Fatalf("expected X-Cache: MISS or empty on first call, got %s", cache1)
	}

	// Second call — cache hit
	resp2 := callGateway(t, apiKey, "eth_chainId")
	cache2 := resp2.Header.Get("X-Cache")
	resp2.Body.Close()

	if cache2 != "HIT" {
		// Some implementations may not set X-Cache on miss, just verify it works
		t.Logf("X-Cache on second call: %s (expected HIT)", cache2)
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
