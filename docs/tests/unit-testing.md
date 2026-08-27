# Unit Testing

Unit tests verify individual functions, packages, and components in isolation. They run fast, require no external services, and run on every PR.

---

## Table of Contents

1. [Go Backend](#go-backend)
2. [Frontend](#frontend)
3. [Critical Path Checklist](#critical-path-checklist)

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

### Package Test Strategy

| Package | What to Test | Mock Strategy |
|---------|-------------|---------------|
| `gateway/middleware` | Auth, rate limiting, request ID | Mock Redis and Postgres pools |
| `shared/blockchain` | Health checks, endpoint selection | Mock HTTP transport |
| `shared/util` | SSRF validation | Pure functions, no mocks needed |
| `shared/storage/postgres` | Queries, transactions | Use `pgxmock` or testcontainers |
| `admin` | Admin auth, audit logging | Mock DB; fix `inet` scan target |
| `gateway/proxy` | Request parsing, cache logic, routing | Mock `blockchain.Client` and Redis |
| `shared/storage/redis` | Rate limit Lua script, cache ops | Use `miniredis` or mock client |
| `shared/telemetry` | Async enqueue, retry logic | Mock DB with buffered channel |
| `gateway` | Config reload, health loop | Mock DB and blockchain client |
| `ingestor` | Block parsing edge cases | Mock RPC responses with malformed blocks |

### Untested Packages

The following packages are intentionally not covered by unit tests. They are either thin wrappers, generated code, or tested via integration tests:

| Package | Reason |
|---------|--------|
| `shared/logger` | Thin wrapper around `slog.NewJSONHandler` |
| `shared/requestid` | Thin context wrapper; trivial pass-through |
| `shared/statsrollup` | Background loop; covered by integration tests |
| `shared/metrics` | Prometheus counter definitions; no logic |
| `shared/model` | Struct definitions only |
| `shared/util/apikey.go` | Cryptographic utilities; bcrypt cost makes unit tests slow; verified in integration tests |

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

The frontend is a thin dashboard over the Admin API. Test the edges and the logic, not the markup.

### Conventions

- Test files: `ComponentName.test.tsx` alongside the component
- Use Vitest + React Testing Library
- Mock API calls with `msw` (Mock Service Worker)
- Test user interactions and state changes, not implementation details
- **Do not write snapshot tests** — they create noise without catching real bugs

### Running Tests

```bash
cd frontend
npm run test        # Run in watch mode
npm run test:ci     # Run once (for CI)
```

### What to Test

| Category | Examples | Why |
|----------|----------|-----|
| API client | `src/api.ts` | Auth header injection, error classification (`AuthError` vs `NetworkError`), status code mapping |
| Auth logic | `src/auth.ts` | Role resolution, session lifecycle, `sessionStorage` fallback behavior |
| Async hooks | `usePolling.ts` | AbortController cleanup, deduplication, visibility-aware polling |
| Toast system | `ToastProvider.tsx` | Add/remove lifecycle, auto-dismiss timing |
| Complex forms | `TenantsSection.tsx`, `BlockchainSection.tsx` | Validation, create/edit mode toggle, confirmation flows, side effects |
| Role gating | `App.tsx` | Conditional rendering by role, auth error handling |
| Error boundaries | `ErrorBoundary.tsx` | Fallback render, recovery action |
| Pure utilities | `utils/color.ts`, `Sparkline.tsx` | Deterministic hashing, coordinate math |

### What NOT to Test

| Category | Examples | Why |
|----------|----------|-----|
| Presentational components | `TopBar`, `Badge`, `RoleGate`, `LearnMore`, `Login` (markup only), `BlocksSection` (table rendering) | No conditional logic; manual QA or visual regression is sufficient |
| Skeleton loaders | `Skeleton.tsx` | No logic; testing CSS classes is not valuable |
| Static icons | `Icons.tsx` | Pure SVG markup |
| Simple data pass-through | `UsageSection`, `StatsStrip` (except trend math), `NodeStatusSection` (except health logic) | Props → render; cover with integration/e2e instead |

### Example: API Client Test

```tsx
// src/api.test.ts
import { describe, it, expect, vi } from 'vitest'
import { api, AuthError, NetworkError } from './api'

describe('api client', () => {
  it('throws AuthError on 401', async () => {
    global.fetch = vi.fn(() =>
      Promise.resolve({ status: 401, ok: false } as Response)
    )
    await expect(api.health()).rejects.toThrow(AuthError)
  })

  it('throws NetworkError when fetch fails', async () => {
    global.fetch = vi.fn(() => Promise.reject(new Error('network down')))
    await expect(api.health()).rejects.toThrow(NetworkError)
  })
})
```

### Example: Auth Hook Test

```tsx
// src/auth.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { getRole, storeSecret, storeViewerSession, clearSession } from './auth'

describe('auth', () => {
  beforeEach(() => {
    clearSession()
  })

  it('returns admin when secret is stored', () => {
    storeSecret('sekrit')
    expect(getRole()).toBe('admin')
  })

  it('returns viewer when viewer session is stored', () => {
    storeViewerSession()
    expect(getRole()).toBe('viewer')
  })

  it('returns null when session is cleared', () => {
    storeSecret('sekrit')
    clearSession()
    expect(getRole()).toBeNull()
  })
})
```

### Example: Complex Form Test

```tsx
// src/TenantsSection.test.tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import TenantsSection from './TenantsSection'

describe('TenantsSection', () => {
  it('toggles from create to edit mode and pre-fills fields', () => {
    const tenant = {
      id: 't1', name: 'Acme', quota_rpm: 500, quota_rps: 5,
      quota_daily: 50000, plan: 'pro', created_at: new Date().toISOString()
    }

    render(
      <TenantsSection
        tenants={[tenant]}
        networks={[]}
        hasLoaded={true}
        onTenantCreated={vi.fn()}
        onTenantDeleted={vi.fn()}
        onTenantUpdated={vi.fn()}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: /edit/i }))
    expect(screen.getByDisplayValue('Acme')).toBeInTheDocument()
    expect(screen.getByDisplayValue('pro')).toBeInTheDocument()
  })
})
```

---

## Critical Path Checklist

Instead of tracking arbitrary coverage percentages, we track whether the areas that actually matter have dedicated tests and whether those tests pass.

For the definitive status of tested areas and known gaps, see the **[Critical Path Checklist in RESULTS.md](RESULTS.md#critical-path-checklist)**.

---

## Related Documents

- [Integration Testing](integration-testing.md) — Testing across service boundaries
- [Testing Strategy](README.md) — Overview of the test pyramid
- [Developer Setup](../developers/setup.md) — Installing test dependencies