# Testing Strategy

This directory documents how ChainMesh is tested across the development lifecycle — from local unit tests to post-production validation.

---

## Test Pyramid

```
     ┌─────────────┐
     │   E2E /     │  Post-production smoke tests
     │   Smoke     │  (docs/tests/post-production-testing.md)
     ├─────────────┤
     │ Integration │  Docker Compose, API contract tests
     │             │  (docs/tests/integration-testing.md)
     ├─────────────┤
     │    Unit     │  Go *_test.go, React *.test.tsx
     │             │  (docs/tests/unit-testing.md)
     └─────────────┘
```

---

## Quick Commands

### Backend

```bash
cd backend

# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run a specific package
go test ./shared/blockchain/...

# Run a specific test
go test -run TestRateLimit ./shared/storage/redis/...
```

### Frontend

```bash
cd frontend

# Type checking
npm run typecheck

# Linting
npm run lint

# Unit tests (high-value targets only — see unit-testing.md)
npm run test

# Production build verification
npm run build
```

---

## CI Pipeline

The GitHub Actions workflow (`.github/workflows/test.yml`) runs:

1. **Backend lint & test** — `go vet ./...` + `go test -race ./...`
2. **Frontend typecheck & lint** — `npm run typecheck` + `npm run lint`
3. **Frontend build** — `npm run build`
4. **Integration test** — Spins up Docker Compose stack and runs API contract tests

---

## Test Organization

| Directory | Contents |
|-----------|----------|
| `backend/*/*_test.go` | Go unit tests (co-located with source) |
| `frontend/src/*.test.tsx` | React component and hook tests (high-ROI only) |
| `tests/e2e/` | End-to-end API tests (run against running stack) |

---

## Coverage Reporting

Current coverage snapshots are tracked in [Test Results](RESULTS.md). Update that file after significant test additions or before tagging a release.

---

## Related Documents

- [Unit Testing](unit-testing.md) — Go and frontend unit test patterns
- [Integration Testing](integration-testing.md) — Docker Compose and API contract tests
- [Post-Production Testing](post-production-testing.md) — Smoke tests, load tests, and chaos validation
- [Developer Setup](../developers/setup.md) — Local environment configuration
