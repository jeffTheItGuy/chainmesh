//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestInvalidAPIKeyRejected(t *testing.T) {
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

func TestMissingAdminSecretRejected(t *testing.T) {
	payload := map[string]any{
		"name": "test",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/tenants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Admin-Secret

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for missing admin secret, got %d", resp.StatusCode)
	}
}

func TestSSRFProtection(t *testing.T) {
	payload := map[string]any{
		"name":            "bad",
		"rpc_endpoint_1":  "http://127.0.0.1:8545",
		"rpc_endpoint_2":  "https://rpc.sepolia.org",
		"enabled":         true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/blockchain", bytes.NewReader(body))
	req.Header.Set("X-Admin-Secret", adminSecret())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for SSRF attempt, got %d", resp.StatusCode)
	}
}

func TestPrivateIPBlocked(t *testing.T) {
	payload := map[string]any{
		"name":            "bad-private",
		"rpc_endpoint_1":  "http://192.168.1.1:8545",
		"rpc_endpoint_2":  "https://rpc.sepolia.org",
		"enabled":         true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/blockchain", bytes.NewReader(body))
	req.Header.Set("X-Admin-Secret", adminSecret())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for private IP, got %d", resp.StatusCode)
	}
}
