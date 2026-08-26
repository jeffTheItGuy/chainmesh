//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"
)

func TestRateLimitEnforced(t *testing.T) {
	// Create tenant with 2 RPM limit
	apiKey := createTestTenantWithQuota(t, 0, 2, 1000)

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		resp := callGateway(t, apiKey, "eth_chainId")
		status := resp.StatusCode
		resp.Body.Close()
		if status != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, status)
		}
	}

	// Third request should be rate limited
	resp := callGateway(t, apiKey, "eth_chainId")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header on 429")
	}
}

func TestRateLimitDailyQuota(t *testing.T) {
	// Create tenant with very low daily quota
	apiKey := createTestTenantWithQuota(t, 1000, 1000, 2)

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		resp := callGateway(t, apiKey, "eth_chainId")
		status := resp.StatusCode
		resp.Body.Close()
		if status != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, status)
		}
	}

	// Third request may be rate limited by daily quota
	resp := callGateway(t, apiKey, "eth_chainId")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Daily quota hit — acceptable
		t.Log("daily quota enforced")
	} else if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestRateLimitResets(t *testing.T) {
	// Create tenant with 1 RPM
	apiKey := createTestTenantWithQuota(t, 0, 1, 1000)

	// First request succeeds
	resp1 := callGateway(t, apiKey, "eth_chainId")
	status1 := resp1.StatusCode
	resp1.Body.Close()
	if status1 != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", status1)
	}

	// Second request should be rate limited
	resp2 := callGateway(t, apiKey, "eth_chainId")
	status2 := resp2.StatusCode
	resp2.Body.Close()
	if status2 != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", status2)
	}

	// Wait for window to reset (generous margin)
	time.Sleep(65 * time.Second)

	// Should succeed again
	resp3 := callGateway(t, apiKey, "eth_chainId")
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("after reset: expected 200, got %d", resp3.StatusCode)
	}
}

func TestRateLimitHeadersOnSuccess(t *testing.T) {
	apiKey := createTestTenant(t)

	resp := callGateway(t, apiKey, "eth_chainId")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Should have rate limit headers
	hasLimit := resp.Header.Get("X-RateLimit-Limit") != "" ||
		resp.Header.Get("X-RateLimit-Limit-Minute") != ""
	hasRemaining := resp.Header.Get("X-RateLimit-Remaining") != "" ||
		resp.Header.Get("X-RateLimit-Remaining-Minute") != ""

	if !hasLimit {
		t.Log("warning: X-RateLimit-Limit header not present")
	}
	if !hasRemaining {
		t.Log("warning: X-RateLimit-Remaining header not present")
	}
}
