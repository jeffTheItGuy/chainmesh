# Test Results Summary

> **Last updated:** 2026-08-25

This document tracks the current health of the test suite. For raw CI output, see the latest GitHub Actions run.

---

## Current Status

| Suite | Status | Coverage | Notes |
|-------|--------|----------|-------|
| Backend Unit | ✅ All pass | 65.3% | 72 tests across 10 packages |
| Frontend Unit | ✅ All pass | 44.3% | 8 tests across 4 files |
| Integration | — | — | Not executed in this run |
| Post-Production Smoke | ⏳ Pending | — | Will run after first production deployment |
| Load / Performance | ⏳ Pending | — | k6 scripts ready; baseline TBD |

---

## Backend Coverage Breakdown

| Package | Coverage | Tested |
|---------|----------|--------|
| `gateway/middleware` | 77.6% | ✅ 8 tests pass |
| `shared/blockchain` | 92.4% | ✅ 10 tests pass |
| `shared/util` | 59.2% | ✅ 1 test (17 subtests) pass |
| `shared/storage/postgres` | 68.0% | ✅ 13 tests pass |
| `shared/storage/redis` | 74.1% | ✅ 5 tests pass |
| `shared/telemetry` | 69.1% | ✅ 6 tests pass |
| `admin` | 47.8% | ✅ 10 tests pass |
| `gateway/proxy` | 90.5% | ✅ 8 tests pass |
| `gateway` | 46.3% | ✅ 8 tests pass |
| `ingestor` | 30.3% | ✅ 3 tests pass |
| `gateway/manager`* | 87.7% | covered via `gateway` tests |
| `shared/statsrollup` | 0.0% | — |
| `shared/logger` | 0.0% | — |
| `shared/requestid` | 0.0% | — |
| `shared/metrics` | — | no test files |
| `shared/model` | — | no test files |

\* File-level coverage within the `gateway` package.

**Run the report locally:**

```bash
cd backend
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

---

## Frontend Coverage Breakdown

| File / Directory | Statements | Branches | Functions | Lines | Tested |
|------------------|------------|----------|-----------|-------|--------|
| `src/auth.ts` | 76.7% | 70.0% | 100% | 84.0% | ✅ 3 tests pass |
| `src/components/ToastProvider.tsx` | 93.8% | 50.0% | 100% | 100% | ✅ 1 test passes |
| `src/api.ts` | 35.6% | 34.5% | 21.7% | 39.0% | ✅ 3 tests pass |
| `src/TenantsSection.tsx` | 29.7% | 36.0% | 14.8% | 30.8% | ✅ 1 test passes |
| **`src/` total** | **38.7%** | **39.3%** | **29.3%** | **40.5%** | |
| **`src/components/` total** | **93.8%** | **50.0%** | **100%** | **100%** | |
| **Frontend overall** | **43.1%** | **39.6%** | **37.9%** | **44.3%** | ✅ 8 tests pass |

**Run the report locally:**

```bash
cd frontend
npm run test:ci -- --coverage
```

---

## Failing Tests

None.

---

## Known Gaps

| Gap | Priority | Plan |
|-----|----------|------|
| Add tests for `ingestor` main & run loop | High | 30.3% covered; `main` and `runIngestor` at 0% |
| Close 80% gap on `gateway/middleware` | High | 77.6% covered; `requestid.go` at 0% |
| Add tests for `shared/util/apikey.go` | Medium | 0% coverage; generation, hash, verify |
| Add tests for `admin` unhandled paths | Medium | 47.8% covered; `main`, list endpoints, stats at 0% |
| Add tests for `shared/statsrollup` | Medium | 0% coverage; materialized-view refresh |
| Add tests for `gateway` bootstrap | Low | `main.go` at 0% |
| Add tests for `shared/requestid` | Low | 0% coverage; thin wrapper |
| **Add tests for `TenantsSection` create/edit/submit flows** | **High** | **30.8% lines; `submit`, `remove`, `rotateKey`, `copyKey` at 0%** |
| **Add tests for `api.ts` request helper & endpoints** | **High** | **39.0% lines; most API methods uncovered** |
| **Add tests for `api.ts` non-AuthError branches** | **Medium** | **NetworkError path and `!res.ok` handling at 0%** |
| **Add tests for `TenantsSection` empty-state and network-select branches** | **Medium** | **Branch coverage 36%; several JSX branches uncovered** |

---

## Coverage Targets

| Layer | Target | Current Status |
|-------|--------|----------------|
| Go critical paths (auth, proxy, rate limit) | ≥ 80% | `gateway/proxy` 90.5% ✅, `shared/blockchain` 92.4% ✅, `gateway/middleware` 77.6% ⏳ |
| Go other packages | ≥ 60% | `shared/storage/redis` 74.1% ✅, `shared/telemetry` 69.1% ✅, `shared/storage/postgres` 68.0% ✅, `shared/util` 59.2% ⏳ |
| Frontend API client, auth, hooks | ≥ 75% | `auth.ts` 84.0% ✅, `ToastProvider` 100% ✅, `api.ts` 39.0% ❌ |
| Frontend complex forms & role gating | ≥ 60% | `TenantsSection` 30.8% ❌ |

> **Reality check (2026-08-25):** `gateway/proxy` jumped from 47.6% → 90.5%, `shared/storage/redis` from 51.7% → 74.1%, `shared/telemetry` from 47.1% → 69.1%, and `ingestor` from 0% → 30.3%. The remaining 80% critical-path gap is `gateway/middleware` (needs `requestid.go`). Frontend suite now running with 8 passing tests; `auth.ts` and `ToastProvider` meet targets, but `api.ts` and `TenantsSection` need significant coverage work to close the 75%/60% frontend gaps.

---

## Update Policy

1. **After significant test additions** — run the full suite and update the table above
2. **Before tagging a release** — verify all suites pass and update the date
3. **Do not paste raw `go test` output here** — link to CI logs for detailed output

---

## CI History

| Date | Commit | Backend | Frontend | Integration |
|------|--------|---------|----------|-------------|
| 2026-08-25 | — | ✅ All pass | ✅ All pass (8/8) | — |
| 2026-08-24 | `a1b2c3d` | ✅ All pass | — | — |

---

## Related Documents

- [Unit Testing](unit-testing.md) — Isolated component tests
- [Integration Testing](integration-testing.md) — Cross-service tests
- [Testing Strategy](README.md) — Overview of the test pyramid
