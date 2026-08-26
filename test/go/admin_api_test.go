//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminHealth(t *testing.T) {
	resp, err := http.Get(adminURL() + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if result["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", result["status"])
	}
}

func TestCreateTenant(t *testing.T) {
	payload := map[string]any{
		"name":        "Integration Test Tenant",
		"quota_rpm":   100,
		"quota_rps":   10,
		"quota_daily": 10000,
		"plan":        "free",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/tenants", bytes.NewReader(body))
	req.Header.Set("X-Admin-Secret", adminSecret())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result["api_key"] == "" {
		t.Fatal("expected api_key in response")
	}
	if result["id"] == "" {
		t.Fatal("expected id in response")
	}
}

func TestCreateTenantMissingSecret(t *testing.T) {
	payload := map[string]any{
		"name":        "Should Fail",
		"quota_rpm":   100,
		"quota_rps":   10,
		"quota_daily": 10000,
		"plan":        "free",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/tenants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Intentionally omit X-Admin-Secret

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListTenants(t *testing.T) {
	req, _ := http.NewRequest("GET", adminURL()+"/tenants", nil)
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should be a slice
	if result == nil {
		t.Fatal("expected list of tenants")
	}
}

func TestGetTenantUsage(t *testing.T) {
	// Create a tenant first
	apiKey := createTestTenant(t)

	// Make a request through the gateway to generate usage
	resp := callGateway(t, apiKey, "eth_chainId")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gateway call failed: %d", resp.StatusCode)
	}

	// Small delay for async telemetry
	time.Sleep(200 * time.Millisecond)

	// Get usage via admin API
	req, _ := http.NewRequest("GET", adminURL()+"/tenants/usage", nil)
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("usage request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestAuditLogs(t *testing.T) {
	// Create a tenant to generate an audit log
	_ = createTestTenant(t)

	req, _ := http.NewRequest("GET", adminURL()+"/audit-logs?limit=5", nil)
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var logs []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		t.Fatalf("decode logs: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("expected at least one audit log entry")
	}
}

func TestBlocksEndpoint(t *testing.T) {
	req, _ := http.NewRequest("GET", adminURL()+"/blocks", nil)
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var blocks []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&blocks); err != nil {
		t.Fatalf("decode blocks: %v", err)
	}

	// Should return a slice (may be empty if ingestor hasn't run)
	if blocks == nil {
		t.Fatal("expected blocks slice, got nil")
	}
}

func TestNodeHealthEndpoint(t *testing.T) {
	req, _ := http.NewRequest("GET", adminURL()+"/health/nodes", nil)
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCreateTenantValidation(t *testing.T) {
	// Missing required field: name
	payload := map[string]any{
		"quota_rpm":   100,
		"quota_rps":   10,
		"quota_daily": 10000,
		"plan":        "free",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", adminURL()+"/tenants", bytes.NewReader(body))
	req.Header.Set("X-Admin-Secret", adminSecret())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}
}

func TestDeleteTenant(t *testing.T) {
	apiKey := createTestTenant(t)

	// First, list tenants to find the ID
	req, _ := http.NewRequest("GET", adminURL()+"/tenants", nil)
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list tenants failed: %v", err)
	}

	var tenants []map[string]any
	json.NewDecoder(resp.Body).Decode(&tenants)
	resp.Body.Close()

	var tenantID string
	for _, tenant := range tenants {
		if tenant["api_key"] == apiKey {
			tenantID = tenant["id"].(string)
			break
		}
	}

	if tenantID == "" {
		t.Fatal("could not find created tenant to delete")
	}

	// Delete the tenant
	req2, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/tenants/%s", adminURL(), tenantID), nil)
	req2.Header.Set("X-Admin-Secret", adminSecret())

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNoContent && resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 204 or 200 deleting tenant, got %d", resp2.StatusCode)
	}
}
