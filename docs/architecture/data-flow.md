# Data Flow

This document traces how requests, configuration, and data move through ChainMesh — from an incoming RPC call to a persisted usage record.

---

## 1. Gateway Request Lifecycle

### Overview

```
Client ──▶ Gateway ──▶ [Middleware Chain] ──▶ Proxy ──▶ Upstream Node
                                      │
                                      ▼
                                Redis (cache/rate limits)
                                      │
                                      ▼
                                PostgreSQL (telemetry)
```

### Step-by-Step

#### 1.1 Request Arrives
```
POST /v1/
Authorization: Bearer bm_live_...
Content-Type: application/json
X-Request-ID: <generated-or-provided>

{ "jsonrpc": "2.0", "method": "eth_getBalance", "params": [...], "id": 1 }
```

#### 1.2 Request ID Middleware (`middleware/requestid.go`)
- Extracts `X-Request-ID` header or generates a random 32-char hex ID
- Injects ID into request context for tracing across all downstream calls
- Sets `X-Request-ID` response header

#### 1.3 Auth Middleware (`middleware/auth.go`)
- Validates `Authorization: Bearer <key>` format
- Extracts the 12-character prefix from the provided key
- Queries `api_keys` for rows matching that prefix with `revoked_at IS NULL`
- For each candidate row, verifies the full key against the stored `bcrypt` hash
- On match: updates `last_used_at`, injects `*model.Tenant` into context
- Returns `401` if key missing, revoked, or not found
- Injected tenant includes quotas, network ID, and plan

#### 1.4 Rate Limit Middleware (`middleware/ratelimit.go`)
- Extracts tenant from context
- Calls `redis.CheckRateLimits()` with tenant's RPS/RPM/Daily quotas
- Uses Lua script for atomic INCR+EXPIRE (prevents race conditions on key creation)
- Sets response headers:
  - `X-RateLimit-Limit-Minute`
  - `X-RateLimit-Remaining-Minute`
  - `X-RateLimit-Reset` (Unix timestamp of next minute)
  - `Retry-After` (on 429)
- Returns `429` if any limit exceeded; returns `503` if Redis unavailable (fail closed)

#### 1.5 Proxy Handler (`proxy/proxy.go`)

**Parse & Validate:**
- Reads body (max 2MB)
- Unmarshals JSON-RPC request
- Validates `jsonrpc: "2.0"` and non-empty `method`
- Returns `400` for invalid JSON or malformed requests

**Determine Network:**
- Uses `tenant.BlockchainNetworkID` if set
- Otherwise queries `GetDefaultBlockchainConfig()` (earliest enabled network)
- Returns `503` if no networks configured

**Cache Lookup:**
- Builds cache key: `rpc:<network_id>:<method>:<params_json>`
- Checks Redis for cacheable methods:
  | Method | TTL |
  |--------|-----|
  | `eth_chainId` | 24h |
  | `eth_blockNumber` | 2s |
  | `eth_getBalance` | 30s |
  | `eth_gasPrice` | 15s |
  | `eth_maxPriorityFeePerGas` | 15s |
- Cache hit: returns immediately with `X-Cache: HIT`

**Upstream Call:**
- Fetches `*blockchain.Client` from Manager
- Client selects endpoint via `endpointsForCall()`:
  1. Healthy endpoints sorted by latency (ascending)
  2. Unhealthy endpoints as last-resort fallback
- Sends `POST` with `User-Agent: ChainMesh-Gateway/1.0`
- Retries next endpoint on transport or parse errors
- Returns `502` if all endpoints fail

**Response Handling:**
- RPC-level errors (in JSON-RPC `error` field) are returned as HTTP 200 with the error body
- Successful responses are cached (if method is cacheable)
- Returns `X-Cache: MISS` for uncached successful responses

**Telemetry Recording:**
- Calculates latency from request start
- Records Prometheus metrics:
  - `chainmesh_gateway_requests_total` (counter, by network/method/status/cache)
  - `chainmesh_gateway_request_duration_seconds` (histogram)
  - `chainmesh_gateway_cache_hits_total` / `cache_misses_total`
- Enqueues async job to Telemetry Recorder:
  - `Usage`: tenant ID, method, count=1, bytes_in, period (truncated to minute)
  - `RequestLog`: tenant ID, network ID, method, status, latency_ms, cache_hit, bytes_in, request_id

---

## 2. Async Telemetry Pipeline

### Flow

```
Proxy ──enqueue──▶ Telemetry.Recorder.queue (bounded channel)
                         │
                         ▼
                    Worker goroutine
                         │
              ┌─────────┴─────────┐
              ▼                   ▼
       RecordUsage()        RecordRequestLog()
              │                   │
              ▼                   ▼
       usage table         request_logs table
```

### Behavior

- **Queue size:** Configurable (default 4096 jobs)
- **Overflow:** Drops jobs with `TelemetryDroppedTotal` metric increment
- **Retries:** 3 attempts with 50ms, 100ms, 150ms backoff
- **Timeout:** 5s per write attempt
- **Fallback:** If telemetry recorder is nil, falls back to synchronous `go func()` with 5s context timeout

### Materialized View Refresh

- `statsrollup.Refresher` runs every 60 seconds
- Attempts `REFRESH MATERIALIZED VIEW CONCURRENTLY request_logs_rollup_1m`
- Falls back to blocking refresh if concurrent fails (e.g., first population)
- Dashboard stats queries use rollup for ranges > 5 minutes old; raw table for recent data

---

## 3. Config Reload & Health Checks

### Gateway Manager (`gateway/manager.go`)

```
Every 15s ──▶ reload() ──▶ ListBlockchainConfigs() ──▶ Compare signatures
                                                    │
                                                    ▼
                                          Endpoint changed? ──Yes──▶ New Client
                                                    │
                                                    No
                                                    │
                                                    ▼
                                          Keep existing Client
```

- Config "signature" = `rpc_endpoint_1|rpc_endpoint_2|enabled`
- If signature changes: stops old health checks, creates new `blockchain.Client`
- If network disabled or deleted: stops health checks, removes from map
- **Important:** Gateway starts even with zero networks (prevents chicken-and-egg problem)

### Blockchain Client Health Checks (`shared/blockchain/client.go`)

```
Every 10s ──▶ runHealthCheck() ──▶ POST eth_chainId to each endpoint
                                          │
                                          ▼
                                   Success: healthy=true, latency=measured
                                   Failure: consecutive_fails++, healthy=false after 3
```

- Concurrent probes across all endpoints
- `User-Agent: ChainMesh-Gateway/1.0` header (prevents WAF blocks)
- Drains response body for connection reuse
- Updates Prometheus `NodeHealthy` gauge
- Endpoint marked unhealthy after **3 consecutive failures**
- Recovers automatically on next successful check

### Call Routing

```
endpointsForCall()
├── Healthy endpoints (sorted by latency ascending)
│   └── Try each in order
└── Unhealthy endpoints (fallback)
    └── Try each in order
```

- Success on any endpoint: updates health (healthy, resets fails, records latency)
- Failure on any endpoint: increments consecutive fails, may mark unhealthy

---

## 4. Ingestor Data Flow

```
Every 12s ──▶ fetchAndStore()
                    │
                    ▼
            eth_getBlockByNumber("latest", false)
                    │
                    ▼
            Parse RPC response (check for RPC error first)
                    │
                    ▼
            Extract: number, hash, parentHash, timestamp, tx_count
                    │
                    ▼
            StoreBlock() ──▶ INSERT ... ON CONFLICT (number, network_id) DO NOTHING
                    │
                    ▼
            Log: "ingested block <number> <hash> <tx_count>"
```

- Uses `false` for full transaction objects (only hashes needed for count)
- Handles RPC-level errors before JSON parsing
- Per-network goroutine (one per enabled config)

---

## 5. Admin API Request Flow

### Authentication

```
Request ──▶ adminAuth() ──▶ X-Admin-Secret header
                                │
                                ▼
                        ConstantTimeEqual(provided, env ADMIN_SECRET)
                                │
                        Match? ──Yes──▶ Proceed
                                │
                                No
                                │
                                ▼
                        RecordAuditLog() ──▶ audit_logs table
                                │
                                ▼
                        Return 403 Forbidden
```

- All admin endpoints (except `/health` and `/blocks`) require the secret
- Admin API exits on startup if `ADMIN_SECRET` is unset
- Failed attempts are logged to `audit_logs` with client IP and User-Agent

### Tenant Creation Flow

```
POST /tenants
├── Validate: name required
├── Set defaults: plan="free", quota_rpm=100, quota_rps=10, quota_daily=10000
├── Resolve network: use provided ID, or default network, or error
├── GenerateAPIKey(): bm_live_<32-char-hex>
├── Transaction:
│   ├── INSERT tenant
│   └── INSERT api_keys (bcrypt hash, prefix)
├── auditLog(): CREATE_TENANT ──▶ audit_logs table
└── Return: tenant + plaintext api_key (shown ONCE)
```

### Stats Query Flow

```
GET /stats/summary?range=1h
├── Parse range (15m, 1h, 24h)
├── Calculate `from` timestamp
├── If from > 5 minutes ago:
│   └── Query request_logs_rollup_1m (fast)
├── Else:
│   └── Query request_logs directly (accurate for recent data)
└── Return: totals, latency (avg/p95), top methods/networks/statuses, time series
```

### Audit Log Query Flow

```
GET /audit-logs?limit=50&offset=0
├── Validate admin auth
├── Parse limit (max 1000) and offset
├── Query audit_logs ORDER BY created_at DESC
└── Return: array of audit events
```

### Blockchain Config Creation Flow

```
POST /blockchain
├── Validate admin auth
├── Validate: name and rpc_endpoint_1 required
├── SSRF check: ValidateRPCEndpoint() rejects loopback/private/link-local IPs
├── SaveBlockchainConfig()
├── auditLog(): CREATE_BLOCKCHAIN_CONFIG ──▶ audit_logs table
└── Return: created config
```

---

## 6. Dashboard Data Flow

### Authentication Flow

```
User ──▶ RoleGate
             │
    ┌────────┴────────┐
    ▼                 ▼
Viewer              Admin
(no auth)           (secret required)
    │                 │
    ▼                 ▼
BlocksSection    Full dashboard
NodeStatus       + Tenants
                 + Networks
                 + Usage
                 + Monitoring
```

- Viewer: session storage flag only; can see blocks and node health
- Admin: `X-Admin-Secret` stored in session; full CRUD access
- Logout: clears all session storage

### Polling Patterns

| Section | Endpoint | Interval | Behavior |
|---------|----------|----------|----------|
| Monitoring | `/stats/summary` | 15s | Abort-safe, deduplicated requests |
| Node Status | `/health/nodes` | 10s | Abort-safe, background refresh |
| Blocks | `/blocks` | On mount | Static after load (no polling) |
| Tenants | `/tenants` | On mount | Manual refresh after mutations |

All polling uses `usePolling` hook with:
- `AbortController` for cancellation
- `document.hidden` check (pauses when tab inactive)
- `visibilitychange` refetch (resumes when tab active)
- Request ID deduplication (ignores stale responses)

---

## 7. Error Propagation

| Layer | Error Condition | HTTP Status | Response Body |
|-------|----------------|-------------|---------------|
| Auth | Missing/invalid Bearer | 401 | `{"error":"unauthorized"}` |
| Rate Limit | Quota exceeded | 429 | `{"error":"rate limit exceeded"}` + Retry-After |
| Rate Limit | Redis down | 503 | `{"error":"rate limiter unavailable"}` |
| Proxy | Invalid JSON | 400 | `{"error":"invalid json"}` |
| Proxy | Body too large | 413 | `{"error":"request body too large"}` |
| Proxy | No network configured | 503 | `{"error":"no blockchain network configured"}` |
| Proxy | Network unavailable | 503 | `{"error":"blockchain network unavailable"}` |
| Proxy | All upstreams failed | 502 | `{"error":"upstream unavailable"}` |
| Admin | Missing secret | 403 | `{"error":"forbidden"}` |
| Admin | Invalid range | 400 | `{"error":"range must be one of: 15m, 1h, 24h"}` |
| Admin | Tenant not found | 404 | `{"error":"tenant not found"}` |

---

## Related Documents

- [Architecture Overview](overview.md) — Component map and tech stack
- [Database Schema](schema.md) — Table structures and relationships
