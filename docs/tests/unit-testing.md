# Unit Testing

Unit tests verify individual functions, packages, and components in isolation. They run fast, require no external services, and run on every PR.

---

## Table of Contents

1. [Go Backend](#go-backend)
2. [Frontend](#frontend)
3. [Coverage Targets](#coverage-targets)

---

## Go Backend

### Conventions

- Test files live alongside source: `foo.go` → `foo_test.go`
- Use standard `testing` package + `testify/assert` where helpful
- Mock external dependencies (Postgres, Redis, upstream RPC) via interfaces
- Table-driven tests for multiple input cases
- **Watch out for Postgres type mismatches:** Scanning an `inet` column into a Go `*string` will fail. Use `net.IP` or change the column to `text` if you need a string.

### Running Tests

```bash
cd backend

# All tests
go test ./...

# With race detector (CI does this)
go test -race ./...

# With coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Verbose output for a single package
go test -v ./gateway/middleware/...
```

### Current Coverage by Package

| Package | Coverage | What to Test | Mock Strategy |
|---------|----------|-------------|---------------|
| `gateway/middleware` | 46.9% | Auth, rate limiting, request ID | Mock Redis and Postgres pools |
| `shared/blockchain` | 46.5% | Health checks, endpoint selection | Mock HTTP transport |
| `shared/util` | 59.2% | SSRF validation | Pure functions, no mocks needed |
| `shared/storage/postgres` | 20.1% | Queries, transactions | Use `pgxmock` or testcontainers |
| `admin` | 5.5% | Admin auth, audit logging | Mock DB; fix `inet` scan target |
| `gateway/proxy` | 47.6% | Request parsing, cache logic, routing | Mock `blockchain.Client` and Redis |
| `shared/storage/redis` | 51.7% | Rate limit Lua script, cache ops | Use `miniredis` or mock client |
| `shared/telemetry` | 47.1% | Async enqueue, retry logic | Mock DB with buffered channel |
| `shared/util/apikey.go` | 0.0% | API key generation, hash, verify | Pure functions, no mocks needed |
| `ingestor` | 0.0% | Block parsing edge cases | Mock RPC responses with malformed blocks |
| `gateway/manager` | 0.0% | Config reload, health loop | Mock DB and blockchain client |
| `shared/statsrollup` | 0.0% | Materialized view refresh | Mock `pgxpool` or testcontainers |

### Example: Table-Driven Test

```go
// shared/util/validate_test.go
package util

import "testing"

func TestValidateRPCEndpoint(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
    }{
        {"public https", "https://ethereum-rpc.publicnode.com", false},
        {"public http", "http://example.com", false},
        {"loopback", "http://127.0.0.1:8545", true},
        {"private", "http://192.168.1.1", true},
        {"invalid scheme", "ftp://example.com", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateRPCEndpoint(tt.url)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateRPCEndpoint(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
            }
        })
    }
}
```

### Example: Mocking the Blockchain Client

```go
// gateway/proxy/proxy_test.go
type mockBlockchainClient struct {
    healthyEndpoints []blockchain.Endpoint
    callResponse     []byte
    callErr          error
}

func (m *mockBlockchainClient) EndpointsForCall() []blockchain.Endpoint {
    return m.healthyEndpoints
}

func (m *mockBlockchainClient) Call(ctx context.Context, method string, params []any) ([]byte, error) {
    return m.callResponse, m.callErr
}
```

---

## Frontend

### Conventions

- Test files: `ComponentName.test.tsx` alongside the component
- Use Vitest (configured in `vite.config.ts`) + React Testing Library
- Mock API calls with `msw` (Mock Service Worker)
- Test user interactions, not implementation details

### Running Tests

```bash
cd frontend
npm run test        # Run in watch mode
npm run test:ci     # Run once (for CI)
```

### What to Test

| Category | Examples |
|----------|----------|
| Components | Form validation, button states, modal open/close |
| Hooks | `usePolling`, `useAuth` — test with `renderHook` |
| API client | Response parsing, error handling |
| Utils | Formatters, validators |

### Example: Component Test

```tsx
// src/components/TenantForm.test.tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import TenantForm from './TenantForm'

describe('TenantForm', () => {
    it('submits with valid data', () => {
        const onSubmit = vi.fn()
        render(<TenantForm onSubmit={onSubmit} />)

        fireEvent.change(screen.getByLabelText(/name/i), {
            target: { value: 'Acme Corp' }
        })
        fireEvent.click(screen.getByRole('button', { name: /create/i }))

        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
            name: 'Acme Corp'
        }))
    })
})
```

---

## Coverage Targets

| Layer | Target | Enforcement |
|-------|--------|-------------|
| Go critical paths (auth, proxy, rate limit) | ≥ 80% | CI gate |
| Go other packages | ≥ 60% | CI warning |
| Frontend components | ≥ 70% | CI gate |
| Frontend hooks/utils | ≥ 75% | CI gate |

> **Current reality check (2026-08-24):** `shared/util/ssrf.go` (90.6%) and `gateway/middleware/ratelimit.go` (84.0%) still meet the critical-path target. `gateway/proxy` improved from 0% → 47.6%, `shared/storage/redis` from 0% → 51.7%, and `shared/telemetry` from 0% → 47.1%. Priorities to close the 80% gap: `gateway/proxy` (cache hit / upstream success), `shared/telemetry` (`RecordRequestLog`, `process`), and `shared/storage/redis` (`Get`/`Set`).

---

## Related Documents

- [Integration Testing](integration-testing.md) — Testing across service boundaries
- [Testing Strategy](README.md) — Overview of the test pyramid
- [Developer Setup](../developers/setup.md) — Installing test dependencies
