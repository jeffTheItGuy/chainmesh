# Developer Setup Guide

This guide covers setting up the BlockMesh development environment locally. You'll run the Go backend services, React frontend, PostgreSQL, and Redis either natively or via Docker.

For production deployment instructions, see [deploy.md](../operators/deploy.md) (Docker Compose) or [deploy-k3s.md](../operators/deploy-k3s.md) (Kubernetes).

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start (Docker Compose)](#quick-start-docker-compose)
3. [Native Development](#native-development)
4. [Project Structure](#project-structure)
5. [Running Tests](#running-tests)
6. [Hot Reloading](#hot-reloading)
7. [Database Migrations](#database-migrations)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://golang.org/dl/) | 1.22+ | Backend services |
| [Node.js](https://nodejs.org/) | 20+ | Frontend build tooling |
| [PostgreSQL](https://postgresql.org/) | 15+ | Primary database |
| [Redis](https://redis.io/) | 7+ | Caching & rate limiting |
| [Docker](https://docker.com/) | 24.0+ | Containerized development (optional) |
| [Docker Compose](https://docs.docker.com/compose/) | v2+ | Multi-service orchestration |
| [Air](https://github.com/cosmtrek/air) | latest | Go hot reloading (optional) |

### Verify Installation

```bash
# Go
go version
# Expected: go version go1.22.x or higher

# Node.js
node --version
# Expected: v20.x.x or higher

# PostgreSQL
psql --version
# Expected: psql (PostgreSQL) 15.x

# Redis
redis-cli --version
# Expected: redis-cli 7.x.x

# Docker
docker --version
docker compose version
```

---

## Quick Start (Docker Compose)

The fastest way to get a complete development environment running.

### Step 1: Clone and Configure

```bash
git clone https://github.com/yourname/blockmesh.git
cd blockmesh
cp .env.example .env
```

Edit `.env` with development values:

```bash
# .env — Development Configuration
POSTGRES_USER=blockmesh
POSTGRES_PASSWORD=dev
POSTGRES_DB=blockmesh
ADMIN_SECRET=dev-secret-not-for-production
DATABASE_URL=postgres://blockmesh:dev@postgres:5432/blockmesh?sslmode=disable
REDIS_ADDR=redis:6379
DOMAIN=localhost
```

> **Note:** The `ADMIN_SECRET` is required even in development. The admin API refuses to start without it.

### Step 2: Start Infrastructure Services

```bash
# Start only Postgres and Redis
docker compose up -d postgres redis

# Wait for Postgres to be ready (≈10 seconds)
sleep 10

# Verify
docker compose ps
```

### Step 3: Run Backend Services (Native)

Open separate terminals for each service:

**Terminal 1 — Gateway:**
```bash
cd backend/gateway
go run main.go manager.go
# Service starts on :8080
```

**Terminal 2 — Admin API:**
```bash
cd backend/admin
go run main.go
# Service starts on :8081
```

**Terminal 3 — Ingestor:**
```bash
cd backend/ingestor
go run main.go
# Background block indexer starts
```

### Step 4: Run Frontend (Native)

```bash
cd frontend
npm install
npm run dev
# Vite dev server starts on http://localhost:5173
```

### Step 5: Verify

| Service | URL | Check |
|---------|-----|-------|
| Gateway | http://localhost:8080/health/nodes | Should return `[]` initially |
| Admin API | http://localhost:8081/health | Should return `{"status":"ok"}` |
| Dashboard | http://localhost:5173 | React app loads |

---

## Native Development

For debugging, breakpoint support, or when you prefer services running directly on your machine.

### 1. Database Setup

**Start PostgreSQL locally:**

```bash
# macOS (Homebrew)
brew install postgresql@15
brew services start postgresql@15

# Create database
createdb blockmesh

# Create user (if needed)
createuser -P blockmesh
# Enter password when prompted

# Verify
psql -U blockmesh -d blockmesh -c "\dt"
```

**Start Redis locally:**

```bash
# macOS (Homebrew)
brew install redis
brew services start redis

# Verify
redis-cli ping
# Expected: PONG
```

### 2. Environment Variables

Create a `.env` file in the project root:

```bash
# Native Development .env
export ADMIN_SECRET=dev-secret-not-for-production
export DATABASE_URL=postgres://blockmesh:dev@localhost:5432/blockmesh?sslmode=disable
export REDIS_ADDR=localhost:6379

# Optional: Postgres connection pool tuning
export POSTGRES_MAX_CONNS=20
export POSTGRES_MIN_CONNS=2
```

Load it:
```bash
source .env
```

### 3. Run Migrations

```bash
# Apply all migrations
psql -U blockmesh -d blockmesh -f backend/database/migrations/001_init.up.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/002_audit_logs.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/002_blockchain_config.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/003_multi_network.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/004_api_keys.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/005_request_logs.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/006_tenant_rate_limits.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/007_request_id.sql
psql -U blockmesh -d blockmesh -f backend/database/migrations/008_stats_rollup.sql

# Verify tables
psql -U blockmesh -d blockmesh -c "\dt"
```

### 4. Run Backend Services

**Gateway:**
```bash
cd backend/gateway
go run main.go manager.go
```

**Admin API:**
```bash
cd backend/admin
go run main.go
```

**Ingestor:**
```bash
cd backend/ingestor
go run main.go
```

### 5. Run Frontend

```bash
cd frontend
npm install
npm run dev -- --host
```

The `--host` flag exposes the dev server to your network (useful for testing on mobile or with colleagues).


---

## Hot Reloading

### Go (Air)

Install [Air](https://github.com/cosmtrek/air) for automatic Go rebuilds on file change:

```bash
go install github.com/cosmtrek/air@latest

# In each backend service directory
cd backend/gateway
air

# Or use the provided .air.toml files
cd backend/admin
air -c .air.toml
```

Each service directory contains a `.air.toml` configured for that service.

### Frontend (Vite)

Vite provides hot module replacement out of the box:

```bash
cd frontend
npm run dev
```

Changes to `.tsx` and `.css` files reflect instantly without page reload.

---

## Database Migrations

### Creating a New Migration

Follow the sequential numbering convention:

```bash
# Create new migration files
NEXT=$(ls backend/database/migrations/*.up.sql | sort -V | tail -1 | sed 's/.*\(0-9\+\)_.*/\1/' | awk '{printf "%03d", $1+1}')
touch backend/database/migrations/${NEXT}_feature_name.up.sql
touch backend/database/migrations/${NEXT}_feature_name.down.sql
```

**Naming convention:**
- `NNN_description.up.sql` — Forward migration
- `NNN_description.down.sql` — Rollback migration

### Applying Migrations

```bash
# During development — apply single migration
psql -U blockmesh -d blockmesh -f backend/database/migrations/009_new_feature.up.sql

# Rollback
psql -U blockmesh -d blockmesh -f backend/database/migrations/009_new_feature.down.sql
```

### Migration Guidelines

- **Always make migrations backward-compatible** (additive only)
- New columns must be nullable or have sensible defaults
- Add indexes concurrently in production: `CREATE INDEX CONCURRENTLY ...`
- Test rollbacks before committing

---

## Running Tests

### Go Tests

```bash
# All backend tests
cd backend
go test ./...

# Specific package
cd backend/shared/blockchain
go test -v ./...

# With race detection
go test -race ./...

# With coverage
go test -cover ./...
```

### Frontend Tests

```bash
cd frontend
npm run typecheck    # TypeScript type checking
npm run lint         # ESLint
npm run build        # Production build verification
```

---

## Common Development Tasks

### Adding a Blockchain Network

Via Admin API:

```bash
curl -X POST http://localhost:8081/blockchain \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ethereum Mainnet",
    "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
    "rpc_endpoint_2": "https://cloudflare-eth.com",
    "chain_id": "1",
    "enabled": true
  }'
```

Or use the Dashboard at http://localhost:5173 → Infrastructure → Blockchain Networks.

### Creating a Test Tenant

```bash
curl -X POST http://localhost:8081/tenants \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Dev Test",
    "quota_rpm": 1000,
    "quota_rps": 10,
    "quota_daily": 100000,
    "plan": "free"
  }'
```

**Save the returned `api_key`** — it's shown only once.

### Testing a Proxied RPC Call

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_chainId",
    "params": [],
    "id": 1
  }'
```

---

## Troubleshooting

### "ADMIN_SECRET is not set"

The admin API requires `ADMIN_SECRET`. Set it:

```bash
export ADMIN_SECRET=dev-secret-not-for-production
```

### "postgres failed" / connection refused

1. Verify Postgres is running: `docker compose ps` or `brew services list`
2. Check `DATABASE_URL` matches your setup
3. Ensure database exists: `createdb blockmesh`
4. Verify user credentials: `psql -U blockmesh -d blockmesh -c "SELECT 1"`

### "redis failed" / connection refused

1. Verify Redis is running: `redis-cli ping`
2. Check `REDIS_ADDR` — use `localhost:6379` for native, `redis:6379` for Docker

### Gateway returns 503 "no blockchain network configured"

You must add at least one network before making RPC calls. See [Adding a Blockchain Network](#adding-a-blockchain-network).

### Frontend proxy errors (502 Bad Gateway)

The Vite dev server proxies `/api` to `http://admin:8081` and `/gateway` to `http://gateway:8080`. When running natively, update `vite.config.ts`:

```typescript
// vite.config.ts — Native development
export default defineConfig({
  // ...
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8081',  // Changed from admin:8081
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/gateway': {
        target: 'http://localhost:8080',  // Changed from gateway:8080
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/gateway/, ''),
      },
    },
  },
})
```

### CORS errors in browser

The Go services don't set CORS headers by default. For frontend development, the Vite proxy handles this. If accessing directly, add CORS middleware or use a browser extension for development.

### Port conflicts

| Service | Default Port | Change via |
|---------|-------------|------------|
| Gateway | 8080 | `Addr` in `main.go` |
| Admin | 8081 | `Addr` in `main.go` |
| Frontend (dev) | 5173 | `server.port` in `vite.config.ts` |
| Postgres | 5432 | `DATABASE_URL` or Docker mapping |
| Redis | 6379 | `REDIS_ADDR` or Docker mapping |

---

## IDE Configuration

### VS Code Extensions (Recommended)

- **Go** — Official Go extension by Google
- **ESLint** — JavaScript/TypeScript linting
- **Prettier** — Code formatting
- **Tailwind CSS IntelliSense** — CSS autocomplete

### Go Workspace

The project uses Go modules. Ensure your `GOPATH` is set and run:

```bash
cd backend
go mod tidy  # Download dependencies
go mod verify
```

### Debug Configuration (VS Code)

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Gateway",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/backend/gateway",
      "envFile": "${workspaceFolder}/.env"
    },
    {
      "name": "Admin API",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/backend/admin",
      "envFile": "${workspaceFolder}/.env"
    }
  ]
}
```

---

## Next Steps

- **[Architecture Overview](../architecture/overview.md)** — Understand system components
- **[Data Flow](../architecture/data-flow.md)** — Trace a request through the stack
- **[API Reference](../../backend/api/openapi.yaml)** — OpenAPI specification
- **[Adding Networks](../admins/add-networks.md)** — Configure blockchain endpoints
- **[Adding Tenants](../admins/add-tenants.md)** — Create API consumers

---

*Last updated: 2026-08-19*
