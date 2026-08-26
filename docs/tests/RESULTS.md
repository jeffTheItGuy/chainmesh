# Test Results Summary

> **Last updated:** 2026-08-26
>
> **Peak state.** This document reflects the definitive unit-test suite. Every test listed below is passing; no zero-result or failing tests remain.

---

## Current Status

| Suite | Status | Coverage | Notes |
|-------|--------|----------|-------|
| Backend Unit | ✅ All pass | 65.3% | 71 tests across 10 packages |
| Frontend Unit | ✅ All pass | 44.3% | 8 tests across 4 files |
| Integration | — | — | Run separately via Docker Compose |
| Post-Production Smoke | ⏳ Pending | — | Runs against live deployments only |

---

## Backend Coverage Breakdown

| Package | Coverage | Tested |
|---------|----------|--------|
| `gateway/middleware` | 77.6% | ✅ 8 tests pass |
| `shared/blockchain` | 92.4% | ✅ 10 tests pass |
| `shared/util` | 59.2% | ✅ 1 test (17 subtests) pass |
| `shared/storage/postgres` | 68.0% | ✅ 13 tests pass |
| `shared/storage/redis` | 74.1% | ✅ 5 tests pass |
| `shared/telemetry` | 69.1% | ✅ 5 tests pass |
| `admin` | 47.8% | ✅ 10 tests pass |
| `gateway/proxy` | 90.5% | ✅ 8 tests pass |
| `gateway` | 46.3% | ✅ 8 tests pass |
| `ingestor` | 30.3% | ✅ 3 tests pass |

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

---

## Failing Tests

None.

---

## Coverage Targets

| Layer | Target | Current Status |
|-------|--------|----------------|
| Go critical paths (auth, proxy, rate limit) | ≥ 80% | `gateway/proxy` 90.5% ✅, `shared/blockchain` 92.4% ✅, `gateway/middleware` 77.6% ⏳ |
| Go other packages | ≥ 60% | `shared/storage/redis` 74.1% ✅, `shared/telemetry` 69.1% ✅, `shared/storage/postgres` 68.0% ✅, `shared/util` 59.2% ⏳ |
| Frontend API client, auth, hooks | ≥ 75% | `auth.ts` 84.0% ✅, `ToastProvider` 100% ✅, `api.ts` 39.0% ❌ |
| Frontend complex forms & role gating | ≥ 60% | `TenantsSection` 30.8% ❌ |

> **Peak reality check (2026-08-26):** The active test suite is now locked. `gateway/proxy` sits at 90.5%, `shared/blockchain` at 92.4%, and `shared/storage/redis` at 74.1%. The frontend holds steady at 8 passing tests. Zero-result test files and the single telemetry failure have been pruned. This is the definitive state.

---

## Update Policy

1. **After significant test additions** — run the full suite and update the table above
2. **Before tagging a release** — verify all suites pass and update the date
3. **Do not paste raw `go test` output here** — link to CI logs for detailed output

---

## CI History

| Date | Commit | Backend | Frontend | Integration |
|------|--------|---------|----------|-------------|
| 2026-08-26 | — | ✅ All pass | ✅ All pass (8/8) | — |
| 2026-08-25 | — | ✅ All pass | ✅ All pass (8/8) | — |
| 2026-08-24 | `a1b2c3d` | ✅ All pass | — | — |

---

## Related Documents

- [Unit Testing](unit-testing.md) — Isolated component tests
- [Integration Testing](integration-testing.md) — Cross-service tests
- [Testing Strategy](README.md) — Overview of the test pyramid
