# Architecture Overview

ChainMesh is composed of four Go services, one React frontend, PostgreSQL, and Redis. Each component has a single responsibility and communicates through well-defined interfaces.

---

## System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ChainMesh Stack                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐               │
│   │    Web UI    │────▶│   Nginx      │────▶│  Admin API   │               │
│   │  (React/Vite)│     │  (reverse    │     │   (:8081)    │               │
│   │   :80/:3000  │     │   proxy)     │     │              │               │
│   └──────────────┘     └──────────────┘     └──────┬───────┘               │
│                                                     │                        │
│   ┌──────────────┐     ┌──────────────┐            │                        │
│   │   Gateway    │────▶│   Redis      │◀───────────┘                        │
│   │   (:8080)    │     │   (:6379)    │  (rate limits, cache)               │
│   └──────┬───────┘     └──────────────┘                                     │
│          │                                                                   │
│   ┌──────┴───────┐     ┌──────────────┐     ┌──────────────┐               │
│   │  Ingestor    │────▶│  PostgreSQL  │◀────│  Admin API   │               │
│   │  (blocks)    │     │   (:5432)    │     │  (tenants,   │               │
│   └──────────────┘     │              │     │   stats)     │               │
│                        └──────────────┘     └──────────────┘               │
│                                                                              │
│   ┌──────────────────────────────────────────────────────────────┐          │
│   │              Upstream RPC Nodes (external)                    │          │
│   │   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │          │
│   │   │  Endpoint 1  │    │  Endpoint 2  │    │   ...        │  │          │
│   │   │ e.g. Alchemy │    │ e.g. Infura  │    │              │  │          │
│   │   └──────────────┘    └──────────────┘    └──────────────┘  │          │
│   └──────────────────────────────────────────────────────────────┘          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

### Gateway (`backend/gateway/`)
- **Port:** `8080`
- **Role:** Public-facing JSON-RPC proxy
- **Key behaviors:**
  - Authenticates requests via `Authorization: Bearer <api-key>`
  - Enforces rate limits (RPS, RPM, daily) using Redis
  - Serves cache hits from Redis in < 5ms
  - Routes to the fastest healthy upstream node
  - Records usage and request logs asynchronously
  - Exposes `/health/nodes` for node health status

### Admin API (`backend/admin/`)
- **Port:** `8081`
- **Role:** Internal management API
- **Key behaviors:**
  - Protected by `X-Admin-Secret` header (constant-time comparison)
  - CRUD for tenants, blockchain networks, and usage data
  - Stats aggregation via raw queries or materialized views
  - Connection testing for RPC endpoints (`eth_chainId` probe)
  - Audit logging for all state-changing operations and auth failures
  - Exposes `/audit-logs` for security review

### Ingestor (`backend/ingestor/`)
- **Role:** Background block indexer
- **Key behaviors:**
  - Polls each enabled network every 12 seconds
  - Calls `eth_getBlockByNumber("latest", false)` for lightweight responses
  - Stores block metadata (number, hash, tx count, timestamp) in Postgres
  - Handles RPC-level errors gracefully before parsing

### Web Dashboard (`src/`)
- **Port:** `80` (Nginx) or `3000` (Vite dev)
- **Role:** React-based admin console
- **Key behaviors:**
  - Viewer mode: read-only access to blocks and node health (no auth)
  - Admin mode: full tenant/network management (requires `ADMIN_SECRET`)
  - Polling for real-time stats and node health
  - Role-based rendering via session storage

### PostgreSQL (`backend/database/migrations/`)
- **Role:** Persistent state
- **Stores:** tenants, hashed API keys, request logs, usage aggregates, blocks, network configs, audit logs
- **Key feature:** `request_logs_rollup_1m` materialized view for fast dashboard queries

### Redis
- **Role:** Ephemeral state
- **Stores:** rate limit counters, RPC response cache
- **Key patterns:**
  - `rl:rps:<tenant>:<unix>` — per-second counters (2s TTL)
  - `rl:min:<tenant>:<YYYY-MM-DDTHH:MM>` — per-minute counters (120s TTL)
  - `rl:day:<tenant>:<YYYY-MM-DD>` — daily counters (25h TTL)
  - `rpc:<network>:<method>:<params>` — cached RPC responses

---

## Design Principles

### 1. Fail Closed
If Redis is unavailable, the rate limiter returns `503` rather than allowing unlimited traffic. If the admin secret is unset, the admin API refuses to start.

### 2. Never Block on Telemetry
Usage recording and request logging happen through a bounded async queue. If Postgres is slow or down, records are dropped (with Prometheus counters) rather than stalling gateway requests.

### 3. Zero-Downtime Config Reload
The gateway reloads blockchain network configurations every 15 seconds without restart. New networks are picked up; removed networks are drained gracefully.

### 4. Secure by Default
- API keys are bcrypt hashed with a visible prefix
- Admin secret uses `subtle.ConstantTimeCompare`
- RPC endpoint URLs are redacted in health responses (host only, path hidden)
- SSRF protection blocks loopback, private, and link-local IPs from RPC configs
- Session-only storage for dashboard credentials
- Immutable audit logging for all admin actions

### 5. Multi-Network, Multi-Tenant
Each tenant can be pinned to a specific network or fall back to the default (earliest enabled by `created_at`). Blocks are stored per-network via composite unique key `(number, network_id)`.

---

## Technology Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Gateway / Admin / Ingestor | Go | 1.22+ |
| HTTP Router | `net/http` + `http.ServeMux` | stdlib |
| Postgres Driver | `jackc/pgx/v5` | v5 |
| Redis Client | `redis/go-redis/v9` | v9 |
| Metrics | `prometheus/client_golang` | — |
| Dashboard | React + TypeScript + Vite | 18 / 5.4 |
| Database | PostgreSQL | 15 |
| Cache | Redis | 7 |
| Reverse Proxy | Nginx (prod) / Vite (dev) | — |

---

## Related Documents

- [Data Flow](data-flow.md) — Request lifecycle in detail
- [Database Schema](schema.md) — Entity relationships and migrations
- [How to Deploy](../operators/deploy.md) — Docker Compose and Kubernetes guides
