# ChainMesh Database Schema

> **Single Source of Truth:** For the machine-readable schema definition (columns, types, constraints, indexes, foreign keys, and migration history), see [`database.xml`](database.xml).
>
> This document provides the human-readable companion to that file.

---

## Migration History

The schema evolves through numbered migrations in `migrations/`. **Run them in order.**

| # | File | Description |
|---|------|-------------|
| 001 | `001_init.up.sql` | Core tables: `tenants`, `usage`, `blocks` |
| 001 | `001_init.down.sql` | Drops core tables |
| 002 | `002_audit_logs.up.sql` | Admin audit trail: `audit_logs` |
| 003 | `003_blockchain_config.up.sql` | Blockchain network configuration |
| 003 | `003_multi_network.sql` | Multi-network FKs; composite PK on `blocks` |
| 004 | `004_multi_network.up.sql` | Consolidated multi-network migration; drops `blocks_hash_key` |
| 005 | `005_api_keys.up.sql` | Separate `api_keys` table with bcrypt hashes |
| 006 | `006_request_logs.up.sqlup.sql` | Per-request audit logging |
| 007 | `007_tenant_rate_limits.up.sql` | Per-tenant `plan`, `quota_rps`, `quota_daily` |
| 008 | `008_request_id.up.sql` | Adds `request_id` to `request_logs` |
| 009 | `009_stats_rollup.up.sql` | Pre-aggregated materialized view for dashboards |

> **Note:** Migrations 003 and 004 both touch multi-network support. In practice, run `003_blockchain_config.up.sql` first, then either `003_multi_network.sql` **or** `004_multi_network.up.sql` (004 is the superset and drops the old `blocks_hash_key` unique constraint).

---

## Entity Relationship Diagram

```
┌─────────────────┐     ┌─────────────────────┐     ┌─────────────────────┐
│  tenants        │     │  api_keys           │     │  blockchain_configs │
├─────────────────┤     ├─────────────────────┤     ├─────────────────────┤
│ id (PK)         │◄────┤ tenant_id (FK)      │     │ id (PK)             │
│ name            │     │ key_hash (UNIQUE)   │     │ name                │
│ api_key (NULL)  │     │ key_prefix          │     │ rpc_endpoint_1      │
│ quota_rpm       │     │ revoked_at          │     │ rpc_endpoint_2      │
│ quota_rps       │     │ last_used_at        │     │ chain_id            │
│ quota_daily     │     └─────────────────────┘     │ enabled             │
│ plan            │                                 └──────────┬──────────┘
│ blockchain_     │◄────────────────────────────────────────────┘
│   network_id(FK)│
└───────┬─────────┘
        │
        │ 1:N
        ▼
┌─────────────────┐     ┌─────────────────────┐     ┌─────────────────────┐
│  usage          │     │  request_logs       │     │  blocks             │
├─────────────────┤     ├─────────────────────┤     ├─────────────────────┤
│ tenant_id (FK) │     │ id (PK)             │     │ number (PK)         │
│ method          │     │ tenant_id (FK)      │     │ hash                │
│ count           │     │ network_id (FK)     │     │ parent_hash         │
│ bytes_in        │     │ method              │     │ timestamp           │
│ period (PK)     │     │ status              │     │ tx_count            │
└─────────────────┘     │ latency_ms          │     │ raw_json            │
                        │ cache_hit           │     │ network_id (FK,PK)  │
                        │ bytes_in            │     │ created_at          │
                        │ request_id          │     └─────────────────────┘
                        │ created_at          │
                        └─────────────────────┘
                                    │
                                    ▼
                        ┌─────────────────────┐
                        │ audit_logs          │
                        ├─────────────────────┤
                        │ id (PK)             │
                        │ actor               │
                        │ action              │
                        │ resource_type       │
                        │ resource_id         │
                        │ details (JSONB)     │
                        │ ip_address          │
                        │ user_agent          │
                        │ created_at          │
                        └─────────────────────┘
```

---

## Table Reference

### `tenants`

The root entity for multi-tenancy. Each tenant represents a customer or application with its own API keys and quotas.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | `UUID` | PK, `uuid_generate_v4()` | System-generated |
| `name` | `VARCHAR(255)` | NOT NULL | Display name |
| `api_key` | `VARCHAR(255)` | UNIQUE, nullable | **Legacy** plaintext key (deprecated by migration 005) |
| `quota_rpm` | `INT` | NOT NULL, DEFAULT 60 | Requests per minute |
| `quota_rps` | `INT` | NOT NULL, DEFAULT 0 | Requests per second (0 = unlimited) |
| `quota_daily` | `INT` | NOT NULL, DEFAULT 0 | Requests per day (0 = unlimited) |
| `plan` | `TEXT` | NOT NULL, DEFAULT 'free' | Informational tier label |
| `blockchain_network_id` | `UUID` | FK → `blockchain_configs(id)`, nullable | Pinned network; NULL = use default |
| `created_at` | `TIMESTAMPTZ` | DEFAULT NOW() | |

**Migration notes:**
- `quota_rps`, `quota_daily`, `plan` added in migration 007
- `blockchain_network_id` added in migration 003/004
- `api_key` made nullable in migration 005 (migrated to separate table)

---

### `api_keys`

Stores hashed API keys with rotation support. A tenant may have multiple keys over time.

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
- Full API keys are never stored. Only a bcrypt hash and a 12-char prefix.
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
| `hash` | `VARCHAR(66)` | NOT NULL | 0x + 64 hex chars |
| `parent_hash` | `VARCHAR(66)` | NOT NULL | Previous block hash |
| `timestamp` | `TIMESTAMPTZ` | NOT NULL | Block time |
| `tx_count` | `INT` | NOT NULL, DEFAULT 0 | Transaction count |
| `raw_json` | `JSONB` | nullable | Full block JSON (optional) |
| `network_id` | `UUID` | FK → `blockchain_configs(id)`, nullable | Added in migration 003/004 |
| `created_at` | `TIMESTAMPTZ` | DEFAULT NOW() | Ingestion time |

**Constraints:**
- `PRIMARY KEY (number, network_id)` — allows same block number on different chains (migration 003/004)

**Indexes:**
- `idx_blocks_network_id` — fast listing by network

**Query patterns:**
- `ORDER BY number DESC LIMIT 50` — dashboard recent blocks
- `WHERE network_id = $1 ORDER BY number DESC LIMIT 1` — latest block per network

> **Note:** The original `blocks_hash_key` UNIQUE constraint was dropped in migration 004 to allow duplicate hashes across different networks (theoretical edge case for different chains).

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
| `status` | `TEXT` | NOT NULL | See status values below |
| `latency_ms` | `INTEGER` | NOT NULL, DEFAULT 0 | Round-trip time |
| `cache_hit` | `BOOLEAN` | NOT NULL, DEFAULT false | |
| `bytes_in` | `INTEGER` | NOT NULL, DEFAULT 0 | Request body size |
| `request_id` | `TEXT` | nullable | Tracing ID from X-Request-ID (added in migration 008) |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | |

**Indexes:**
- `idx_request_logs_created_at DESC` — time-series queries
- `idx_request_logs_tenant_created_at DESC` — per-tenant history
- `idx_request_logs_method` — method analysis
- `idx_request_logs_status` — error analysis
- `idx_request_logs_network_created_at DESC` — per-network history
- `idx_request_logs_request_id` — tracing lookups (added in migration 008)

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
| `action` | `TEXT` | NOT NULL | Action type |
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

### 1. Separate `api_keys` Table (Migration 005)

**Why:** The original `tenants.api_key` stored plaintext. Migration 005:
- Made `tenants.api_key` nullable (backward compatibility)
- Created `api_keys` with bcrypt hashed storage
- Enables key rotation (multiple rows per tenant over time)
- Supports multiple named keys per tenant (future-proofing)

### 2. Composite Primary Key on `blocks` (Migration 003/004)

**Why:** Different networks can have the same block number (e.g., Ethereum mainnet block 100 ≠ Sepolia block 100). Changed from `number` PK to `(number, network_id)` composite PK.

### 3. Materialized View for Stats (Migration 009)

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

### Unhealthy nodes (ingestion gaps)
```sql
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

### Lookup API key by prefix (auth path)
```sql
SELECT * FROM api_keys
WHERE key_prefix = 'bm_live_abc123'
  AND revoked_at IS NULL;
```

---

## Related Documents

- [`database.xml`](database.xml) — Machine-readable schema definition
- [`architecture/overview.md`](architecture/overview.md) — Component map and tech stack
- [`architecture/data-flow.md`](architecture/data-flow.md) — How data moves through the system
- [`operators/deploy.md`](operators/deploy.md) — Database setup and migrations
