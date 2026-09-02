<!-- integration-testing.md -->
# Integration Testing

Integration tests verify that ChainMesh services work together correctly — Gateway, Admin API, Redis, and Postgres.

The integration suite lives in:

```text
tests/go/
```

All integration tests use the Go build tag:

```go
//go:build integration
```

---

## Running Integration Tests

Integration tests require a running Gateway/Admin environment with Postgres and Redis available.

Run the integration suite through Docker Compose using Make:

```bash
make test-integration
```

This command starts the Gateway/Admin stack, runs the integration tests in `tests/go/`, and tears the stack down.

View the last integration summary:

```bash
make test-integration-logs
```

Tear down the integration environment:

```bash
make test-integration-down
```

The runner writes logs to:

```text
test-results/integration-full.log
test-results/integration-summary.log
```

---

## Environment Variables

The integration tests use the following environment variables internally. When using `make test-integration`, Docker Compose manages them automatically.

| Variable | Purpose | Default |
|---|---|---|
| `GATEWAY_URL` | Gateway JSON-RPC endpoint | `http://localhost:8080/v1/` |
| `ADMIN_URL` | Admin API base URL | `http://localhost:8081` |
| `ADMIN_SECRET` | Admin secret for admin endpoints | `devsecret` |

---

## Implemented Integration Test Files

| File | Purpose |
|---|---|
| `tests/go/admin_api_test.go` | Admin API contract tests |
| `tests/go/cache_test.go` | Gateway cache behavior |
| `tests/go/failover_test.go` | Gateway availability and JSON-RPC error behavior |
| `tests/go/gateway_proxy_test.go` | Gateway auth, RPC, batch, method, and header behavior |
| `tests/go/rate_limit_test.go` | RPM, daily quota, reset, and header behavior |
| `tests/go/security_test.go` | Invalid key, missing admin secret, SSRF, private IP blocking |
| `tests/go/testutil.go` | Shared helpers for integration tests |

---

## What Is Covered

### Admin API

Implemented tests cover:

- `/health`
- Tenant creation
- Tenant listing
- Tenant deletion
- Tenant validation errors
- Missing admin secret rejection
- Tenant usage endpoint
- Audit log endpoint
- `/blocks` endpoint

Relevant file:

```text
tests/go/admin_api_test.go
```

---

### Gateway RPC and Auth

Implemented tests cover:

- Authenticated JSON-RPC calls
- Missing auth rejection
- Invalid API key rejection
- Supported method calls:
  - `eth_chainId`
  - `eth_blockNumber`
  - `net_version`
- Batch requests
- Rate-limit headers on successful RPC calls
- Gateway health endpoint behavior

Relevant file:

```text
tests/go/gateway_proxy_test.go
```

---

### Caching

Implemented tests cover:

- Cache MISS on first call
- Cache HIT on second call
- Non-cacheable method behavior
- Cache isolation between API keys

The cache MISS/HIT test uses `eth_getBalance` with a random address to avoid false failures caused by a warm cache.

Relevant file:

```text
tests/go/cache_test.go
```

---

### Rate Limiting

Implemented tests cover:

- RPM enforcement
- Daily quota behavior
- Rate-limit window reset
- Rate-limit headers on successful requests

The rate-limit tests use a helper to avoid minute-boundary flakiness.

Relevant file:

```text
tests/go/rate_limit_test.go
```

---

### Security

Implemented tests cover:

- Invalid API key rejection
- Missing admin secret rejection
- SSRF protection for loopback endpoints
- Private IP blocking

Relevant file:

```text
tests/go/security_test.go
```

---

### Gateway Availability and JSON-RPC Errors

Implemented tests cover:

- Gateway serving successful RPC requests when healthy
- Gateway returning an acceptable JSON-RPC error for invalid methods

These tests are not full failover or chaos tests. They only verify basic availability and error-shape behavior.

Relevant file:

```text
tests/go/failover_test.go
```

---

## What Is Not Currently Covered

The integration suite does **not** currently automate:

- Migration rollback tests
- Down-migration verification
- True failover testing
- Chaos testing
- Redis failure behavior
- Postgres failure behavior
- Production external-endpoint validation

Those areas are either not automated yet or belong in manual/runbook documentation.

---

## Notes

- The integration suite counts top-level Go test functions.
- Some tests contain subtests, especially method coverage tests.
- The cache tests intentionally avoid using already-warm methods such as `eth_chainId` for MISS/HIT verification.
- Rate-limit tests may take over one minute because reset behavior waits for a new rate-limit window.

---

## Related Documents

- [readme.md](readme.md)
- [results.md](results.md)
- [unit-testing.md](unit-testing.md)
- [smoke-test.md](smoke-test.md)
- [load-test.md](load-test.md)