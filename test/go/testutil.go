//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func gatewayURL() string {
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return u
	}
	return "http://localhost:8080/v1/"
}

func adminURL() string {
	if u := os.Getenv("ADMIN_URL"); u != "" {
		return u
	}
	return "http://localhost:8081"
}

func adminSecret() string {
	if s := os.Getenv("ADMIN_SECRET"); s != "" {
		return s
	}
	return "devsecret"
}

func createTestTenant(t *testing.T, quotas ...int) string {
	t.Helper()

	// Default quotas: rpm=100, rps=10, daily=10000
	rpm, rps, daily := 100, 10, 10000
	if len(quotas) >= 1 {
		rpm = quotas[0]
	}
	if len(quotas) >= 2 {
		rps = quotas[1]
	}
	if len(quotas) >= 3 {
		daily = quotas[2]
	}

	payload := map[string]any{
		"name":        fmt.Sprintf("Test Tenant %d", time.Now().UnixNano()),
		"quota_rpm":   rpm,
		"quota_rps":   rps,
		"quota_daily": daily,
		"plan":        "free",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/tenants", bytes.NewReader(body))
	req.Header.Set("X-Admin-Secret", adminSecret())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create tenant request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating tenant, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode tenant response: %v", err)
	}

	apiKey, ok := result["api_key"].(string)
	if !ok || apiKey == "" {
		t.Fatal("expected api_key in tenant response")
	}
	return apiKey
}

func createTestTenantWithQuota(t *testing.T, rpm, rps, daily int) string {
	t.Helper()
	return createTestTenant(t, rpm, rps, daily)
}

func callGateway(t *testing.T, apiKey, method string) *http.Response {
	t.Helper()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  []any{},
		"id":      1,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", gatewayURL(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	return resp
}

func callGatewayRaw(t *testing.T, apiKey string, payload map[string]any) *http.Response {
	t.Helper()

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", gatewayURL(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	return resp
}
