# Deploying BlockMesh

This guide covers deploying BlockMesh on a single server via Docker Compose or on a Kubernetes cluster.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Docker Compose (Single Server)](#docker-compose-single-server)
3. [Kubernetes / K3s](#kubernetes--k3s)
4. [Environment Variables](#environment-variables)
5. [First-Time Setup](#first-time-setup)
6. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Docker Compose

- Docker 24.0+ and Docker Compose v2
- 1 vCPU, 1 GB RAM, 10 GB disk (minimum)
- 2 vCPU, 2 GB RAM, 20 GB SSD (recommended)
- Outbound internet access to RPC endpoints

### Kubernetes

- K3s, k0s, or any Kubernetes 1.28+ cluster
- `kubectl` configured
- Traefik or another ingress controller with cert-manager (for TLS)
- 2 vCPU, 2 GB RAM, 20 GB disk (recommended)

---

## Docker Compose (Single Server)

### Step 1: Clone and Configure

```bash
git clone https://github.com/yourname/blockmesh.git
cd blockmesh
cp .env.example .env
```

Edit `.env`:

```bash
# Required — set strong, unique values
POSTGRES_PASSWORD=your-strong-password-here
ADMIN_SECRET=a-random-32-char-string-for-admin-protection

# Optional but recommended for production
DOMAIN=rpc.yourdomain.com
ACME_EMAIL=you@yourdomain.com
```

> **Important:** `ADMIN_SECRET` is **required**. The admin API refuses to start without it. Generate a strong secret:
> ```bash
> openssl rand -hex 32
> ```

### Step 2: Start the Stack

```bash
docker compose up -d
```

This starts:
- `postgres` — Database (port 5432 internally)
- `redis` — Cache (port 6379 internally)
- `gateway` — RPC proxy (port 8080)
- `admin` — Admin API (port 8081)
- `ingestor` — Block indexer
- `web` — Dashboard served by Nginx (port 80/443 via Traefik, or 3000 direct)
- `traefik` — Reverse proxy with auto-HTTPS (ports 80/443)

> **Note:** Blockchain RPC endpoints are **not** configured via environment variables. They are added dynamically through the Admin API or Web Dashboard after deployment. See [Adding Blockchain Networks](../HowToAddNetworks.md).

### Step 3: Verify

```bash
docker compose ps
```

All services should show `Up`:

| Service | Status Check |
|---------|-------------|
| postgres | `docker compose logs postgres` should show "database system is ready" |
| gateway | `curl http://localhost:8080/health/nodes` should return `[]` (no networks yet) |
| admin | `curl http://localhost:8081/health` should return `{"status":"ok"}` |
| web | `curl http://localhost` should return the dashboard HTML |

### Step 4: Access

| Service | URL |
|---------|-----|
| Dashboard | `http://localhost` or `https://rpc.yourdomain.com` |
| Gateway | `http://localhost:8080/v1/` |
| Admin API | `http://localhost:8081` |
| Traefik Dashboard | `https://traefik.rpc.yourdomain.com` (if configured) |

### Step 5: Create Your First Network and Tenant

1. Open the dashboard at `http://localhost`
2. Sign in as admin with your `ADMIN_SECRET`
3. Go to **Infrastructure → Blockchain Networks** and add a network
4. Go to **Access → Tenants** and create a tenant
5. Copy the generated API key

Or via API:
```bash
export ADMIN_SECRET=your-secret-here

# Add a network
curl -X POST http://localhost:8081/blockchain   -H "X-Admin-Secret: $ADMIN_SECRET"   -H "Content-Type: application/json"   -d '{
    "name": "Ethereum Mainnet",
    "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
    "rpc_endpoint_2": "https://cloudflare-eth.com",
    "chain_id": "1",
    "enabled": true
  }'

# Create a tenant
curl -X POST http://localhost:8081/tenants   -H "X-Admin-Secret: $ADMIN_SECRET"   -H "Content-Type: application/json"   -d '{"name":"My App","quota_rpm":1000}'
```

---

## Kubernetes / K3s

### Step 1: Build Images

```bash
make build-all
```

### Step 2: Import into K3s (or push to a registry)

```bash
for img in gateway admin ingestor web; do
  docker save blockmesh-$img:latest | sudo k3s ctr images import -
done
```

Or push to GHCR / Docker Hub:
```bash
docker tag blockmesh-gateway:latest ghcr.io/yourname/blockmesh-gateway:latest
docker push ghcr.io/yourname/blockmesh-gateway:latest
# Repeat for admin, ingestor, web
```

### Step 3: Create Namespace and Secrets

```bash
kubectl create namespace blockmesh

kubectl create secret generic blockmesh-secrets   --from-literal=postgres-user="blockmesh"   --from-literal=postgres-password="your-strong-password"   --from-literal=database-url="postgres://blockmesh:your-strong-password@postgres:5432/blockmesh"   --from-literal=admin-secret="your-admin-secret"   --from-literal=redis-addr="redis:6379"   -n blockmesh
```

**Never commit secrets to git.** Use the example file as a template:
```bash
cp deployments/base/secrets-example.yaml deployments/base/secrets.yaml
# Edit secrets.yaml with real values, then:
kubectl apply -f deployments/base/secrets.yaml
```

### Step 4: Deploy

```bash
kubectl apply -k deployments/base/
```

### Step 5: Verify

```bash
kubectl get pods -n blockmesh
kubectl logs -n blockmesh deployment/gateway
kubectl logs -n blockmesh deployment/ingestor
```

### Step 6: Configure Ingress

Edit `deployments/base/ingress.yaml` and replace `yourdomain.com` with your actual domain:

```yaml
spec:
  tls:
    - hosts:
        - api.yourdomain.com
        - admin.yourdomain.com
        - dashboard.yourdomain.com
```

Apply:
```bash
kubectl apply -f deployments/base/ingress.yaml
```

Ensure your DNS points `api.yourdomain.com`, `admin.yourdomain.com`, and `dashboard.yourdomain.com` to your cluster's ingress IP.

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | Postgres connection string |
| `REDIS_ADDR` | Yes | — | Redis host:port |
| `ADMIN_SECRET` | **Yes** | — | Shared secret for Admin API and Dashboard login |
| `POSTGRES_USER` | Yes | `blockmesh` | Database username |
| `POSTGRES_PASSWORD` | Yes | `blockmesh` | Database password |
| `POSTGRES_DB` | Yes | `blockmesh` | Database name |
| `POSTGRES_MAX_CONNS` | No | `20` | Postgres max connections |
| `POSTGRES_MIN_CONNS` | No | `2` | Postgres min connections |
| `POSTGRES_MAX_CONN_LIFETIME` | No | `30m` | Max connection lifetime |
| `POSTGRES_MAX_CONN_IDLE_TIME` | No | `10m` | Max idle time before close |
| `POSTGRES_HEALTH_CHECK_PERIOD` | No | `5m` | Health check interval |
| `TELEMETRY_SHUTDOWN_TIMEOUT` | No | `5s` | Graceful telemetry worker shutdown |
| `TELEMETRY_DRAIN_TIMEOUT` | No | `2s` | Queue drain timeout on shutdown |
| `DOMAIN` | No | `localhost` | Domain for Traefik TLS |
| `ACME_EMAIL` | No | — | Let's Encrypt contact email |

> **Note:** `RPC_ENDPOINT_1` and `RPC_ENDPOINT_2` are **not** environment variables. Networks are configured dynamically via the Admin API. This prevents restart cycles when adding or changing endpoints.

---

## First-Time Setup

After deployment, the system is not yet usable until you:

1. **Add at least one blockchain network** — The gateway will return `503` for all requests until a network exists.
2. **Create at least one tenant** — All gateway requests require authentication.
3. **Verify health checks** — Wait 10–20 seconds for the first health check cycle to complete.

### Verification Checklist

```bash
# 1. Admin API is healthy
curl http://localhost:8081/health
# Expected: {"status":"ok"}

# 2. At least one network is configured
curl http://localhost:8081/blockchain   -H "X-Admin-Secret: $ADMIN_SECRET"
# Expected: Array with at least one enabled network

# 3. Gateway can see the network
curl http://localhost:8080/health/nodes
# Expected: Array with network health status

# 4. Tenant API key works
curl -X POST http://localhost:8080/v1/   -H "Authorization: Bearer $API_KEY"   -H "Content-Type: application/json"   -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
# Expected: {"jsonrpc":"2.0","id":1,"result":"0x1"}
```

---

## Troubleshooting

### Services won't start

```bash
docker compose logs
```

Common causes:
- **Postgres not ready** — Gateway/Admin crash-loop until Postgres passes health check. Wait 10–20 seconds.
- **Bad `DATABASE_URL`** — Must match `POSTGRES_USER`/`POSTGRES_PASSWORD` in `.env`.
- **Port conflict** — Something else is using 8080, 8081, or 80. Change ports in `docker-compose.yml`.
- **Missing `ADMIN_SECRET`** — Admin API exits immediately. Check `.env` and restart.

### Gateway returns `502 Bad Gateway`

1. Check upstream RPC endpoints are reachable:
   ```bash
   curl -X POST https://ethereum-rpc.publicnode.com      -H "Content-Type: application/json"      -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
   ```
2. Check gateway logs:
   ```bash
   docker compose logs gateway
   ```
3. Verify the network is enabled in the admin API.

### Dashboard shows "database error"

1. Check admin logs:
   ```bash
   docker compose logs admin
   ```
2. Verify migrations ran:
   ```bash
   docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "\dt"
   ```
   You should see `tenants`, `usage`, `blocks`, `blockchain_configs`, `api_keys`, and `request_logs` tables.
3. If tables are missing, restart the admin service to trigger migration checks:
   ```bash
   docker compose restart admin
   ```

### Can't create tenants (403 Forbidden)

Your admin set `ADMIN_SECRET` in `.env` but you're not sending it. Include the header:
```bash
curl -X POST http://localhost:8081/tenants   -H "X-Admin-Secret: your-secret-here"   -H "Content-Type: application/json"   -d '{"name":"Test","quota_rpm":100}'
```

### Gateway returns `503 Service Unavailable`

- No blockchain networks configured — add one via the dashboard or API
- Network has no healthy nodes — check `/health/nodes` and verify endpoints
- Tenant has no network assigned and no default exists — enable at least one network

---

## Related Documents

- [Configuration](configure.md) — Detailed configuration reference
- [Backups](backup.md) — Database backup and restore procedures
- [Upgrades](upgrade.md) — Zero-downtime upgrade procedures
- [Security](security.md) — Security hardening and threat model
- [Adding Blockchain Networks](../HowToAddNetworks.md) — Network configuration
- [Adding Tenants](../HowToAddTenants.md) — Tenant management
