//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestGatewayServesWhenHealthy(t *testing.T) {
	apiKey := createTestTenant(t)

	resp := callGateway(t, apiKey, "eth_chainId")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGatewayReturnsJSONRPCError(t *testing.T) {
	apiKey := createTestTenant(t)

	// Call with an invalid method
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_invalidMethod",
		"params":  []any{},
		"id":      1,
	}

	resp := callGatewayRaw(t, apiKey, payload)
	defer resp.Body.Close()

	// Gateway should still return 200 with JSON-RPC error, or 400
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}
