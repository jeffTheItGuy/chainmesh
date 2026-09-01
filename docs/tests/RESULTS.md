# Test Results Summary

> **Last updated:** 2026-09-01
>
> Unit-test suite and integration suite are both green. No zero-result or failing tests remain in either.
>
> Latest in-cluster smoke test passed 6/6.

---

## Current Status

| Suite | Status | Notes |
|-------|--------|-------|
| Backend Unit | ✅ All pass | 71 tests across 10 packages |
| Frontend Unit | ✅ All pass | 8 tests across 4 files |
| Integration | ✅ All pass | 30 tests pass, 71.2s runtime |
| Post-Production Smoke | ✅ Pass | 6/6 in-cluster smoke checks passed on 2026-09-01 |

---

## Backend Test Breakdown

| Package | Tested |
|---------|--------|
| `gateway/middleware` | ✅ 8 tests pass |
| `shared/blockchain` | ✅ 10 tests pass |
| `shared/util` | ✅ 1 test (17 subtests) pass |
| `shared/storage/postgres` | ✅ 13 tests pass |
| `shared/storage/redis` | ✅ 5 tests pass |
| `shared/telemetry` | ✅ 5 tests pass |
| `admin` | ✅ 10 tests pass |
| `gateway/proxy` | ✅ 8 tests pass |
| `gateway` | ✅ 8 tests pass |
| `ingestor` | ✅ 3 tests pass |

---

## Frontend Test Breakdown

| File / Directory | Tested |
|------------------|--------|
| `src/auth.ts` | ✅ 3 tests pass |
| `src/components/ToastProvider.tsx` | ✅ 1 test passes |
| `src/api.ts` | ✅ 3 tests pass |
| `src/TenantsSection.tsx` | ✅ 1 test passes |
| **Frontend overall** | ✅ 8 tests pass |

---

## Integration Test Breakdown

| Area | Tests | Notes |
|------|-------|-------|
| Admin API (tenants, usage, audit logs, health) | 10 | Includes create/list/delete tenant, validation, missing-secret rejection |
| Blocks / Node health | 2 | |
| Gateway caching | 3 | Cache hit header, non-cacheable methods, cache doesn't interfere with auth |
| Gateway RPC | 8 | Health, auth (with/without/invalid key), JSON-RPC error handling, batch requests, method coverage (`eth_chainId`, `eth_blockNumber`, `net_version`) |
| Rate limiting | 5 | Enforcement, daily quota, reset behavior, headers |
| Auth / security | 4 | Invalid API key, missing admin secret, SSRF protection, private IP blocking |
| **Total** | **30** | ✅ All pass, 71.2s runtime |

Full output: see CI job artifact / `integration-full.log` (not pasted here per Update Policy below).

---

## In-Cluster Smoke Test Results — 2026-09-01

✅ **ChainMesh Smoke Test — in-cluster**

Namespace: `chainmesh` · Gateway svc: `chainmesh-gateway-svc` · Passed: **6/6**

| Check | Status | Detail |
|------|--------|--------|
| Gateway reachable | ✅ Pass | HTTP 401 |
| Authenticated RPC | ✅ Pass | returned result |
| Cache MISS (1st call) | ✅ Pass | `X-Cache: MISS` |
| Cache HIT (2nd call) | ✅ Pass | `X-Cache: HIT` |
| Rate limit headers | ✅ Pass | present |
| Web dashboard | ✅ Pass | HTTP 200 |

---

## Failing Tests

None — across unit, integration, and latest in-cluster smoke suites.

---

## Critical Path Checklist

Instead of a coverage %, this tracks whether the areas that actually matter have dedicated tests and whether those tests pass.

| Area | Dedicated Tests | Status |
|------|-----------------|--------|
| Auth / API key validation | Unit (`gateway/middleware`) + Integration (`TestInvalidAPIKeyRejected`, `TestGatewayRPCInvalidKey`) | ✅ Tested |
| Rate limiting (enforce / quota / reset) | Unit (`shared/storage/redis`) + Integration (`TestRateLimitEnforced`, `TestRateLimitDailyQuota`, `TestRateLimitResets`) + Smoke (`Rate limit headers`) | ✅ Tested |
| RPC routing & batch requests | Unit (`gateway/proxy`) + Integration (`TestGatewayRPCMethods`, `TestGatewayBatchRequest`) + Smoke (`Authenticated RPC`) | ✅ Tested |
| Caching (hit/miss, auth interplay) | Unit (`gateway/proxy`) + Integration (`TestCacheHitHeader`, `TestCacheDoesNotInterfereWithAuth`) + Smoke (`Cache MISS`, `Cache HIT`) | ✅ Tested |
| SSRF / private IP protection | Unit (`shared/util`) + Integration (`TestSSRFProtection`, `TestPrivateIPBlocked`) | ✅ Tested |
| Admin auth & audit logging | Unit (`admin`) + Integration (`TestMissingAdminSecretRejected`, `TestAuditLogs`) | ✅ Tested |
| Tenant lifecycle (create / list / delete / validate) | Unit (`admin`) + Integration (`TestCreateTenant`, `TestDeleteTenant`, `TestCreateTenantValidation`) | ✅ Tested |
| Blockchain endpoint health | Unit (`shared/blockchain`) + Integration (`TestNodeHealthEndpoint`) | ✅ Tested |
| Frontend auth/session logic | Unit (`auth.ts`) | ✅ Tested |
| Frontend API client error handling | Unit (`api.ts`) | ⚠️ Partial — only 401/network-failure paths covered |
| Frontend complex forms (Tenants/Blockchain sections) | Unit (`TenantsSection.tsx`) | ⚠️ Partial — create/edit toggle tested; confirmation flows & side effects not yet |
| Ingestor block parsing | Unit (`ingestor`) | ⚠️ Partial — 3 tests cover known edge cases only |

> **Status snapshot (2026-09-01):** Unit and integration suites are fully green. Latest in-cluster smoke test passed 6/6. The known gaps are on the frontend — API client error classification and TenantsSection form flows are only partially tested — plus ingestor block-parsing edge cases.

---

## Update Policy

1. **After significant test additions** — run the full suite and update the table above
2. **Before tagging a release** — verify all suites pass and update the date
3. **Do not paste raw `go test` output here** — link to CI logs for detailed output

---

## CI History

| Date | Commit | Backend | Frontend | Integration / Smoke |
|------|--------|---------|----------|-------------|
| 2026-09-01 | — | — | — | ✅ In-cluster smoke pass (6/6) |
| 2026-08-27 | — | — | — | ✅ All pass (30/30) |
| 2026-08-26 | — | ✅ All pass | ✅ All pass (8/8) | — |
| 2026-08-25 | — | ✅ All pass | ✅ All pass (8/8) | — |
| 2026-08-24 | `a1b2c3d` | ✅ All pass | — | — |

---

## Related Documents

- [Unit Testing](unit-testing.md) — Isolated component tests
- [Integration Testing](integration-testing.md) — Cross-service tests
- [Testing Strategy](README.md) — Overview of the test pyramid