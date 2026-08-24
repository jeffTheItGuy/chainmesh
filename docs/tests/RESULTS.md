# Test Results Summary

> **Last updated:** 2026-08-24

This document tracks the current health of the test suite. For raw CI output, see the latest GitHub Actions run.

---

## Current Status

| Suite | Status | Coverage | Notes |
|-------|--------|----------|-------|
| Backend Unit | ✅ All pass | 23.6% | 21 tests across 8 packages |
| Frontend Unit | — | — | Not executed in this run |
| Integration | — | — | Not executed in this run |
| Post-Production Smoke | ⏳ Pending | — | Will run after first production deployment |
| Load / Performance | ⏳ Pending | — | k6 scripts ready; baseline TBD |

---

## Backend Coverage Breakdown

| Package | Coverage | Tested |
|---------|----------|--------|
| `gateway/middleware` | 46.9% | ✅ 4 tests pass |
| `shared/blockchain` | 46.5% | ✅ 5 tests pass |
| `shared/util` | 59.2% | ✅ 17 subtests pass |
| `shared/storage/postgres` | 20.1% | ✅ 3 tests pass |
| `shared/storage/redis` | 51.7% | ✅ 2 tests pass |
| `shared/telemetry` | 47.1% | ✅ 2 tests pass |
| `admin` | 5.5% | ✅ 3 tests pass |
| `gateway/proxy` | 47.6% | ✅ 1 test (7 subtests) pass |
| `gateway` | 0.0% | — |
| `ingestor` | 0.0% | — |
| `gateway/manager` | 0.0% | — |
| `shared/statsrollup` | 0.0% | — |
| `shared/logger` | 0.0% | — |
| `shared/requestid` | 0.0% | — |
| `shared/metrics` | — | no test files |
| `shared/model` | — | no test files |

**Run the report locally:**

```bash
cd backend
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

---

## Failing Tests

None.

---

## Known Gaps

| Gap | Priority | Plan |
|-----|----------|------|
| Add tests for `gateway/proxy` | High | 47.6% covered; needs cache-hit & upstream success paths |
| Add tests for `shared/storage/redis` | High | 51.7% covered; needs `Get`/`Set`/cache ops |
| Add tests for `shared/telemetry` | High | 47.1% covered; needs `RecordRequestLog`, `process`, retry |
| Add tests for `ingestor` | High | 0% coverage; block parsing edge cases |
| Add tests for `gateway/manager` | Medium | 0% coverage; config reload, health loop |
| Add tests for `shared/statsrollup` | Medium | 0% coverage; materialized-view refresh |
| Add tests for `shared/util/apikey.go` | Medium | 0% coverage; generation, hash, verify |
| Add tests for `admin` audit-log paths | Medium | `auditLog` helper still at 0% |

---

## Update Policy

1. **After significant test additions** — run the full suite and update the table above
2. **Before tagging a release** — verify all suites pass and update the date
3. **Do not paste raw `go test` output here** — link to CI logs for detailed output

---

## CI History

| Date | Commit | Backend | Frontend | Integration |
|------|--------|---------|----------|-------------|
| 2026-08-24 | `a1b2c3d` | ✅ All pass | — | — |

---

## Related Documents

- [Unit Testing](unit-testing.md) — Isolated component tests
- [Integration Testing](integration-testing.md) — Cross-service tests
- [Testing Strategy](README.md) — Overview of the test pyramid
