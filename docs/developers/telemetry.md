# Telemetry System

This document explains how BlockMesh records, aggregates, and serves usage data — from the moment a request hits the gateway to the materialized view that powers the dashboard.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Async Recorder](#async-recorder)
4. [Data Tables](#data-tables)
5. [Materialized View & Rollup](#materialized-view--rollup)
6. [Stats Query Strategy](#stats-query-strategy)
7. [Prometheus Metrics](#prometheus-metrics)
8. [Configuration](#configuration)
9. [Adding New Telemetry Fields](#adding-new-telemetry-fields)
10. [Troubleshooting](#troubleshooting)

---

## Overview

BlockMesh telemetry answers two questions:

1. **What happened?** — Every gateway request is logged with tenant, network, method, status, latency, and cache result.
2. **What are the trends?** — Aggregated stats power the dashboard charts, top-methods tables, and latency percentiles.

Design goals:

| Goal | Implementation |
|------|---------------|
| **Never block the gateway** | Bounded async queue with overflow drop |
| **Survive Postgres outages** | Retries with backoff; drop after max attempts |
| **Fast dashboard queries** | Pre-aggregated materialized view refreshed every 60s |
| **Accurate recent data** | Raw table queried directly for the last 5 minutes |
| **Graceful shutdown** | Cancellation + bounded drain timeout |

---

## Architecture

```
Gateway Request
│
▼
┌────────────────────────────────────────────────────────────────────┐
│  proxy.recordOutcome()                                             │
│  ├── Prometheus metrics (synchronous, in-process)                  │
│  └── telemetry.Recorder.RecordUsage() + RecordRequestLog()         │
│       │                                                            │
│       ▼                                                            │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  Bounded Channel (default: 4096 jobs)                    │      │
│  │  If full → DROP + increment TelemetryDroppedTotal        │      │
│  └──────────────────────────┬───────────────────────────────┘      │
│                             │                                      │
│                             ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  Worker Goroutine                                        │      │
│  │  ├── 3 attempts per job                                  │      │
│  │  ├── 50ms / 100ms / 150ms backoff                        │      │
│  │  ├── 5s timeout per write                                │      │
│  │  └── After 3 failures → DROP + TelemetryDroppedTotal     │      │
│  └──────────────────────────┬───────────────────────────────┘      │
│                             │                                      │
└─────────────────────────────┼──────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
     ┌─────────────────┐            ┌──────────────────┐
     │  usage table    │            │  request_logs    │
     │  (aggregated)   │            │  (raw audit)     │
     └────────┬────────┘            └────────┬─────────┘
              │                               │
              │                               ▼
              │                    ┌────────────────────────────┐
              │                    │ request_logs_rollup_1m (MV)│
              │                    │ refreshed every 60s        │
              │                    └────────────┬───────────────┘
              │                                 │
              ▼                                 ▼
     ┌──────────────────────────────────────────────────────────┐
     │  Admin API /stats/summary                                │
     │  ├── range > 5 min old → query rollup (fast)             │
     │  └── range ≤ 5 min old → query raw request_logs (exact)  │
     └──────────────────────────────────────────────────────────┘
```

---

## Async Recorder

**File:** `backend/shared/telemetry/recorder.go`

### Why Async?

The gateway serves RPC requests with < 5ms cache-hit latency. A synchronous Postgres write on every request would add 2–10ms and create a hard dependency on database availability. The recorder decouples these:

- **Gateway latency is unaffected** by Postgres write speed
- **Database outages don't cascade** — records are dropped gracefully
- **Backpressure is explicit** — queue overflow is measurable via metrics

### Queue Behavior

```go
type Recorder struct {
    db    *postgres.DB
    log   *slog.Logger
    queue chan job       // bounded channel
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}
```

| Condition | Behavior |
|-----------|----------|
| Queue has space | Job enqueued normally |
| Queue is full | Job **dropped immediately**; `TelemetryDroppedTotal` increments |
| Recorder is stopped | New jobs are **rejected**; `TelemetryDroppedTotal` increments |
| Postgres write fails | Up to 3 retries with backoff, then drop |
| Context cancelled | Worker drains remaining queue (bounded by drain timeout) |

### Retry Strategy

```
Attempt 1 → write → fail → wait 50ms
Attempt 2 → write → fail → wait 100ms
Attempt 3 → write → fail → DROP + log error + increment TelemetryDroppedTotal
```

Backoff is linear: `50 × attempt` milliseconds.

### Graceful Shutdown

When the gateway receives SIGTERM:

1. `Recorder.Stop()` is called
2. Context is cancelled → worker stops accepting new jobs from ticker
3. Worker drains remaining queue (bounded by `TELEMETRY_DRAIN_TIMEOUT`)
4. `Stop()` waits up to `TELEMETRY_SHUTDOWN_TIMEOUT` for the worker to finish
5. If timeout exceeded: remaining jobs are dropped with a warning log

```go
func (r *Recorder) Stop() {
    r.cancel()
    timeout := envDuration("TELEMETRY_SHUTDOWN_TIMEOUT", 5*time.Second)
    done := make(chan struct{})
    go func() {
        r.wg.Wait()
        close(done)
    }()
    select {
    case <-done:
        r.log.Info("telemetry worker stopped cleanly")
    case <-time.After(timeout):
        r.log.Warn("telemetry worker shutdown timed out",
            "timeout", timeout, "dropped_jobs", len(r.queue))
    }
}
```

### Fallback Path

If the telemetry recorder is nil (should not happen in production), the proxy falls back to a synchronous goroutine:

```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    p.db.RecordUsage(ctx, usage)
    p.db.RecordRequestLog(ctx, requestLog)
}()
```

This is a safety net only — it does not retry and can block briefly.

---

## Data Tables

### `usage` — Aggregated Metering

Stores per-tenant, per-method, per-minute counters. Upserted atomically.

| Column | Type | Notes |
|--------|------|-------|
| `tenant_id` | UUID | FK → tenants, CASCADE |
| `method` | VARCHAR(255) | e.g., `eth_getBalance` |
| `count` | BIGINT | Incremented per request |
| `bytes_in` | BIGINT | Total request body bytes |
| `period` | TIMESTAMPTZ | Truncated to minute |

**PK:** `(tenant_id, method, period)`

**Upsert logic:**
```sql
INSERT INTO usage (tenant_id, method, count, bytes_in, period)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, method, period)
DO UPDATE SET count = usage.count + $3, bytes_in = usage.bytes_in + $4
```

**Use case:** Daily usage reports, billing integration.

---

### `request_logs` — Raw Audit Trail

Every gateway request gets one row. This is the source of truth for the dashboard.

| Column | Type | Notes |
|--------|------|-------|
| `id` | BIGSERIAL | Auto-increment PK |
| `tenant_id` | UUID | FK → tenants, CASCADE |
| `network_id` | UUID | FK → blockchain_configs, SET NULL |
| `method` | TEXT | JSON-RPC method |
| `status` | TEXT | `success`, `rpc_error`, `upstream_error`, `invalid_request`, `network_unavailable` |
| `latency_ms` | INTEGER | Round-trip time |
| `cache_hit` | BOOLEAN | Whether served from Redis |
| `bytes_in` | INTEGER | Request body size |
| `request_id` | TEXT | Tracing ID (X-Request-ID) |
| `created_at` | TIMESTAMPTZ | Insertion time |

**Indexes:**
- `idx_request_logs_created_at DESC`
- `idx_request_logs_tenant_created_at DESC`
- `idx_request_logs_method`
- `idx_request_logs_status`
- `idx_request_logs_network_created_at DESC`
- `idx_request_logs_request_id`

**Growth:** This table grows indefinitely. See [Retention](#retention) for cleanup strategies.

---

## Materialized View & Rollup

**File:** `backend/database/migrations/008_stats_rollup.sql`
**Refresher:** `backend/shared/statsrollup/refresher.go`

### Why a Materialized View?

The dashboard queries aggregate stats across potentially millions of rows. A raw `SELECT COUNT(*) ... GROUP BY minute` on `request_logs` would be O(n) per query. The rollup pre-computes these aggregations every 60 seconds.

### Schema

```sql
CREATE MATERIALIZED VIEW request_logs_rollup_1m AS
SELECT
    date_trunc('minute', created_at) AS bucket,
    COALESCE(network_id::text, '') AS network_id,
    method,
    status,
    cache_hit,
    COUNT(*) AS requests,
    COUNT(*) FILTER (WHERE status <> 'success') AS errors,
    COUNT(*) FILTER (WHERE cache_hit) AS cache_hits,
    COALESCE(AVG(latency_ms)::float8, 0) AS avg_latency_ms,
    COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::float8, 0) AS p95_latency_ms
FROM request_logs
GROUP BY 1, 2, 3, 4, 5
WITH NO DATA;
```

**Indexes:**
- `request_logs_rollup_1m_unique` — enables `REFRESH CONCURRENTLY`
- `request_logs_rollup_1m_bucket_idx` — time-range filtering

### Refresher

Runs in the gateway process. Refreshes every 60 seconds:

```go
func (r *Refresher) refresh() {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Prefer concurrent refresh (non-blocking)
    _, err := r.db.Pool().Exec(ctx,
        `REFRESH MATERIALIZED VIEW CONCURRENTLY request_logs_rollup_1m`)
    if err == nil {
        return
    }

    // Fallback to blocking refresh
    _, err = r.db.Pool().Exec(ctx,
        `REFRESH MATERIALIZED VIEW request_logs_rollup_1m`)
    if err != nil {
        r.log.Error("materialized view refresh failed", "err", err)
    }
}
```

**Concurrency:** `REFRESH CONCURRENTLY` requires a unique index. It allows reads during refresh. Falls back to blocking refresh on first population.

**Trade-off:** p95 is stored per-minute bucket. The global p95 computed from the rollup is **approximate**, not exact.

---

## Stats Query Strategy

**File:** `backend/shared/storage/postgres/stats_store.go`

The Admin API `/stats/summary` endpoint uses a hybrid strategy:

```
┌──────────────────────────────────────────────────────┐
│  GET /stats/summary?range=1h                         │
│                                                      │
│  from = now - 1h                                     │
│  recentCutoff = now - 5min                           │
│                                                      │
│  if from < recentCutoff:                             │
│      → Try rollup first (fast)                       │
│      → Fall back to raw if rollup fails              │
│  else:                                               │
│      → Query raw request_logs (accurate)             │
└──────────────────────────────────────────────────────┘
```

**Why 5 minutes?** The rollup refreshes every 60 seconds. Data in the last 5 minutes may be stale. For "15m" and "1h" ranges, the bulk of data is older than 5 minutes, so the rollup is used. For very recent windows, raw queries ensure accuracy.

### Query Types

| Range | Source | Latency | Accuracy |
|-------|--------|---------|----------|
| 15m | Rollup (with raw fallback) | ~5ms | High (≤60s staleness) |
| 1h | Rollup (with raw fallback) | ~10ms | High |
| 24h | Rollup (with raw fallback) | ~20ms | High |
| Any (recent 5 min) | Raw `request_logs` | ~50–200ms | Exact |

### Series Granularity

| Range | Bucket Size |
|-------|-------------|
| 15m | 1 minute |
| 1h | 1 minute |
| 24h | 1 hour |

---

## Prometheus Metrics

All telemetry-related metrics are prefixed with `blockmesh_gateway_telemetry_*`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockmesh_gateway_telemetry_write_failures_total` | Counter | `kind` | Failed Postgres write attempts |
| `blockmesh_gateway_telemetry_dropped_total` | Counter | `kind` | Records dropped (queue full or retries exhausted) |

**`kind` label values:**
- `usage` — Usage aggregate record
- `request_log` — Request log record

### Request-Level Metrics (Recorded Synchronously)

These are recorded in the proxy before the async telemetry enqueue:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockmesh_gateway_requests_total` | Counter | `network_id`, `method`, `status`, `cache_hit` | Total gateway requests |
| `blockmesh_gateway_request_duration_seconds` | Histogram | `network_id`, `method`, `status`, `cache_hit` | Request latency |
| `blockmesh_gateway_cache_hits_total` | Counter | `network_id`, `method` | Cache hits |
| `blockmesh_gateway_cache_misses_total` | Counter | `network_id`, `method` | Cache misses (cacheable methods only) |
| `blockmesh_gateway_rate_limited_total` | Counter | `limit` | Rate-limited requests |
| `blockmesh_gateway_rate_limit_errors_total` | Counter | — | Redis errors during rate limit check |
| `blockmesh_gateway_upstream_requests_total` | Counter | `network_id`, `endpoint`, `method`, `status` | Upstream RPC calls |
| `blockmesh_gateway_upstream_errors_total` | Counter | `network_id`, `endpoint`, `method`, `reason` | Upstream transport errors |
| `blockmesh_gateway_upstream_request_duration_seconds` | Histogram | `network_id`, `endpoint`, `method` | Upstream latency |
| `blockmesh_gateway_node_healthy` | Gauge | `network_id`, `endpoint` | Node health (1=up, 0=down) |

### Alerting Rules

```yaml
# Alert if telemetry is dropping records
- alert: TelemetryDroppingRecords
  expr: rate(blockmesh_gateway_telemetry_dropped_total[5m]) > 0
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Telemetry dropping records"
    description: "{{ $labels.kind }} records being dropped at {{ $value }}/s"

# Alert if Postgres writes are failing
- alert: TelemetryWriteFailures
  expr: rate(blockmesh_gateway_telemetry_write_failures_total[5m]) > 1
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Telemetry Postgres writes failing"
    description: "Check Postgres connectivity and disk space"
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TELEMETRY_SHUTDOWN_TIMEOUT` | `5s` | Max time to wait for worker to finish on shutdown |
| `TELEMETRY_DRAIN_TIMEOUT` | `2s` | Max time to drain queue after cancellation |

### Queue Size

The queue buffer size is set in code when the recorder is created:

```go
telemetryRecorder := telemetry.New(db, log, 10000)
```

**Default:** 4096 jobs. **Current production value:** 10,000 jobs.

To change, edit `backend/gateway/main.go`:
```go
telemetryRecorder := telemetry.New(db, log, 20000) // larger buffer
```

### Tuning Guidance

| Scenario | Recommendation |
|----------|---------------|
| High traffic (>10K req/s) | Increase queue to 20,000–50,000 |
| Slow Postgres | Increase `TELEMETRY_SHUTDOWN_TIMEOUT` to 10s |
| Frequent drops | Check Postgres connection pool size; increase `POSTGRES_MAX_CONNS` |
| Fast shutdown needed | Decrease timeouts (accept more drops) |

---

## Adding New Telemetry Fields

### Step 1: Update the Model

Add the field to the appropriate struct in `backend/shared/model/`:

```go
// backend/shared/model/request_log.go
type RequestLog struct {
    ID        int64     `json:"id"`
    TenantID  string    `json:"tenant_id"`
    NetworkID string    `json:"network_id,omitempty"`
    Method    string    `json:"method"`
    Status    string    `json:"status"`
    LatencyMS int64     `json:"latency_ms"`
    CacheHit  bool      `json:"cache_hit"`
    BytesIn   int64     `json:"bytes_in"`
    RequestID string    `json:"request_id,omitempty"`
    NewField  string    `json:"new_field,omitempty"`  // ← your field
    CreatedAt time.Time `json:"created_at"`
}
```

### Step 2: Create a Migration

```sql
-- backend/database/migrations/009_add_new_field.sql
ALTER TABLE request_logs
    ADD COLUMN IF NOT EXISTS new_field TEXT;

CREATE INDEX IF NOT EXISTS idx_request_logs_new_field
    ON request_logs(new_field);
```

### Step 3: Update the Insert Query

Edit `backend/shared/storage/postgres/request_log_store.go`:

```go
func (d *DB) RecordRequestLog(ctx context.Context, l *model.RequestLog) error {
    _, err := d.pool.Exec(ctx,
        `INSERT INTO request_logs (
            tenant_id, network_id, method, status,
            latency_ms, cache_hit, bytes_in, request_id, new_field
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
        l.TenantID, networkID, l.Method, l.Status,
        l.LatencyMS, l.CacheHit, l.BytesIn, requestID, l.NewField,
    )
    return err
}
```

### Step 4: Populate the Field

Set the value in `backend/gateway/proxy/proxy.go` → `recordOutcome()`:

```go
p.telemetry.RecordRequestLog(&model.RequestLog{
    TenantID:  tenant.ID,
    NetworkID: networkID,
    Method:    method,
    Status:    status,
    LatencyMS: latency.Milliseconds(),
    CacheHit:  cacheHit,
    BytesIn:   bytesIn,
    RequestID: requestID,
    NewField:  computedValue,  // ← populate here
})
```

### Step 5: Update Stats Queries (If Needed)

If the field should appear in stats or the rollup:

1. Add to `request_logs_rollup_1m` in a new migration (recreate MV)
2. Update `getStatsSummaryFromRaw()` and `getStatsSummaryFromRollup()` in `stats_store.go`
3. Update the `StatsSummary` model if exposed via API

### Step 6: Update OpenAPI Spec

If the field is exposed in API responses, update `backend/api/openapi.yaml`.

---

## Retention

### The Problem

`request_logs` grows unbounded. At 1,000 req/s, that's ~86M rows per day. Without retention:

- Disk fills up
- Queries slow down (even indexed)
- Materialized view refresh takes longer

### Recommended Strategies

#### Option 1: Periodic Delete (Simple)

```sql
-- Run daily via pg_cron or application cron
DELETE FROM request_logs
WHERE created_at < NOW() - INTERVAL '90 days';

VACUUM request_logs;
```

#### Option 2: Table Partitioning (Production)

```sql
-- Create monthly partitions
CREATE TABLE request_logs (
    id BIGSERIAL,
    tenant_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    -- ... other columns
) PARTITION BY RANGE (created_at);

-- Monthly partitions
CREATE TABLE request_logs_2026_08 PARTITION OF request_logs
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

CREATE TABLE request_logs_2026_09 PARTITION OF request_logs
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');

-- Drop old partitions (instant, no VACUUM needed)
DROP TABLE request_logs_2026_05;
```

#### Option 3: TimescaleDB (Advanced)

For very high-volume deployments, consider TimescaleDB for automatic partitioning and compression.

### Usage Table Retention

The `usage` table grows more slowly (one row per tenant-method-minute). Retention is less critical but recommended:

```sql
DELETE FROM usage WHERE period < NOW() - INTERVAL '365 days';
```

---

## Troubleshooting

### Telemetry is dropping records

**Symptoms:** `blockmesh_gateway_telemetry_dropped_total` is increasing.

**Causes:**
1. Queue is full (traffic exceeds write capacity)
2. Postgres is slow or down
3. Connection pool exhausted

**Solutions:**
```bash
# Check Postgres connection count
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c \
    "SELECT count(*) FROM pg_stat_activity WHERE datname = 'blockmesh';"

# Increase connection pool
export POSTGRES_MAX_CONNS=50

# Increase queue size in gateway/main.go
# telemetryRecorder := telemetry.New(db, log, 20000)
```

### Dashboard stats are stale

**Symptoms:** Stats don't update for > 2 minutes.

**Causes:**
1. Materialized view refresh is failing
2. Rollup refresher stopped

**Solutions:**
```bash
# Check gateway logs for refresh errors
docker compose logs gateway | grep "materialized view"

# Manual refresh
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c \
    "REFRESH MATERIALIZED VIEW CONCURRENTLY request_logs_rollup_1m;"

# Check if refresher is running (should log every 60s on errors)
docker compose logs gateway | grep "statsrollup"
```

### Usage reports show zero

**Symptoms:** `/tenants/{id}/usage` returns empty array.

**Causes:**
1. Telemetry worker is not running
2. Tenant ID mismatch
3. No requests made in the queried time window

**Debug:**
```bash
# Check raw request_logs
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c \
    "SELECT count(*) FROM request_logs WHERE tenant_id = '<id>' AND created_at > NOW() - INTERVAL '1 hour';"

# Check usage table
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c \
    "SELECT * FROM usage WHERE tenant_id = '<id>' ORDER BY period DESC LIMIT 5;"
```

### High p95 latency in stats

**Note:** p95 from the rollup is **approximate** (computed per-minute bucket, then MAX across buckets). For exact p95, query raw `request_logs`:

```sql
SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95
FROM request_logs
WHERE created_at >= NOW() - INTERVAL '1 hour';
```

### Worker shutdown timeout

**Symptoms:** Log message `"telemetry worker shutdown timed out"`.

**Cause:** Postgres writes are taking too long during shutdown.

**Solution:**
```bash
# Increase shutdown timeout
export TELEMETRY_SHUTDOWN_TIMEOUT=10s
export TELEMETRY_DRAIN_TIMEOUT=5s
```

Or accept the drops if fast shutdown is more important than telemetry completeness.

---

## Code Reference

| File | Purpose |
|------|---------|
| `backend/shared/telemetry/recorder.go` | Async bounded queue worker |
| `backend/shared/statsrollup/refresher.go` | Materialized view refresh loop |
| `backend/shared/metrics/metrics.go` | Prometheus metric definitions |
| `backend/shared/storage/postgres/usage_store.go` | Usage upsert queries |
| `backend/shared/storage/postgres/request_log_store.go` | Request log inserts |
| `backend/shared/storage/postgres/stats_store.go` | Stats summary queries (hybrid) |
| `backend/gateway/proxy/proxy.go` | `recordOutcome()` — telemetry entry point |
| `backend/database/migrations/005_request_logs.sql` | Request logs table |
| `backend/database/migrations/008_stats_rollup.sql` | Materialized view definition |

---

## Related Documents

- [Architecture Overview](../architecture/overview.md) — System components and design principles
- [Data Flow](../architecture/data-flow.md) — Request lifecycle including telemetry recording
- [Database Schema](../architecture/schema.md) — Table structures and relationships
- [Developer Setup](setup.md) — Local development environment
- [Contributing](contributing.md) — Coding standards and PR process