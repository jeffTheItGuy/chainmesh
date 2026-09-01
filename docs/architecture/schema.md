# Database Schema

ChainMesh uses PostgreSQL 15 with pgx/v5. The schema evolves through numbered migrations and supports multi-tenant, multi-network operation.

---

## Migration History

| File | Purpose |
|------|---------|
| `001_init.up.sql` | Core tables: `tenants`, `usage`, `blocks` |
| `001_init.down.sql` | Drop core tables |
| `002_audit_logs.sql` | Admin audit trail |
| `002_blockchain_config.sql` | Add `blockchain_configs` table |
| `003_multi_network.sql` | Multi-network support: FKs, unique constraints, indexes |
| `004_api_keys.sql` | Separate `api_keys` table; hash-based auth |
| `005_request_logs.sql` | Per-request audit logging |
| `006_tenant_rate_limits.sql` | Per-tenant plan, RPS, daily quotas |
| `007_request_id.sql` | Add `request_id` to request logs |
| `008_stats_rollup.sql` | Pre-aggregated materialized view for dashboards |

---

## Entity Relationship Diagram

---

## Table Reference

### `tenants`

The root entity for multi-tenancy. Each tenant represents a customer or application with its own API key and quotas.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | `UUID` | PK, `uuid_generate_v4()` | System-generated |
| `name` | `VARCHAR(255)` | NOT NULL | Display name |
| `api_key` | `VARCHAR(255)` | UNIQUE, nullable | Legacy plaintext key (deprecated by 004) |
| `quota_rpm` | `INT` | NOT NULL, DEFAULT 60 | Requests per minute |
| `quota_rps` | `INT` | NOT NULL, DEFAULT 0 | Requests per second (0 = unlimited) |
| `quota_daily` | `INT` | NOT NULL, DEFAULT 0 | Requests per day (0 = unlimited) |
| `plan` | `TEXT` | NOT NULL, DEFAULT 'free' | Informational tier label |
| `blockchain_network_id` | `UUID` | FK → `blockchain_configs(id)`, nullable | Pinned network; NULL = use default |
| `created_at` | `TIMESTAMPTZ` | DEFAULT NOW() | |

**Migration notes:**
- `quota_rps`, `quota_daily`, `plan` added in `006_tenant_rate_limits.sql`
- `blockchain_network_id` added in `003_multi_network.sql`
- `api_key` made nullable in `004_api_keys.sql` (migrated to separate table)

---

### `api_keys`

Stores hashed API keys with rotation support. A tenant may have multiple keys over time (one active at a time in normal operation).

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | `UUID` | PK, `gen_random_uuid()` | System-generated |
| `tenant_id` | `UUID` | NOT NULL, FK → `tenants(id)`, ON DELETE CASCADE | |
| `name` | `TEXT` | NOT NULL, DEFAULT 'default' | Human-readable label |
| `key_hash` | `TEXT` | NOT NULL, UNIQUE | bcrypt hash of full key |
| `key_prefix` | `TEXT` | NOT NULL | First 12 chars of key (for display) |
| `revoked_at` | `TIMESTAMPTZ` | nullable | Set on rotation or deletion |
| `last_used_at` | `TIMESTAMPTZ` | nullable | Updated on each successful auth |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | |

**Indexes:**
- `idx_api_keys_tenant_id` — fast lookup by tenant
- `idx_api_keys_key_hash` — fast auth lookup (critical path)

**Security model:**
- Full API keys are never stored. Only a `bcrypt` hash and a 12-char prefix.
- Auth query: extract prefix → query rows matching prefix → bcrypt verify against `key_hash` → update `last_used_at`
- Revoked keys have `revoked_at` set; auth query filters `revoked_at IS NULL`

---

### `blockchain_configs`

Dynamic configuration for blockchain networks. Managed entirely via Admin API — no env vars.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | `UUID` | PK, `gen_random_uuid()` | System-generated |
| `name` | `TEXT` | NOT NULL | Display name |
| `rpc_endpoint_1` | `TEXT` | NOT NULL | Primary endpoint |
| `rpc_endpoint_2` | `TEXT` | nullable | Failover endpoint |
| `chain_id` | `TEXT` | nullable | Chain identifier (e.g., "1", "137") |
| `enabled` | `BOOLEAN` | NOT NULL, DEFAULT true | Gateway ignores disabled configs |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | |

**Behavior:**
- Gateway reloads every 15 seconds; only `enabled=true` configs become active
- Deleting a config uses a transaction to unlink `tenants.blockchain_network_id` and `blocks.network_id` first (avoids FK violations)
- Earliest enabled config by `created_at` serves as the default network
- New endpoints are validated via SSRF protection (loopback, private, and link-local IPs are rejected)

---

### `blocks`

Stores ingested block metadata per network. Raw JSON is kept for potential reprocessing.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `number` | `BIGINT` | Part of PK | Block number |
| `hash` | `VARCHAR(66)` | NOT NULL, UNIQUE | 0x + 64 hex chars |
| `parent_hash` | `VARCHAR(66)` | NOT NULL | Previous block hash |
| `timestamp` | `TIMESTAMPTZ` | NOT NULL | Block time |
| `tx_count` | `INT` | NOT NULL, DEFAULT 0 | Transaction count |
| `raw_json` | `JSONB` | nullable | Full block JSON (optional) |
| `network_id` | `UUID` | FK → `blockchain_configs(id)`, nullable | Added in 003 |
| `created_at` | `TIMESTAMPTZ` | DEFAULT NOW() | Ingestion time |

**Constraints:**
- `blocks_number_network_key UNIQUE (number, network_id)` — allows same block number on different chains

**Indexes:**
- `idx_blocks_network_id` — fast listing by network

**Query patterns:**
- `ORDER BY number DESC LIMIT 50` — dashboard recent blocks
- `WHERE network_id = $1 ORDER BY number DESC LIMIT 1` — latest block per network

---

### `usage`

Aggregated usage per tenant, method, and minute. Upserted continuously by the telemetry worker.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `tenant_id` | `UUID` | NOT NULL, FK → `tenants(id)`, ON DELETE CASCADE | |
| `method` | `VARCHAR(255)` | NOT NULL | JSON-RPC method name |
| `count` | `BIGINT` | NOT NULL, DEFAULT 0 | Request count |
| `bytes_in` | `BIGINT` | NOT NULL, DEFAULT 0 | Total request body bytes |
| `period` | `TIMESTAMPTZ` | NOT NULL | Truncated to minute |

**PK:** `(tenant_id, method, period)`

**Upsert logic:**
```sql
INSERT INTO usage (...) VALUES (...)
ON CONFLICT (tenant_id, method, period)
DO UPDATE SET count = usage.count + $3, bytes_in = usage.bytes_in + $4
```

**Query patterns:**
- `WHERE tenant_id = $1 AND DATE(period) = DATE($2)` — daily usage report

---

### `request_logs`

Raw audit trail of every gateway request. Written asynchronously; queried for stats and debugging.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | `BIGSERIAL` | PK | Auto-increment |
| `tenant_id` | `UUID` | NOT NULL, FK → `tenants(id)`, ON DELETE CASCADE | |
| `network_id` | `UUID` | FK → `blockchain_configs(id)`, ON DELETE SET NULL | Nullable for pre-network requests |
| `method` | `TEXT` | NOT NULL | JSON-RPC method or empty for invalid |
| `status` | `TEXT` | NOT NULL | `success`, `rpc_error`, `upstream_error`, `invalid_request`, `network_unavailable` |
| `latency_ms` | `INTEGER` | NOT NULL, DEFAULT 0 | Round-trip time |
| `cache_hit` | `BOOLEAN` | NOT NULL, DEFAULT false | |
| `bytes_in` | `INTEGER` | NOT NULL, DEFAULT 0 | Request body size |
| `request_id` | `TEXT` | nullable | Tracing ID from X-Request-ID |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | |

**Indexes:**
- `idx_request_logs_created_at DESC` — time-series queries
- `idx_request_logs_tenant_created_at DESC` — per-tenant history
- `idx_request_logs_method` — method analysis
- `idx_request_logs_status` — error analysis
- `idx_request_logs_network_created_at DESC` — per-network history
- `idx_request_logs_request_id` — tracing lookups

**Status values:**
| Status | Meaning |
|--------|---------|
| `success` | Proxied successfully, valid JSON-RPC result |
| `rpc_error` | Upstream returned JSON-RPC error object |
| `upstream_error` | Transport failure (connection, timeout) |
| `invalid_request` | Bad JSON, missing fields, or body too large |
| `network_unavailable` | No blockchain network configured for tenant |

---

### `request_logs_rollup_1m` (Materialized View)

Pre-aggregated stats for fast dashboard queries. Refreshed every 60 seconds.

| Column | Type | Notes |
|--------|------|-------|
| `bucket` | `TIMESTAMPTZ` | Truncated to minute |
| `network_id` | `TEXT` | Empty string for NULL |
| `method` | `TEXT` | JSON-RPC method |
| `status` | `TEXT` | Request status |
| `cache_hit` | `BOOLEAN` | |
| `requests` | `BIGINT` | Count |
| `errors` | `BIGINT` | Count where status ≠ 'success' |
| `cache_hits` | `BIGINT` | Count where cache_hit = true |
| `avg_latency_ms` | `FLOAT8` | Average latency |
| `p95_latency_ms` | `FLOAT8` | 95th percentile latency |

**Indexes:**
- `request_logs_rollup_1m_unique` — supports `REFRESH CONCURRENTLY`
- `request_logs_rollup_1m_bucket_idx` — time-range filtering

**Query strategy:**
- Stats for ranges ending > 5 minutes ago → query rollup (fast)
- Stats for recent ranges → query raw `request_logs` (accurate, avoids stale MV)

---

### `audit_logs`

Immutable record of admin actions and authentication events. Written synchronously in a fire-and-forget goroutine.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | `BIGSERIAL` | PK | Auto-increment |
| `actor` | `TEXT` | NOT NULL | Who performed the action (e.g., "admin") |
| `action` | `TEXT` | NOT NULL | Action type (e.g., `CREATE_TENANT`, `ADMIN_AUTH_FAILURE`) |
| `resource_type` | `TEXT` | NOT NULL | Category of resource affected |
| `resource_id` | `TEXT` | nullable | Specific resource identifier |
| `details` | `JSONB` | nullable | Structured metadata about the event |
| `ip_address` | `INET` | nullable | Client IP address |
| `user_agent` | `TEXT` | nullable | Client User-Agent string |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | Event timestamp |

**Indexes:**
- `idx_audit_logs_created_at DESC` — chronological listing
- `idx_audit_logs_action` — filter by action type
- `idx_audit_logs_resource` — filter by resource type and ID
- `idx_audit_logs_actor` — filter by actor

**Recorded events:**
| Action | Trigger |
|--------|---------|
| `ADMIN_AUTH_FAILURE` | Invalid `X-Admin-Secret` submitted |
| `CREATE_TENANT` | New tenant created via API or dashboard |
| `UPDATE_TENANT` | Tenant quotas or network assignment changed |
| `DELETE_TENANT` | Tenant permanently removed |
| `ROTATE_API_KEY` | Tenant API key rotated |
| `CREATE_BLOCKCHAIN_CONFIG` | New network added |
| `UPDATE_BLOCKCHAIN_CONFIG` | Network endpoints or status changed |
| `DELETE_BLOCKCHAIN_CONFIG` | Network removed |

---

## Key Design Decisions

### 1. Separate `api_keys` Table (Migration 004)

**Why:** The original `tenants.api_key` stored plaintext. Migration 004:
- Made `tenants.api_key` nullable (backward compatibility)
- Created `api_keys` with bcrypt hashed storage
- Enables key rotation (multiple rows per tenant over time)
- Supports multiple named keys per tenant (future-proofing)

### 2. Composite Primary Key on `blocks` (Migration 003)

**Why:** Different networks can have the same block number (e.g., Ethereum mainnet block 100 ≠ Sepolia block 100). Changed from `number` PK to `(number, network_id)` unique constraint.

### 3. Materialized View for Stats (Migration 008)

**Why:** `request_logs` grows unbounded. Aggregating on every dashboard load would be O(n) on a large table. The MV reduces this to O(minutes in range) with `REFRESH CONCURRENTLY` for non-blocking updates.

**Trade-off:** Stats for the last 5 minutes are queried from raw table to avoid stale MV data.

### 4. Nullable `network_id` in Request Logs

**Why:** A request may fail before a network is determined (e.g., invalid auth, missing config). `ON DELETE SET NULL` preserves logs even if a network is deleted.

### 5. `ON DELETE CASCADE` on Tenant FKs

**Why:** Deleting a tenant should cleanly remove all associated data (keys, usage, logs). This is intentional — no soft-delete on tenants.

### 6. Audit Logging (Migration 002)

**Why:** Admin actions modify tenant credentials and network routing. An immutable audit trail is required for security review and compliance. Writes are fire-and-forget (3-second timeout) so they never block the admin API response.

---

## Query Examples

### Daily usage for a tenant
```sql
SELECT method, SUM(count) as total, SUM(bytes_in) as bytes
FROM usage
WHERE tenant_id = '...' AND DATE(period) = DATE('2026-08-19')
GROUP BY method;
```

### Recent error rate
```sql
SELECT 
  status,
  COUNT(*) as count,
  AVG(latency_ms) as avg_latency
FROM request_logs
WHERE created_at >= NOW() - INTERVAL '1 hour'
GROUP BY status;
```

### Cache hit ratio from rollup
```sql
SELECT 
  SUM(cache_hits)::float / NULLIF(SUM(requests), 0) as hit_ratio
FROM request_logs_rollup_1m
WHERE bucket >= NOW() - INTERVAL '1 hour';
```

### Unhealthy nodes (from health endpoint data)
```sql
-- This is application-level; health state lives in memory
-- Blocks table shows ingestion gaps:
SELECT network_id, MAX(number) as latest, COUNT(*) as total_24h
FROM blocks
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY network_id;
```

### Recent admin audit events
```sql
SELECT action, resource_type, resource_id, details, ip_address, created_at
FROM audit_logs
WHERE created_at >= NOW() - INTERVAL '24 hours'
ORDER BY created_at DESC
LIMIT 50;
```

---

## Related Documents

- [Architecture Overview](overview.md) — Component map and tech stack
- [Data Flow](data-flow.md) — How data moves through the system
- [How to Deploy](../HowToDeploy.md) — Database setup and migrations
