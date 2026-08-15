# How to Deploy BlockMesh

BlockMesh can run on a single VPS via Docker Compose or on a Kubernetes cluster. This guide covers both paths.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Docker Compose (Single Server)](#docker-compose-single-server)
3. [Kubernetes / K3s](#kubernetes--k3s)
4. [Environment Variables](#environment-variables)
5. [Backups](#backups)
6. [Upgrades](#upgrades)
7. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Docker Compose

- Docker 24.0+ and Docker Compose v2
- 1 vCPU, 1 GB RAM, 10 GB disk (minimum)
- Outbound internet access to RPC endpoints

### Kubernetes

- K3s, k0s, or any Kubernetes 1.28+ cluster
- `kubectl` configured
- Traefik or another ingress controller with cert-manager (for TLS)
- 2 vCPU, 2 GB RAM, 20 GB disk (recommended)

---

## Docker Compose (Single Server)

### Step 1: Clone and configure

```bash
git clone https://github.com/yourname/blockmesh.git
cd blockmesh
cp .env.example .env
```

Edit `.env`:

```bash
# Required
POSTGRES_PASSWORD=your-strong-password-here

# Optional but recommended for production
DOMAIN=rpc.yourdomain.com
ACME_EMAIL=you@yourdomain.com
ADMIN_SECRET=a-random-string-for-admin-protection
```

### Step 2: Run the installer

```bash
make install
```

This:
1. Validates Docker is installed
2. Creates `.env` if missing
3. Pulls and builds images
4. Starts all services

### Step 3: Verify

```bash
docker compose ps
```

All services should show `Up`:
- `postgres` — Database
- `redis` — Cache
- `gateway` — RPC proxy (port 8080)
- `admin` — Admin API (port 8081)
- `ingestor` — Block indexer
- `web` — Dashboard (port 3000)
- `traefik` — Reverse proxy with auto-HTTPS (ports 80/443)

### Step 4: Access

| Service | URL |
|---------|-----|
| Dashboard | `http://localhost:3000` or `https://rpc.yourdomain.com` |
| Gateway | `http://localhost:8080/v1/` or `https://rpc.yourdomain.com/v1/` |
| Admin API | `http://localhost:8081` |
| Traefik Dashboard | `https://traefik.rpc.yourdomain.com` |

### Step 5: Create your first tenant

Open the dashboard, click **+ Create Tenant**, and copy the generated API key.

Or via API:
```bash
curl -X POST http://localhost:8081/tenants \
  -H "Content-Type: application/json" \
  -d '{"name":"My App","quota_rpm":1000}'
```

---

## Kubernetes / K3s

### Step 1: Build images

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

### Step 3: Create secrets

```bash
kubectl create namespace blockmesh

kubectl create secret generic blockmesh-secrets \
  --from-literal=postgres-user="blockmesh" \
  --from-literal=postgres-password="your-strong-password" \
  --from-literal=database-url="postgres://blockmesh:your-strong-password@postgres:5432/blockmesh" \
  --from-literal=admin-secret="your-admin-secret" \
  -n blockmesh
```

**Never commit `secrets.yaml` to git.** Use the example file as a template:
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

### Step 6: Configure ingress

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
| `RPC_ENDPOINT_1` | Yes | — | Primary blockchain RPC URL |
| `RPC_ENDPOINT_2` | No | — | Fallback blockchain RPC URL |
| `POSTGRES_USER` | Yes | `blockmesh` | Database username |
| `POSTGRES_PASSWORD` | Yes | `blockmesh` | Database password |
| `POSTGRES_DB` | Yes | `blockmesh` | Database name |
| `ADMIN_SECRET` | No | — | If set, required for `POST /tenants` |
| `DOMAIN` | No | `localhost` | Domain for Traefik TLS |
| `ACME_EMAIL` | No | — | Let's Encrypt contact email |

---

## Backups

### PostgreSQL

**Backup:**
```bash
docker exec blockmesh-postgres-1 pg_dump -U blockmesh blockmesh > backup-$(date +%F).sql
```

**Restore:**
```bash
cat backup-2026-08-15.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh
```

**Automated (cron):**
```bash
0 2 * * * docker exec blockmesh-postgres-1 pg_dump -U blockmesh blockmesh > /backups/blockmesh-$(date +\%F).sql
```

### Redis

Redis data is ephemeral (rate limit counters, cache). No backup needed unless you want to preserve cache warmness:
```bash
docker exec blockmesh-redis-1 redis-cli SAVE
docker cp blockmesh-redis-1:/data/dump.rdb /backups/redis-$(date +%F).rdb
```

---

## Upgrades

### Docker Compose

```bash
cd blockmesh
git pull
docker compose pull
docker compose up -d
```

Services restart with zero downtime for stateless containers (gateway, web). Postgres and Redis keep their data via volumes.

### Kubernetes

```bash
# Rebuild images
make build-all

# Re-import or re-push
for img in gateway admin ingestor web; do
  docker save blockmesh-$img:latest | sudo k3s ctr images import -
done

# Rolling restart
kubectl rollout restart deployment/gateway -n blockmesh
kubectl rollout restart deployment/admin -n blockmesh
kubectl rollout restart deployment/ingestor -n blockmesh
kubectl rollout restart deployment/web -n blockmesh
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
- **Port conflict** — Something else is using 8080, 8081, or 3000. Change ports in `docker-compose.yml`.

### Gateway returns `502 Bad Gateway`

1. Check upstream RPC endpoints are reachable:
   ```bash
   curl -X POST $RPC_ENDPOINT_1 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
   ```
2. Check gateway logs:
   ```bash
   docker compose logs gateway
   ```
3. Verify `RPC_ENDPOINT_1` and `RPC_ENDPOINT_2` in `.env`.

### Dashboard shows "database error"

1. Check admin logs:
   ```bash
   docker compose logs admin
   ```
2. Verify migrations ran:
   ```bash
   docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "\dt"
   ```
   You should see `tenants`, `usage`, and `blocks` tables.
3. If tables are missing, restart Postgres (migrations run on init):
   ```bash
   docker compose restart postgres
   ```

### Can't create tenants (403 Forbidden)

Your admin set `ADMIN_SECRET` in `.env` but you're not sending it. Include the header:
```bash
curl -X POST http://localhost:8081/tenants \
  -H "X-Admin-Secret: your-secret-here" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","quota_rpm":100}'
```

Or unset `ADMIN_SECRET` in `.env` and restart:
```bash
docker compose up -d admin
```

### Rate limits too aggressive

Edit the tenant's quota in the database:
```bash
docker exec -it blockmesh-postgres-1 psql -U blockmesh -d blockmesh
docker=# UPDATE tenants SET quota_rpm = 10000 WHERE api_key = 'bm_live_...';
```

Or delete the Redis counter to reset immediately:
```bash
docker exec blockmesh-redis-1 redis-cli KEYS "ratelimit:*" | xargs docker exec blockmesh-redis-1 redis-cli DEL
```

---

## Security Checklist

Before going to production:

- [ ] Change `POSTGRES_PASSWORD` from default
- [ ] Set `ADMIN_SECRET` to a random 32+ character string
- [ ] Replace `demo-key` with real tenant keys
- [ ] Enable Traefik HTTPS (`DOMAIN` and `ACME_EMAIL` set)
- [ ] Restrict `docker-compose.yml` port bindings (don't expose 8080/8081 publicly if using Traefik)
- [ ] Set up automated Postgres backups
- [ ] Review `deployments/base/secrets.yaml` is in `.gitignore`
