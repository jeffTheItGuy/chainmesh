# ChainMesh

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://react.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Tests](https://github.com/yourname/chainmesh/actions/workflows/test.yml/badge.svg)](https://github.com/yourname/chainmesh/actions)

**Self-hosted, multi-tenant blockchain API gateway.**

ChainMesh adds intelligent routing, Redis caching, per-tenant rate limiting, and usage metering to your blockchain RPC infrastructure. Run it on your own servers.

---

## What It Is

Raw RPC endpoints are unreliable, slow, unmetered, and lack tenant isolation. ChainMesh solves this by sitting between your applications and upstream nodes, providing automatic failover, aggressive caching, and granular quota enforcement.

---

## Quick Start

### Docker Compose (Single Server)

```bash
git clone https://github.com/yourname/chainmesh.git
cd chainmesh
cp .env.example .env
# Edit .env — set POSTGRES_PASSWORD and ADMIN_SECRET
docker compose up -d
```

This starts PostgreSQL, Redis, Gateway, Admin API, Ingestor, Web Dashboard, and Nginx.

### Kubernetes / K3s

```bash
kubectl create namespace chainmesh
kubectl create secret generic chainmesh-secrets \
  --from-literal=database-url='postgres://chainmesh:yourpassword@postgres:5432/chainmesh' \
  --from-literal=admin-secret='your-admin-secret' \
  -n chainmesh
kubectl apply -k deployments/base/
```

---

## Features

- **Health-aware smart routing** — Automatic failover between multiple upstream RPC endpoints with latency-based selection.
- **Multi-network support** — Configure Ethereum, Sepolia, Polygon, and other networks dynamically via the Admin API.
- **Secure multi-tenancy** — API keys are bcrypt hashed with a visible prefix. Supports key rotation and constant-time authentication.
- **Domain-aware caching** — Method-specific Redis TTLs (e.g., `eth_chainId` for 24h, `eth_blockNumber` for 2s).
- **Granular rate limiting** — Per-tenant RPS, RPM, and daily quotas enforced via Redis with atomic counters.
- **Async telemetry** — Non-blocking usage recording to PostgreSQL with pre-aggregated stats views.
- **Role-based dashboard** — Viewer mode for public chain data; Admin mode for full management.

---

## Testing

```bash
# Backend (Go)
cd backend
go test ./...
go test -race ./...

# Frontend (React + TypeScript)
cd frontend
npm run typecheck
npm run lint
npm run test
```

All contributions must pass the full test suite. See [docs/tests/](docs/tests/) for the complete testing strategy, including integration and post-production validation guides.

---

## API Reference

### Gateway

Proxies any valid ETH JSON-RPC method with authentication, caching, and metering.

**Endpoint:** `POST /v1/`

**Headers:**
| Header | Value |
|--------|-------|
| `Authorization` | `Bearer <api-key>` |
| `Content-Type` | `application/json` |

**Example:**
```bash
curl -X POST http://localhost:8080/v1/ \
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
| `/tenants/:id/rotate-key` | POST | Revoke old key and generate a new one |
| `/tenants/:id/usage` | GET | Daily usage report for a tenant |
| `/blockchain` | GET/POST | List or add blockchain networks |
| `/blockchain/test` | POST | Test RPC endpoint connectivity |
| `/blocks` | GET | Last 50 ingested blocks |
| `/audit-logs` | GET | Query admin audit logs |

---

## Configuration

All services are configured via environment variables. Blockchain RPC endpoints are **not** configured via environment variables; they are managed dynamically via the Admin API or Web Dashboard.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ADMIN_SECRET` | **Yes** | — | Shared secret for Admin API and Dashboard login |
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `REDIS_ADDR` | Yes | — | Redis host:port |
| `POSTGRES_USER` | Yes | `chainmesh` | Database user |
| `POSTGRES_PASSWORD` | Yes | `chainmesh` | Database password |
| `POSTGRES_DB` | Yes | `chainmesh` | Database name |
| `DOMAIN` | No | `localhost` | Domain for Traefik TLS |
| `ACME_EMAIL` | No | — | Let's Encrypt contact email |
| `TLS_CERT` | No | — | Path to TLS certificate file |
| `TLS_KEY` | No | — | Path to TLS private key file |

---

## Security

- Never commit `.env` or Kubernetes secrets to version control.
- Always set a strong, random `ADMIN_SECRET` (32+ characters recommended).
- API keys are bcrypt hashed before storage; only a 12-character prefix is retained for display.
- Admin secret validation uses constant-time comparison to prevent timing attacks.
- The Web Dashboard stores credentials in session-only storage; closing the tab clears the session.
- RPC endpoint URLs are redacted in public health responses.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Gateway / Admin / Ingestor | Go 1.22 |
| Dashboard | React 18, TypeScript, Vite |
| Cache | Redis 7 |
| Database | PostgreSQL 15 |
| Ingress / Proxy | Nginx (or Traefik for Docker Compose TLS) |
| Metrics | Prometheus |

---

## Documentation

Full documentation is available in the `docs/` directory:

- **Operators** — Deployment, configuration, backups, and upgrades
- **Administrators** — Managing tenants, networks, and monitoring
- **API Consumers** — Authentication, rate limits, and error reference
- **Developers** — Local setup, contributing guidelines, and architecture
- **Tests** — Unit, integration, and post-production testing strategy

---

## License

MIT
