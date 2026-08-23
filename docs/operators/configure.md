# Configuration Reference

BlockMesh is configured entirely through environment variables. This document describes every variable, its behavior, and recommended values for different environments.

---

## Configuration Philosophy

BlockMesh follows **12-Factor App** principles: all configuration is via environment variables, no config files. This makes the system predictable across Docker, Kubernetes, and bare-metal deployments.

Key decisions:
- **No config files** — Environment variables only
- **No RPC endpoints in env vars** — Networks are configured dynamically via API (prevents restart cycles)
- **Sensible defaults** — Works out of the box for development; production requires explicit tuning
- **Fail closed** — Missing required variables cause startup failure with clear messages

---

## Required Variables

### `ADMIN_SECRET`

**Required by:** Admin API, Web Dashboard admin mode

The shared secret for admin authentication. Used for:
- `X-Admin-Secret` header validation on all admin endpoints
- Dashboard admin login

**Constraints:**
- Minimum 32 characters recommended
- Compared using constant-time comparison (`subtle.ConstantTimeCompare`)
- If unset, the admin API **exits immediately** on startup

**Generation:**
```bash
openssl rand -hex 32
```

**Example:**
```bash
ADMIN_SECRET=7f3a9c2e1d8b4f5067a9c2e1d8b4f5067a9c2e1d8b4f5067a9c2e1d8b4f506
```

**Security:**
- Store in a secret manager (Kubernetes secrets, Docker secrets, Vault)
- Never commit to git
- Rotate if any admin leaves or if suspected compromise

---

### `DATABASE_URL`

**Required by:** Gateway, Admin API, Ingestor

PostgreSQL connection string in standard format.

**Format:**
```
postgres://<user>:<password>@<host>:<port>/<database>?<params>
```

**Example:**
```bash
DATABASE_URL=postgres://blockmesh:strongpassword@postgres:5432/blockmesh?sslmode=disable
```

**Pool tuning (via separate env vars):**

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_MAX_CONNS` | `20` | Maximum open connections |
| `POSTGRES_MIN_CONNS` | `2` | Minimum idle connections |
| `POSTGRES_MAX_CONN_LIFETIME` | `30m` | Max lifetime of a connection |
| `POSTGRES_MAX_CONN_IDLE_TIME` | `10m` | Max idle time before close |
| `POSTGRES_HEALTH_CHECK_PERIOD` | `5m` | Background health check interval |

**Tuning guidance:**
- **Development:** Defaults are fine
- **Production (light):** Max 20, Min 5
- **Production (heavy):** Max 50, Min 10, Lifetime 1h
- **Connection limit math:** `max_conns × (gateway_replicas + admin_replicas + ingestor_count)` must be under Postgres `max_connections` (default 100)

---

### `REDIS_ADDR`

**Required by:** Gateway

Redis host and port for caching and rate limiting.

**Example:**
```bash
REDIS_ADDR=redis:6379
```

**In Docker Compose:**
```yaml
services:
  redis:
    image: redis:7-alpine
  gateway:
    environment:
      REDIS_ADDR: redis:6379
```

**In Kubernetes:**
```yaml
env:
  - name: REDIS_ADDR
    value: "redis.blockmesh.svc.cluster.local:6379"
```

**Redis configuration:**
- BlockMesh uses Redis as a cache, not a persistent store
- No persistence required (RDB/AOF optional)
- Maxmemory policy should be `allkeys-lru` or `allkeys-lfu` (cache eviction)
- Rate limit keys have short TTLs (2s to 25h); stale keys self-expire

---

## Database Credentials (Docker Compose)

These are used by the Postgres container and to construct `DATABASE_URL`:

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_USER` | `blockmesh` | Database owner |
| `POSTGRES_PASSWORD` | `blockmesh` | **Must be changed in production** |
| `POSTGRES_DB` | `blockmesh` | Database name |

**In `.env`:**
```bash
POSTGRES_USER=blockmesh
POSTGRES_PASSWORD=your-very-strong-password-here
POSTGRES_DB=blockmesh
```

The `docker-compose.yml` typically constructs `DATABASE_URL` from these:
```yaml
environment:
  DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}
```

---

## Optional Variables

### Telemetry Tuning

| Variable | Default | Description |
|----------|---------|-------------|
| `TELEMETRY_SHUTDOWN_TIMEOUT` | `5s` | Max time to wait for telemetry worker to finish on shutdown |
| `TELEMETRY_DRAIN_TIMEOUT` | `2s` | Max time to drain remaining queue after cancellation |

**When to tune:**
- Increase if you see "telemetry worker shutdown timed out" warnings during deploys
- Decrease if you need faster shutdown (may drop more records)

### TLS

| Variable | Default | Description |
|----------|---------|-------------|
| `TLS_CERT` | — | Path to TLS certificate file |
| `TLS_KEY` | — | Path to TLS private key file |

**When to use:**
- Set both to enable HTTPS directly on the Gateway and Admin API services
- If using Traefik or another reverse proxy for TLS termination, leave these unset

### Traefik / TLS (Docker Compose)

| Variable | Default | Description |
|----------|---------|-------------|
| `DOMAIN` | `localhost` | Domain for Traefik routing and TLS |
| `ACME_EMAIL` | — | Let's Encrypt contact email for certificate notifications |

**Example for production:**
```bash
DOMAIN=rpc.yourdomain.com
ACME_EMAIL=ops@yourdomain.com
```

**Without TLS (development only):**
```bash
DOMAIN=localhost
# ACME_EMAIL unset — Traefik will not attempt HTTPS
```

---

## What NOT to Configure via Environment Variables

### Blockchain RPC Endpoints

**Do not** set `RPC_ENDPOINT_1` or `RPC_ENDPOINT_2` in environment variables. These are managed dynamically via the Admin API:

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

**Why:** Dynamic configuration allows:
- Adding networks without restart
- Zero-downtime endpoint changes
- Per-tenant network assignment
- Multiple concurrent networks

### Rate Limits

**Do not** set rate limits globally. These are per-tenant and managed via the Admin API:

```bash
curl -X POST http://localhost:8081/tenants \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "quota_rps": 10,
    "quota_rpm": 1000,
    "quota_daily": 100000
  }'
```

---

## Environment-Specific Recommendations

### Development (Local)

```bash
# .env
POSTGRES_PASSWORD=dev
ADMIN_SECRET=dev-secret-not-for-production
DATABASE_URL=postgres://blockmesh:dev@postgres:5432/blockmesh?sslmode=disable
REDIS_ADDR=redis:6379
DOMAIN=localhost
```

### Staging

```bash
# .env
POSTGRES_PASSWORD=<strong-random>
ADMIN_SECRET=<strong-random-32-char>
DATABASE_URL=postgres://blockmesh:<pass>@postgres:5432/blockmesh?sslmode=require
REDIS_ADDR=redis:6379
POSTGRES_MAX_CONNS=20
POSTGRES_MIN_CONNS=5
DOMAIN=staging-rpc.yourdomain.com
ACME_EMAIL=ops@yourdomain.com
```

### Production

```bash
# .env
POSTGRES_PASSWORD=<strong-random-64-char>
ADMIN_SECRET=<strong-random-64-char>
DATABASE_URL=postgres://blockmesh:<pass>@postgres:5432/blockmesh?sslmode=require
REDIS_ADDR=redis:6379
POSTGRES_MAX_CONNS=50
POSTGRES_MIN_CONNS=10
POSTGRES_MAX_CONN_LIFETIME=1h
TELEMETRY_SHUTDOWN_TIMEOUT=10s
DOMAIN=rpc.yourdomain.com
ACME_EMAIL=ops@yourdomain.com
```

**Additional production hardening:**
- Use a dedicated Postgres user (not superuser)
- Enable SSL/TLS for Postgres connections (`sslmode=require` or `verify-full`)
- Run Redis with AUTH enabled and bind to internal network only
- Set `POSTGRES_PASSWORD` and `ADMIN_SECRET` via secret manager, not `.env` file
- Restrict `docker-compose.yml` port bindings — don't expose 8080/8081 publicly if using Traefik

---

## Configuration Validation

BlockMesh validates configuration at startup:

| Service | Validates | On Failure |
|---------|-----------|------------|
| Gateway | `DATABASE_URL`, `REDIS_ADDR` | Logs error, exits |
| Admin | `ADMIN_SECRET`, `DATABASE_URL` | Logs error, exits |
| Ingestor | `DATABASE_URL` | Logs error, exits |

**Check startup logs:**
```bash
docker compose logs gateway | grep -E "(starting|failed|error)"
docker compose logs admin | grep -E "(starting|failed|error)"
```

---

## Related Documents

- [Deploy](deploy.md) — Full deployment procedures
- [Security](security.md) — Secret management and hardening
- [Architecture Overview](../architecture/overview.md) — Component interactions
