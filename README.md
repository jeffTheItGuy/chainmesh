
# BlockMesh

**Self-hosted, multi-tenant blockchain API gateway.**

Intelligent routing · Redis caching · Per-tenant rate limits · Usage metering · One-command deploy

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

---

## What It Is

BlockMesh is a production-grade API gateway that sits between your applications and blockchain RPC nodes. It adds **reliability, speed, and tenant isolation** that raw node endpoints don't provide.

Raw RPC endpoints are:
- **Unreliable** — public nodes go down without warning
- **Slow** — repeated identical queries hit the node every time
- **Unmetered** — no way to enforce quotas or bill by usage
- **Insecure** — no tenant isolation between customers

BlockMesh solves all four. Run it on your own server.

---

## Quick Start

### Docker Compose (Single Server)

```bash
git clone https://github.com/yourname/blockmesh.git
cd blockmesh
cp .env.example .env
# Edit .env — set POSTGRES_PASSWORD and DOMAIN
make install
```

That's it. The installer checks for Docker, creates your environment, and starts Postgres, Redis, Gateway, Admin API, Ingestor, Web Dashboard, and Traefik with automatic HTTPS.

### Kubernetes / K3s

```bash
# 1. Create secrets
kubectl create namespace blockmesh
kubectl create secret generic blockmesh-secrets   --from-literal=database-url='postgres://blockmesh:yourpassword@postgres:5432/blockmesh'   --from-literal=postgres-user='blockmesh'   --from-literal=postgres-password='yourpassword'   -n blockmesh

# 2. Deploy everything
kubectl apply -k deployments/base/

# 3. Verify
kubectl get pods -n blockmesh
```

---

## Self-Hosting Options

| Method | Best For | TLS | Command |
|--------|----------|-----|---------|
| **Docker Compose** | Single VPS / homelab | Traefik + Let's Encrypt | `make install` |
| **K3s** | Single-node K8s | cert-manager + Traefik | `kubectl apply -k deployments/base/` |
| **Coolify / Easypanel** | GUI-managed PaaS | Built-in | Use provided Docker Compose template |

### Pre-built Images

No need to compile. Pull from GHCR:

```bash
ghcr.io/yourname/blockmesh-gateway:latest
ghcr.io/yourname/blockmesh-admin:latest
ghcr.io/yourname/blockmesh-ingestor:latest
ghcr.io/yourname/blockmesh-web:latest
```

---

## Hardware Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 1 vCPU | 2 vCPU |
| RAM | 1 GB | 2 GB |
| Disk | 10 GB | 20 GB SSD |
| Network | Any | Stable outbound to RPC endpoints |

---

## Features

### 🔀 Intelligent Node Failover
- Pool of upstream RPC endpoints with automatic failover
- If a node fails, traffic routes to the next healthy endpoint

### ⚡ Domain-Aware Caching
- `eth_chainId` → 24h TTL
- `eth_getBlockByNumber(latest)` → 2s TTL
- `eth_getBalance` → 30s TTL
- Cache hits served from Redis in **< 5ms**

### 🔐 Multi-Tenant Isolation
- API key authentication per request
- Per-tenant rate limiting (token bucket in Redis)
- Quota enforcement at the middleware layer
- Usage recorded per method, per tenant, per minute

### 📊 Usage Metering
- Async writes to PostgreSQL
- Daily aggregation via Admin API
- Ready for billing integration

### 📈 Observability
- Prometheus metrics on all services
- Liveness probes for K8s health checks
- Grafana-compatible scraping endpoints

---

## API Reference

### Gateway — `POST /v1/`

Proxies any valid ETH JSON-RPC method with auth, caching, and metering.

**Headers:**
| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <api-key>` |
| `Content-Type` | `application/json` |

**Example:**
```bash
curl -H "Authorization: Bearer demo-key"   -X POST http://localhost:8080/v1/   -d '{
    "jsonrpc": "2.0",
    "method": "eth_getBalance",
    "params": ["0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb", "latest"],
    "id": 1
  }'
```

### Admin API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health |
| `/tenants` | GET | List all tenants |
| `/usage?tenant=demo-key&day=2026-08-14` | GET | Daily usage report |
| `/blocks` | GET | Last 50 ingested blocks |

### Web Dashboard

Visit `http://localhost:3000` (or your configured domain) for a live React dashboard showing:
- System health
- Tenant list with rate limits
- Per-tenant usage reports
- Recently ingested blocks

---

## Configuration

All services are configured via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | Postgres connection string |
| `REDIS_ADDR` | Yes | — | Redis host:port |
| `RPC_ENDPOINT_1` | Yes | — | Primary blockchain RPC |
| `RPC_ENDPOINT_2` | No | — | Fallback blockchain RPC |
| `POSTGRES_USER` | Yes | `blockmesh` | DB user |
| `POSTGRES_PASSWORD` | Yes | `blockmesh` | DB password |
| `DOMAIN` | No | `localhost` | For Traefik TLS |
| `ACME_EMAIL` | No | — | Let's Encrypt email |

Copy `.env.example` to `.env` and adjust.

---

## Backups

PostgreSQL data lives in the `pgdata` Docker volume. To back up:

```bash
docker exec blockmesh-postgres-1 pg_dump -U blockmesh blockmesh > backup.sql
```

To restore:

```bash
cat backup.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh
```

---

## Upgrading

```bash
cd blockmesh
git pull
docker compose pull
docker compose up -d
```

Zero-downtime for the gateway (2 replicas in K8s, restart policy in Compose).

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Gateway / Admin / Ingestor | Go 1.22 |
| Dashboard | React 18 + Vite |
| Cache | Redis 7 |
| Database | PostgreSQL 15 |
| Ingress | Traefik v3 (auto HTTPS) |
| Orchestration | Docker Compose or K3s |
| Metrics | Prometheus |

---

## Security

- Never commit `.env` or `deployments/base/secrets.yaml` to git
- Change `demo-key` before production use
- Use strong `POSTGRES_PASSWORD`
- Traefik handles TLS termination; no plaintext traffic

---

## License

MIT

---

Built for self-hosters who need institutional-grade blockchain infrastructure without the cloud tax.

</div>
# BlockMesh

**Self-hosted, multi-tenant blockchain API gateway.**

Intelligent routing · Redis caching · Per-tenant rate limits · Usage metering · Role-based dashboard

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://react.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## What It Is

BlockMesh is a production-grade API gateway that sits between your applications and blockchain RPC nodes. It adds **reliability, speed, and tenant isolation** that raw node endpoints don't provide.

Raw RPC endpoints are:
- **Unreliable** — public nodes go down without warning
- **Slow** — repeated identical queries hit the node every time
- **Unmetered** — no way to enforce quotas or bill by usage
- **Insecure** — no tenant isolation between customers

BlockMesh solves all four. Run it on your own infrastructure.

---

## Key Features

### 🔀 Health-Aware Smart Routing
- Continuously probes upstream RPC nodes using background health checks (`eth_chainId`).
- Routes traffic to the fastest healthy endpoint and automatically fails over if a node goes down.

### 🌐 Multi-Network Support
- Configure and manage multiple blockchain networks (e.g., Ethereum Mainnet, Sepolia, Polygon).
- Assign specific networks to individual tenants or use a default fallback.

### 🔐 Secure Multi-Tenancy
- API keys are **SHA-256 hashed** in the database (never stored in plaintext).
- Supports key rotation, prefix-based identification, and secure constant-time comparison.
- Role-based Web Dashboard (Viewer vs. Admin).

### ⚡ Domain-Aware Caching
- Intelligent Redis caching with method-specific TTLs to drastically reduce upstream load.
- Examples: `eth_chainId` (24h), `eth_getBalance` (30s), `eth_blockNumber` (2s).

### 🛡️ Granular Rate Limiting
- Enforce per-tenant quotas for **Requests Per Second (RPS)**, **Requests Per Minute (RPM)**, and **Daily limits** using Redis.
- Returns standard `429 Too Many Requests` with `Retry-After` headers.

### 📊 Async Telemetry & Metering
- Non-blocking, bounded async workers record usage and request logs to PostgreSQL.
- Ensures gateway latency is never impacted by database write bottlenecks.
- Pre-aggregated materialized views (`request_logs_rollup_1m`) power real-time dashboard stats.

---

## Architecture

| Component | Tech | Description |
|-----------|------|-------------|
| **Gateway** | Go | HTTP proxy, middleware (Auth, Rate Limit, Request ID), caching, and telemetry. |
| **Admin API** | Go | REST API for managing tenants, networks, and viewing aggregated stats. |
| **Ingestor** | Go | Background worker that polls configured networks for the latest blocks. |
| **Web Dashboard** | React / Vite / Nginx | Frontend application with role-based access control. Nginx acts as a reverse proxy. |
| **Database** | PostgreSQL 15 | Persistent storage for tenants, hashed API keys, request logs, and block data. |
| **Cache** | Redis 7 | Caching RPC responses and tracking rate-limit state. |

---

## Quick Start (Docker Compose)

### 1. Clone and Configure

```bash
git clone https://github.com/yourname/blockmesh.git
cd blockmesh
cp .env.example .env
```

Edit `.env` and set a strong `POSTGRES_PASSWORD` and an `ADMIN_SECRET` (required for the Admin API and Dashboard).

### 2. Start the Stack

```bash
docker compose up -d
```

This starts Postgres, Redis, the Go services, and the Nginx-served Web Dashboard.
*Note: The database migrations run automatically on startup via the Postgres entrypoint.*

### 3. Access the Dashboard

Visit `http://localhost` (or your configured domain).
- **Viewer Mode**: Browse chain data, recent blocks, and node health without credentials.
- **Admin Mode**: Click "Admin sign in" and enter your `ADMIN_SECRET` to manage tenants and networks.

---

## API Reference

### Gateway (JSON-RPC Proxy)

Proxies any valid ETH JSON-RPC method with auth, caching, and metering.

**Endpoint:** `POST /v1/`
**Headers:**
| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <api-key>` |
| `Content-Type` | `application/json` |

**Example:**
```bash
curl -X POST http://localhost/v1/ \
  -H "Authorization: Bearer <your-tenant-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_getBalance",
    "params": ["0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb", "latest"],
    "id": 1
  }'
```

### Admin API

Protected by the `X-Admin-Secret` header.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/stats/summary?range=1h` | GET | Aggregated stats (15m, 1h, 24h) |
| `/tenants` | GET/POST | List or create tenants |
| `/tenants/:id` | GET/PUT/DELETE | Manage specific tenant |
| `/tenants/:id/rotate-key`| POST | Revoke old key and generate a new one |
| `/usage?api_key=<key>` | GET | Daily usage report for a specific API key |
| `/blocks` | GET | Last 50 ingested blocks across all networks |
| `/blockchain` | GET/POST | List or add blockchain networks |
| `/blockchain/test` | POST | Test RPC endpoint connectivity |

---

## Development

For local development with hot-reloading (via Air for Go and Vite for React):

```bash
docker compose -f docker-compose.dev.yml up
```

- **Web Dashboard**: `http://localhost:3000` (Vite dev server)
- **Gateway API**: `http://localhost:8080`
- **Admin API**: `http://localhost:8081`

---

## Configuration

All services are configured via environment variables in your `.env` file:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTGRES_USER` | Yes | `blockmesh` | Database user |
| `POSTGRES_PASSWORD` | Yes | `blockmesh` | Database password |
| `POSTGRES_DB` | Yes | `blockmesh` | Database name |
| `ADMIN_SECRET` | **Yes** | — | Shared secret for Admin API and Dashboard login |
| `DATABASE_URL` | Auto | — | Constructed automatically by Docker Compose |
| `REDIS_ADDR` | Auto | — | Constructed automatically by Docker Compose |

*Note: Blockchain RPC endpoints are **not** configured via environment variables. They are managed dynamically via the Admin API or the Web Dashboard (`/blockchain`).*

---

## Backups & Maintenance

PostgreSQL data lives in the `pgdata` Docker volume.

**Backup:**
```bash
docker exec blockmesh-postgres-1 pg_dump -U blockmesh blockmesh > backup.sql
```

**Restore:**
```bash
cat backup.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh
```

**Upgrading:**
```bash
git pull
docker compose pull
docker compose up -d
```
*(Migrations run automatically on startup).*

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Gateway / Admin / Ingestor | Go 1.22 (`pgx`, `net/http`, `slog`) |
| Dashboard | React 18, TypeScript, Vite |
| Cache | Redis 7 |
| Database | PostgreSQL 15 |
| Ingress / Proxy | Nginx (Web container routes `/api` and `/gateway`) |
| Metrics | Prometheus (`prometheus/client_golang`) |

---

## Security

- **Never** commit `.env` to git.
- **Always** set a strong, random `ADMIN_SECRET`.
- API keys are hashed using SHA-256 before storage.
- Admin secret validation uses constant-time comparison to prevent timing attacks.
- The Web Dashboard uses session-only storage for credentials; closing the tab clears the session.

---

## License

MIT