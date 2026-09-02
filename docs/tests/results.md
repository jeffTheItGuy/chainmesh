<!-- results.md -->
# Test Results Summary

> **Last updated:** 2026-09-02  
> Documentation corrected to match the currently implemented tests.

---

## Current Status

| Suite | Status | Notes |
|---|---|---|
| Backend Unit | ✅ All pass | 71 tests across 10 packages |
| Frontend Unit | ✅ All pass | 8 tests across 4 files |
| Integration | ✅ All pass | 30 top-level tests, 71.2s runtime |
| Smoke | ✅ Pass | 6/6 in-cluster smoke checks passed |
| Load | Manual workflow available | Run via `.github/workflows/load-test.yml` |

---

## Backend Test Breakdown

| Package | Tested |
|---|---|
| `gateway/middleware` | ✅ 8 tests pass |
| `shared/blockchain` | ✅ 10 tests pass |
| `shared/util` | ✅ 1 test, 17 subtests pass |
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
|---|---|
| `src/auth.ts` | ✅ 3 tests pass |
| `src/components/ToastProvider.tsx` | ✅ 1 test passes |
| `src/api.ts` | ✅ 3 tests pass |
| `src/TenantsSection.tsx` | ✅ 1 test passes |
| **Frontend overall** | ✅ 8 tests pass |

---

## Integration Test Breakdown

The integration suite contains **30 top-level Go tests**.

`TestGatewayRPCMethods` contains subtests, but the table below counts top-level test functions.

| Area | Tests | Notes |
|---|---:|---|
| Admin API core | 8 | Health, tenant create/list/delete, validation, missing-secret rejection, usage, audit logs |
| Blocks / node health | 2 | `/blocks` and gateway `/health/nodes` |
| Gateway caching | 3 | Cache MISS/HIT, non-cacheable methods, cache/auth isolation |
| Gateway RPC / availability | 9 | Authenticated RPC, missing auth, invalid key, method coverage, batch requests, rate-limit headers, healthy response, JSON-RPC error handling |
| Rate limiting | 4 | RPM enforcement, daily quota, window reset, headers on success |
| Auth / security | 4 | Invalid API key, missing admin secret, SSRF protection, private IP blocking |
| **Total** | **30** | ✅ All pass |

---

## In-Cluster Smoke Test Results

✅ **ChainMesh Smoke Test — in-cluster**

| Check | Status | Detail |
|---|---|---|
| Gateway reachable | ✅ Pass | HTTP response received |
| Authenticated RPC | ✅ Pass | returned result |
| Cache MISS, first call | ✅ Pass | `X-Cache: MISS` |
| Cache HIT, second call | ✅ Pass | `X-Cache: HIT` |
| Rate limit headers | ✅ Pass | present |
| Web dashboard | ✅ Pass | HTTP 200 |

---

## Failing Tests

None across the current unit, integration, and latest in-cluster smoke suites.

---

## Critical Path Checklist

This tracks whether important behavior has dedicated tests, not arbitrary coverage percentages.

| Area | Dedicated Tests | Status |
|---|---|---|
| Auth / API key validation | Unit + integration | ✅ Tested |
| Rate limiting | Unit + integration + smoke header check | ✅ Tested |
| RPC routing and batch requests | Unit + integration + smoke RPC check | ✅ Tested |
| Caching | Unit + integration + smoke MISS/HIT checks | ✅ Tested |
| SSRF / private IP protection | Unit + integration | ✅ Tested |
| Admin auth and audit logging | Unit + integration | ✅ Tested |
| Tenant lifecycle | Unit + integration | ✅ Tested |
| Blockchain endpoint health | Unit + integration | ✅ Tested |
| Frontend auth/session logic | Unit | ✅ Tested |
| Frontend API client error handling | Unit | ⚠️ Partial |
| Frontend complex forms | Unit | ⚠️ Partial |
| Ingestor block parsing | Unit | ⚠️ Partial |

---

## Known Gaps

The current known gaps are:

1. Frontend API client error classification is partially tested.
2. Frontend `TenantsSection` create/edit toggle is tested, but confirmation flows and side effects are not fully covered.
3. Ingestor block parsing has limited edge-case coverage.
4. Failover and chaos tests are not currently automated.
5. Load-test metrics currently cover p95 latency and error rate only. Cache hit rate and 429-spike detection are not currently measured.

---

## Update Policy

1. After significant test additions, run the relevant suite and update this document.
2. Before tagging a release, verify unit, integration, smoke, and load expectations.
3. Do not paste raw `go test` or Vitest output here. Link to CI logs or generated artifacts instead.

---

## Related Documents

- [readme.md](readme.md)
- [unit-test.md](unit-test.md)
- [integrations-test.md](integrations-test.md)
- [smoke-test.md](smoke-test.md)
- [load-test.md](load-test.md)