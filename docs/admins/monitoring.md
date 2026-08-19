# Monitoring & Observability

Guide to monitoring BlockMesh health, performance, and usage through the dashboard and metrics.

---

## Dashboard Overview

The BlockMesh dashboard provides real-time visibility into:

- **System health** — Node status and connectivity
- **Request statistics** — Volume, errors, cache performance, latency
- **Tenant activity** — Usage by method and time period
- **Block ingestion** — Recently indexed blocks across networks

Access the dashboard at your configured domain (e.g., `http://localhost` or `https://rpc.yourdomain.com`).

---

## Dashboard Sections

### Stats Strip

The top bar shows at-a-glance metrics:

| Metric | Description |
|--------|-------------|
| **Tenants** | Total number of configured tenants (admin only) |
| **Latest block** | Highest block number across all networks |
| **Tx in recent blocks** | Total transactions in the last 50 ingested blocks |

---

### Monitoring Section

**Path:** Dashboard → Observability → Monitoring

Provides detailed request analytics with selectable time ranges: **15 minutes**, **1 hour**, **24 hours**.

#### Stat Cards

| Metric | Description |
|--------|-------------|
| **Requests** | Total requests in the selected range |
| **Errors** | Total failed requests (upstream, RPC, network errors) |
| **Cache hit rate** | Percentage of requests served from Redis cache |
| **Avg latency** | Mean response time in milliseconds |
| **p95 latency** | 95th percentile response time |

#### Request Volume Chart

A bar chart showing requests over time:
- **Blue bars** — Successful requests
- **Red bars** — Requests with errors

Hover over bars to see exact counts per time bucket.

#### Top Tables

| Table | Shows |
|-------|-------|
| **Top methods** | Most frequently called JSON-RPC methods |
| **Top networks** | Most active blockchain networks |
| **Top statuses** | Distribution of response statuses |

**Data source:** For ranges ending > 5 minutes ago, queries the `request_logs_rollup_1m` materialized view. For recent data, queries the raw `request_logs` table directly.

**Refresh:** Auto-refreshes every 15 seconds. Manual refresh available via the **Refresh** button.

---

### Node Status Section

**Path:** Dashboard → Infrastructure → Node status

Shows real-time health for all configured RPC endpoints:

| Column | Description |
|--------|-------------|
| **Endpoint** | Redacted URL (host only, path hidden) |
| **Status** | Healthy (green) or Down (red) |
| **Latency** | Last probe response time in ms |
| **Fails** | Consecutive failed health checks |
| **Requests** | Total requests routed to this endpoint |

**Refresh:** Auto-refreshes every 10 seconds. Manual refresh available.

**Health check mechanics:**
- Probes `eth_chainId` every 10 seconds
- 3 consecutive failures marks endpoint as Down
- 1 successful probe marks endpoint as Healthy
- Endpoints are sorted by latency for routing

---

### Blocks Section

**Path:** Dashboard → Chain data → Recent blocks

Lists the 50 most recently ingested blocks:

| Column | Description |
|--------|-------------|
| **Network** | Network name or ID prefix |
| **Number** | Block number |
| **Hash** | Block hash (truncated, hover for full) |
| **Txs** | Transaction count |
| **Time** | Ingestion timestamp |

The ingestor polls each enabled network every 12 seconds for the latest block.

---

## Admin API Stats Endpoints

### Stats Summary

```bash
curl "http://localhost:8081/stats/summary?range=1h" \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

**Parameters:**
- `range` — `15m`, `1h` (default), or `24h`

**Response:**
```json
{
  "range": "1h",
  "from": "2026-08-19T13:00:00Z",
  "to": "2026-08-19T14:00:00Z",
  "totals": {
    "requests": 15420,
    "success": 15200,
    "errors": 220,
    "cache_hits": 8900,
    "cache_misses": 6520
  },
  "latency": {
    "avg_ms": 45.2,
    "p95_ms": 120.5
  },
  "top_methods": [
    { "name": "eth_getBalance", "count": 5200 },
    { "name": "eth_call", "count": 4100 }
  ],
  "top_networks": [
    { "name": "Ethereum Mainnet", "count": 12000 },
    { "name": "Sepolia", "count": 3420 }
  ],
  "top_statuses": [
    { "name": "success", "count": 15200 },
    { "name": "rpc_error", "count": 150 },
    { "name": "upstream_error", "count": 70 }
  ],
  "series": [
    {
      "time": "2026-08-19T13:00:00Z",
      "requests": 250,
      "errors": 3,
      "cache_hits": 145
    }
  ]
}
```

### Node Health

```bash
curl http://localhost:8080/health/nodes
```

No authentication required — useful for external monitoring tools.

---

## Prometheus Metrics

BlockMesh exposes Prometheus-compatible metrics for scraping.

### Gateway Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockmesh_gateway_requests_total` | Counter | `network_id`, `method`, `status`, `cache_hit` | Total requests |
| `blockmesh_gateway_request_duration_seconds` | Histogram | `network_id`, `method`, `status`, `cache_hit` | Request latency |
| `blockmesh_gateway_cache_hits_total` | Counter | `network_id`, `method` | Cache hits |
| `blockmesh_gateway_cache_misses_total` | Counter | `network_id`, `method` | Cache misses |
| `blockmesh_gateway_rate_limited_total` | Counter | `limit` | Rate-limited requests |
| `blockmesh_gateway_rate_limit_errors_total` | Counter | — | Redis errors during rate limit check |
| `blockmesh_gateway_node_healthy` | Gauge | `network_id`, `endpoint` | 1 = healthy, 0 = unhealthy |

### Upstream Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockmesh_gateway_upstream_requests_total` | Counter | `network_id`, `endpoint`, `method`, `status` | Upstream calls |
| `blockmesh_gateway_upstream_errors_total` | Counter | `network_id`, `endpoint`, `method`, `reason` | Transport/parsing errors |
| `blockmesh_gateway_upstream_request_duration_seconds` | Histogram | `network_id`, `endpoint`, `method` | Upstream latency |

### Telemetry Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `blockmesh_gateway_telemetry_write_failures_total` | Counter | `kind` | Failed Postgres writes |
| `blockmesh_gateway_telemetry_dropped_total` | Counter | `kind` | Dropped records (queue full) |

---

## Alerting Recommendations

### Critical Alerts

| Condition | Threshold | Action |
|-----------|-----------|--------|
| All nodes unhealthy | `node_healthy == 0` for all endpoints | Page on-call; check upstream providers |
| High error rate | `errors / requests > 0.05` for 5m | Investigate upstream issues |
| Rate limiter down | `rate_limit_errors_total` increasing | Check Redis connectivity |
| Telemetry dropping | `telemetry_dropped_total` increasing | Check Postgres health |

### Warning Alerts

| Condition | Threshold | Action |
|-----------|-----------|--------|
| Cache hit rate drop | `< 30%` for 10m | Review cacheable method usage |
| High p95 latency | `> 500ms` for 5m | Check upstream node performance |
| Single node unhealthy | `node_healthy == 0` for one endpoint | Verify endpoint; failover active |

---

## Log Analysis

### Gateway Logs

```bash
docker compose logs gateway | grep -E "(upstream failed|rpc invalid|rate limit)"
```

Key log patterns:
- `upstream failed` — Transport error to RPC node
- `rpc invalid response` — Node returned unparseable JSON
- `health check failed` — Endpoint failing probes

### Admin Logs

```bash
docker compose logs admin | grep -E "(stats query failed|database error)"
```

### Ingestor Logs

```bash
docker compose logs ingestor | grep -E "(fetch failed|store block failed)"
```

---

## Materialized View Refresh

The `request_logs_rollup_1m` view refreshes automatically every 60 seconds. To manually refresh:

```bash
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c \
  "REFRESH MATERIALIZED VIEW CONCURRENTLY request_logs_rollup_1m;"
```

If concurrent refresh fails (e.g., first population), it falls back to blocking refresh.

---

## Dashboard Polling Behavior

| Section | Interval | Behavior |
|---------|----------|----------|
| Monitoring | 15s | Pauses when tab is inactive; resumes on visibility |
| Node Status | 10s | Same visibility-aware behavior |
| Blocks | On mount only | Static after load |
| Tenants | On mount + after mutations | Manual refresh not needed |

All polling uses `AbortController` to cancel in-flight requests when navigating away or when a newer request is made.

---

## Performance Baselines

| Metric | Expected | Investigation Threshold |
|--------|----------|------------------------|
| Cache hit rate | > 40% | < 20% |
| Avg latency | < 100ms | > 300ms |
| p95 latency | < 250ms | > 1000ms |
| Error rate | < 1% | > 5% |
| Node health check latency | < 200ms | > 1000ms |
| Ingestor block lag | < 30s | > 2 minutes |

---

## Related Documents

- [Managing Tenants](add-tenants.md) — Tenant usage and quotas
- [Managing Networks](add-networks.md) — Network health and configuration
- [Security](../operators/security.md) — Metrics endpoint protection
- [Upgrades](../operators/upgrade.md) — Zero-downtime monitoring during deploys
