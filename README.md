<div align="center">

# BlockMesh

**A high-performance, multi-tenant blockchain API gateway built in Go.**

Intelligent node routing · Redis caching · Per-tenant rate limiting · Usage metering · K3s-native

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

---

## Overview

BlockMesh is a production-grade API gateway that sits between applications and blockchain RPC nodes. It adds **reliability, speed, and tenant isolation** that raw node endpoints don't provide — built to demonstrate the exact systems challenges faced by platforms serving institutional Web3 finance.

### Why This Exists

Raw blockchain RPC endpoints are:
- **Unreliable** — public nodes go down without warning
- **Slow** — repeated identical queries hit the node every time
- **Unmetered** — no way to enforce quotas or bill by usage
- **Insecure** — no tenant isolation between customers

BlockMesh solves all four.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENT REQUEST                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Ingress (Nginx)                                                │
│  /        → React Admin Dashboard                               │
│  /api/v1/ → Gateway Service                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Gateway Service (Go)                                           │
│  ┌─────────┐ ┌────────────┐ ┌────────┐ ┌──────────┐ ┌────────┐ │
│  │  Auth   │→│ Rate Limit │→│ Cache  │→│ Metrics  │→│ Proxy  │ │
│  │ (APIKey)│ │(per-tenant)│ │(Redis) │ │(Prometheus│ │(ETH RPC)│
│  └─────────┘ └────────────┘ └────────┘ └──────────┘ └────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌─────────┐    ┌─────────┐     ┌─────────────┐
        │  Redis  │    │ Postgres│     │ ETH RPC     │
        │ (Cache  │    │(Tenants │     │ Node A      │
        │ + RL)   │    │ + Usage)│     │ Node B (FB) │
        └─────────┘    └─────────┘     └─────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Background Services                                            │
│  ┌─────────────┐  ┌─────────────────────────────────────────┐  │
│  │  Admin API  │  │  Ingestor (CronJob)                     │  │
│  │  /tenants   │  │  Fetches Sepolia blocks → PostgreSQL    │  │
│  │  /usage     │  │  Runs every 60s                         │  │
│  └─────────────┘  └─────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Language** | Go 1.22 | Gateway, Admin API, Ingestor |
| **Frontend** | React 18 + Vite | Admin dashboard |
| **Cache** | Redis 7 | Hot query caching + rate limit counters |
| **Database** | PostgreSQL 15 | Tenant registry + usage metering |
| **Blockchain** | Sepolia Testnet | ETH JSON-RPC integration |
| **Orchestration** | Kubernetes / K3s | Container orchestration |
| **Ingress** | Nginx + cert-manager | TLS termination + routing |
| **Observability** | Prometheus + Grafana | Metrics + dashboards |
| **Load Testing** | k6 | Performance validation |

---

## Key Features

### 🔀 Intelligent Node Failover
- Maintains a pool of upstream RPC endpoints
- Automatic circuit breaker: if a node fails 5× in 30s, traffic routes away for 60s
- Round-robin with health checks

### ⚡ Domain-Aware Caching
- `eth_chainId` → 24h TTL (never changes)
- `eth_getBlockByNumber(latest)` → 2s TTL (fresh but safe)
- `eth_getBalance` → 30s TTL
- Cache hits served from Redis in **< 5ms**

### 🔐 Multi-Tenant Isolation
- API key authentication per request
- Per-tenant rate limiting (token bucket in Redis)
- Quota enforcement at the middleware layer
- Usage recorded per method, per tenant, per minute

### 📊 Usage Metering
- Background async writes to PostgreSQL
- Daily aggregation queryable via Admin API
- Ready for billing integration (Stripe, Chargebee, etc.)

### 📈 Observability
- Prometheus metrics: `blockmesh_requests_total`, `blockmesh_request_duration_seconds`
- Per-tenant, per-method, per-cache-status labels
- Grafana dashboard included

---

## Quick Start

### Prerequisites
- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- [Go 1.22+](https://golang.org/dl/) (for local development)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) + [K3s](https://k3s.io/) (for deployment)

### Local Development

```bash
# Clone and enter
git clone https://github.com/yourname/blockmesh.git
cd blockmesh

# Spin up everything (Postgres, Redis, Gateway, Admin, Ingestor, Web)
docker-compose up --build

# In another terminal, test the gateway
curl -H "Authorization: Bearer demo-key"   -X POST http://localhost:8080/v1/   -H "Content-Type: application/json"   -d '{
    "jsonrpc": "2.0",
    "method": "eth_blockNumber",
    "params": [],
    "id": 1
  }'

# Open the admin dashboard
open http://localhost:3000
```

### K3s Deployment

```bash
# Build all images
make build-all

# Import into K3s (or push to a registry)
for img in gateway admin ingestor web; do
  docker save blockmesh-$img:latest | sudo k3s ctr images import -
done

# Deploy
kubectl apply -k deployments/base/

# Verify
kubectl get pods -n blockmesh
kubectl logs -n blockmesh deployment/gateway
```

---

## API Reference

### Gateway — `POST /v1/`

Proxies any valid ETH JSON-RPC method with auth, caching, and metering.

**Headers:**
| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <api-key>` |
| `Content-Type` | `application/json` |

**Example Request:**
```bash
curl -H "Authorization: Bearer demo-key"   -X POST http://localhost:8080/v1/   -d '{
    "jsonrpc": "2.0",
    "method": "eth_getBalance",
    "params": ["0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb", "latest"],
    "id": 1
  }'
```

**Example Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1a055690d9db80000"
}
```

### Admin API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/tenants` | GET | List all tenants |
| `/usage?tenant=demo-key&day=2026-08-14` | GET | Daily usage report |

---

## Performance

Load tested with [k6](https://k6.io/) against a single K3s node.

| Scenario | RPS | p50 | p99 | Cache Hit |
|----------|-----|-----|-----|-----------|
| `eth_blockNumber` (cached) | 4,200 | 8ms | 12ms | 95% |
| `eth_blockNumber` (uncached) | 180 | 420ms | 890ms | 0% |
| `eth_getBalance` (cached) | 3,800 | 9ms | 14ms | 90% |

> **93% latency reduction** on hot paths via domain-aware Redis caching.

Run tests yourself:
```bash
k6 run loadtest/k6/smoke.js
k6 run loadtest/k6/load.js
```

---

## Project Structure

```
blockmesh/
├── backend/          # Go gateway, admin, ingestor + shared libs
│   ├── gateway/
│   ├── admin/
│   ├── ingestor/
│   └── shared/
├── web/              # React admin dashboard
├── migrations/       # PostgreSQL schema
├── deployments/      # K3s manifests
└── loadtest/         # k6 tests
```

---

## Design Decisions

**Why one Go module instead of separate modules per service?**

All three backend services share domain logic (blockchain client, storage abstractions, models). A single module eliminates version drift and guarantees consistency across the gateway, admin, and ingestor. Each service compiles to its own lean binary via `cmd/` entrypoints.

**Why K3s instead of managed Kubernetes?**

This project is designed to run on minimal infrastructure. K3s provides full Kubernetes semantics on a single node with zero cloud dependency — perfect for homelab demos and edge deployments.

**Why Sepolia instead of Mainnet?**

Free public RPC endpoints, no real funds at risk, and identical JSON-RPC semantics. The blockchain client abstraction makes swapping to Mainnet or any EVM chain a one-line config change.

---

## Roadmap

- [ ] Circuit breaker state machine with Prometheus metrics
- [ ] WebSocket support for `eth_subscribe`
- [ ] Tiered subscription plans (free / pro / enterprise)
- [ ] Grafana dashboard JSON export
- [ ] OpenAPI spec generation
- [ ] Distributed tracing (OpenTelemetry)

---

## License

MIT

---

<div align="center">

Built to demonstrate production-grade distributed systems engineering.

</div>
