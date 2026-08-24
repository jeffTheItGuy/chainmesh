# Integration Testing

Integration tests verify that ChainMesh services work together correctly — Gateway ↔ Redis ↔ Postgres, Admin API ↔ Database, and the full request lifecycle.

---

## Table of Contents

1. [Test Environment](#test-environment)
2. [Test Categories](#test-categories)
3. [CI Integration](#ci-integration)
4. [Troubleshooting](#troubleshooting)

---

## Test Environment

Integration tests run against a real Docker Compose stack:

```bash
# Start the full stack
docker compose up -d

# Wait for health checks
sleep 15

# Run integration tests
cd tests/go
go test ./... -tags=integration
```

---

## Test Categories

### 1. API Contract Tests

Verify that the Admin API and Gateway accept and return the expected shapes.

**File:** `tests/go/admin_api_test.go`

```go
//go:build integration

package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
    "testing"
)

func TestCreateTenant(t *testing.T) {
    adminSecret := os.Getenv("ADMIN_SECRET")
    payload := map[string]any{
        "name":       "Integration Test Tenant",
        "quota_rpm":  100,
        "quota_rps":  10,
        "quota_daily": 10000,
        "plan":       "free",
    }
    body, _ := json.Marshal(payload)

    req, _ := http.NewRequest("POST", "http://localhost:8081/tenants", bytes.NewReader(body))
    req.Header.Set("X-Admin-Secret", adminSecret)
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
    json.NewDecoder(resp.Body).Decode(&result)
    if result["api_key"] == "" {
        t.Fatal("expected api_key in response")
    }
}
```

### 2. Gateway Proxy Tests

Verify end-to-end RPC proxying with auth, caching, and rate limiting.

**File:** `tests/go/gateway_proxy_test.go`

```go
//go:build integration

func TestGatewayRPCWithAuth(t *testing.T) {
    apiKey := createTestTenant(t) // helper that calls Admin API

    payload := map[string]any{
        "jsonrpc": "2.0",
        "method":  "eth_chainId",
        "params":  []any{},
        "id":      1,
    }
    body, _ := json.Marshal(payload)

    req, _ := http.NewRequest("POST", "http://localhost:8080/v1/", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }

    var result map[string]any
    json.NewDecoder(resp.Body).Decode(&result)
    if result["result"] == nil {
        t.Fatal("expected result in JSON-RPC response")
    }
}
```

### 3. Rate Limit Integration Tests

Verify Redis-backed rate limiting works across the real stack.

```go
func TestRateLimitEnforced(t *testing.T) {
    apiKey := createTestTenantWithQuota(t, 0, 2, 1000) // 2 RPM

    // First 2 requests should succeed
    for i := 0; i < 2; i++ {
        resp := callGateway(t, apiKey, "eth_chainId")
        if resp.StatusCode != http.StatusOK {
            t.Fatalf("request %d: expected 200, got %d", i+1, resp.StatusCode)
        }
    }

    // Third request should be rate limited
    resp := callGateway(t, apiKey, "eth_chainId")
    if resp.StatusCode != http.StatusTooManyRequests {
        t.Fatalf("expected 429, got %d", resp.StatusCode)
    }

    retryAfter := resp.Header.Get("Retry-After")
    if retryAfter == "" {
        t.Fatal("expected Retry-After header on 429")
    }
}
```

### 4. Cache Integration Tests

Verify Redis caching behavior for cacheable methods.

```go
func TestCacheHitHeader(t *testing.T) {
    apiKey := createTestTenant(t)

    // First call — cache miss
    resp1 := callGateway(t, apiKey, "eth_chainId")
    if resp1.Header.Get("X-Cache") != "MISS" {
        t.Fatalf("expected X-Cache: MISS on first call, got %s", resp1.Header.Get("X-Cache"))
    }

    // Second call — cache hit
    resp2 := callGateway(t, apiKey, "eth_chainId")
    if resp2.Header.Get("X-Cache") != "HIT" {
        t.Fatalf("expected X-Cache: HIT on second call, got %s", resp2.Header.Get("X-Cache"))
    }
}
```

### 5. Database Migration Tests

Verify migrations apply cleanly and roll back correctly.

```bash
# In CI or locally with a fresh Postgres container
docker run -d --name chainmesh-test-pg -e POSTGRES_PASSWORD=test postgres:15-alpine

# Apply all migrations
for f in backend/database/migrations/*.up.sql; do
    docker exec -i chainmesh-test-pg psql -U postgres -d postgres < "$f"
done

# Verify tables exist
docker exec chainmesh-test-pg psql -U postgres -d postgres -c "\dt"

# Rollback (if down migrations exist)
for f in $(ls backend/database/migrations/*.down.sql | sort -r); do
    docker exec -i chainmesh-test-pg psql -U postgres -d postgres < "$f"
done
```

---

## CI Integration

The GitHub Actions workflow:

1. Starts Postgres + Redis services
2. Builds and starts Gateway + Admin API
3. Waits for `/health` endpoints
4. Runs `go test ./tests/go/... -tags=integration`
5. Captures service logs on failure

```yaml
# .github/workflows/test.yml (integration job excerpt)
- name: Start services
  run: docker compose up -d

- name: Wait for health
  run: |
    for i in {1..30}; do
      curl -sf http://localhost:8081/health && break
      sleep 2
    done

- name: Run integration tests
  run: cd tests/go && go test ./... -tags=integration -v
  env:
    ADMIN_SECRET: test-secret
```

---

## Troubleshooting

### "connection refused" to Gateway/Admin

Services need time to start. Add a health-check wait loop or use `docker compose up --wait` (Compose v2.20+).

### Redis rate limit tests flaky

Rate limit windows are time-based. Use generous margins (e.g., test 2 RPM with a 3-second window) or mock the clock if possible.

### Postgres state bleeding between tests

Use `t.Parallel()` carefully. Either:
- Run tests sequentially, or
- Create a unique tenant per test (recommended), or
- Use test transactions that roll back

---

## Related Documents

- [Unit Testing](unit-testing.md) — Isolated component tests
- [Post-Production Testing](post-production-testing.md) — Validation against live deployments
- [Testing Strategy](README.md) — Overview of the test pyramid
- [Developer Setup](../developers/setup.md) — Local Docker Compose configuration
