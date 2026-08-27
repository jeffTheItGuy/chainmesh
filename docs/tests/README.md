# Testing Strategy

This directory documents how ChainMesh is tested across the development lifecycle — from local unit tests to post-production validation.

---

## Table of Contents

1. [Test Pyramid](#test-pyramid)
2. [Running the Tests](#running-the-tests)
3. [CI Pipeline](#ci-pipeline)
4. [Test Organization](#test-organization)
5. [Coverage Reporting](#coverage-reporting)
6. [Related Documents](#related-documents)

---

## Test Pyramid

Testing is layered as a pyramid, with the broadest, fastest checks at the base and the narrowest, most environment-dependent checks at the top.

| Layer | What it covers | Documented in |
|-------|----------------|---------------|
| E2E / Smoke (top) | Post-production smoke tests against live deployments | [post-production-testing.md](post-production-testing.md) |
| Integration (middle) | Docker Compose stack, API contract tests | [integration-testing.md](integration-testing.md) |
| Unit (base) | Go `*_test.go`, React `*.test.tsx` | [unit-testing.md](unit-testing.md) |

---

## Running the Tests

### Backend

Backend tests run through the Go toolchain from the `backend/` directory. A plain `go test ./...` runs the full suite; CI adds the race detector and produces coverage via `-coverprofile` followed by `go tool cover`. You can target a single package by path, or a single test with a `-run` filter. The canonical containerized run uses `docker-compose.test.yml`, which brings up disposable Postgres and Redis, applies migrations, then streams unit, Postgres-backed, and Redis-backed test stages into `test-results/`.

### Frontend

Frontend checks run from the `frontend/` directory: type checking, linting, the Vitest unit suite (high-value targets only — see unit-testing.md), and a production build verification. In CI, Vitest runs once with verbose and JSON reporters plus coverage; locally it runs in watch mode.

---

## CI Pipeline

The GitHub Actions workflow (`.github/workflows/test.yml`) runs:

1. **Backend lint & test** — `go vet` followed by the race-enabled test pass
2. **Frontend typecheck & lint** — TypeScript checking plus linting
3. **Frontend build** — production build verification
4. **Integration test** — spins up the Docker Compose stack and runs the API contract tests

---

## Test Organization

| Directory | Contents |
|-----------|----------|
| `backend/*/*_test.go` | Go unit tests (co-located with source) |
| `frontend/src/*.test.tsx` | React component and hook tests (high-ROI only) |
| `tests/go/` | Integration / API contract tests (run against the Docker Compose stack) |
| `tests/smoke/` | Post-production smoke and latency-baseline scripts |
| `tests/load/` | k6 load-test scripts |

---

## Coverage Reporting

Coverage snapshots now live in the test docs themselves: backend and frontend unit results in [Unit Testing](unit-testing.md), cross-service results in [Integration Testing](integration-testing.md). Update those docs after significant test additions or before tagging a release. Don't paste raw test output — link to CI logs instead.

---

## Related Documents

- [Unit Testing](unit-testing.md) — Go and frontend unit test patterns
- [Integration Testing](integration-testing.md) — Docker Compose and API contract tests
- [Post-Production Testing](post-production-testing.md) — Smoke tests, load tests, and chaos validation
- [Developer Setup](../developers/setup.md) — Local environment configuration