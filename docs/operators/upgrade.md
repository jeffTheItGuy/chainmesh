# Upgrading BlockMesh

This guide covers upgrading BlockMesh with minimal or zero downtime.

---

## Table of Contents

1. [Upgrade Strategy](#upgrade-strategy)
2. [Docker Compose Upgrades](#docker-compose-upgrades)
3. [Kubernetes Upgrades](#kubernetes-upgrades)
4. [Database Migrations](#database-migrations)
5. [Rollback Procedures](#rollback-procedures)
6. [Breaking Changes](#breaking-changes)

---

## Upgrade Strategy

BlockMesh uses **rolling upgrades** — stateless services restart individually while Postgres and Redis maintain state.

**Upgrade order:**
1. Review [breaking changes](#breaking-changes) for the target version
2. Back up the database ([see backup guide](backup.md))
3. Upgrade stateless services (gateway, admin, ingestor, web)
4. Verify health checks pass
5. Monitor for errors

**Zero-downtime assumptions:**
- You have at least 2 gateway replicas (Kubernetes) or accept brief unavailability (Docker Compose)
- Postgres and Redis are not upgraded in-place (separate maintenance window)
- Database migrations are backward-compatible (additive only)

---

## Docker Compose Upgrades

### Standard Upgrade

```bash
cd blockmesh
git pull                    # Get latest code
docker compose pull         # Pull new images
docker compose up -d        # Recreate containers with new images
```

**What happens:**
- Postgres container: restarted (data preserved in volume)
- Redis container: restarted (data lost unless persisted)
- Gateway/Admin/Ingestor/Web: recreated with new images
- Traefik: recreated if image updated

**Downtime:** Brief (seconds) while containers restart. Gateway is unavailable during restart.

### Zero-Downtime Upgrade (Manual Blue/Green)

For production Docker Compose deployments requiring zero downtime:

```bash
# 1. Back up
docker exec blockmesh-postgres-1 pg_dump -U blockmesh blockmesh > pre-upgrade.sql

# 2. Start new stack on different ports
cp docker-compose.yml docker-compose.green.yml
# Edit: change ports 8080→9080, 8081→9081, 80→90
# Edit: use same postgres and redis volumes

docker compose -f docker-compose.green.yml up -d

# 3. Verify green stack
curl http://localhost:9080/health/nodes
curl http://localhost:9081/health

# 4. Switch traffic (update reverse proxy or DNS)
# 5. Stop blue stack
docker compose down

# 6. On next upgrade, blue becomes green and vice versa
```

### Image-Only Upgrade

If only application code changed (no dependency updates):

```bash
docker compose build gateway admin ingestor web
docker compose up -d
```

---

## Kubernetes Upgrades

### Rolling Restart

```bash
# 1. Rebuild images
make build-all

# 2. Import or push
for img in gateway admin ingestor web; do
  docker save blockmesh-$img:latest | sudo k3s ctr images import -
done

# 3. Rolling restart (zero downtime with 2+ replicas)
kubectl rollout restart deployment/gateway -n blockmesh
kubectl rollout restart deployment/admin -n blockmesh
kubectl rollout restart deployment/ingestor -n blockmesh
kubectl rollout restart deployment/web -n blockmesh

# 4. Monitor rollout
kubectl rollout status deployment/gateway -n blockmesh
kubectl rollout status deployment/admin -n blockmesh
```

### Canary Deployment

For high-risk upgrades, deploy a canary gateway:

```yaml
# canary-gateway.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway-canary
  namespace: blockmesh
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gateway
      track: canary
  template:
    metadata:
      labels:
        app: gateway
        track: canary
    spec:
      containers:
      - name: gateway
        image: ghcr.io/yourname/blockmesh-gateway:v1.3.0
        # ... same env as production gateway
---
apiVersion: v1
kind: Service
metadata:
  name: gateway-canary
  namespace: blockmesh
spec:
  selector:
    app: gateway
    track: canary
  ports:
  - port: 8080
```

Route 10% of traffic to canary via ingress or service mesh, monitor error rates, then promote or rollback.

---

## Database Migrations

### How Migrations Work

Migrations are SQL files in `backend/database/migrations/`. They run automatically on service startup via the Postgres entrypoint or application initialization.

**Migration naming:**
```
001_init.up.sql          # Create tables
001_init.down.sql        # Drop tables
002_blockchain_config.sql
003_multi_network.sql
...
```

### Pre-Upgrade Check

```bash
# Check current migration state
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "SELECT version, applied_at FROM schema_migrations ORDER BY version;"
```

> **Note:** If you don't use a migration tool (e.g., golang-migrate), migrations run via application init or Docker entrypoint. Verify your setup.

### Migration Safety

**Safe (no data loss):**
- Adding columns with defaults
- Adding indexes (concurrently if possible)
- Creating new tables
- Adding nullable foreign keys

**Unsafe (requires planning):**
- Dropping columns
- Renaming columns
- Changing column types
- Adding non-nullable columns without defaults

**BlockMesh migration policy:** All migrations are additive and backward-compatible. No destructive migrations are shipped in patch/minor versions.

### Manual Migration (if automatic fails)

```bash
# 1. Stop services that write
docker compose stop gateway admin ingestor

# 2. Run migration manually
docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh < backend/database/migrations/009_new_feature.sql

# 3. Restart services
docker compose start gateway admin ingestor
```

---

## Rollback Procedures

### Docker Compose Rollback

```bash
# 1. Stop current stack
docker compose down

# 2. Restore database from pre-upgrade backup
cat pre-upgrade.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh

# 3. Checkout previous version
git checkout v1.2.0  # or previous commit

# 4. Start previous version
docker compose up -d
```

### Kubernetes Rollback

```bash
# Rollback to previous revision
kubectl rollout undo deployment/gateway -n blockmesh
kubectl rollout undo deployment/admin -n blockmesh

# Or to specific revision
kubectl rollout history deployment/gateway -n blockmesh
kubectl rollout undo deployment/gateway -n blockmesh --to-revision=3
```

### Database Rollback

**If migration was destructive, you must restore from backup.** There is no automatic down-migration in production.

```bash
# Stop all services
docker compose down

# Restore database
docker volume rm blockmesh_pgdata
docker compose up -d postgres
sleep 10
cat pre-upgrade.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh

# Start services
docker compose up -d
```

---

## Breaking Changes

### v1.2.0 (Current)

- **API endpoint change:** `/usage?api_key=` removed. Use `/tenants/{id}/usage` instead.
- **Environment variables:** `RPC_ENDPOINT_1` and `RPC_ENDPOINT_2` no longer read from env. Configure networks via Admin API.
- **Database:** Migration `008_stats_rollup.sql` adds materialized view. First refresh may take time on large `request_logs` tables.

### v1.1.0

- **Auth model change:** API keys moved from `tenants.api_key` to `api_keys` table with SHA-256 hashing.
- **Action:** Existing plaintext keys in `tenants.api_key` still work but are deprecated. Rotate keys to use new system.

### v1.0.0

- Initial release.

---

## Post-Upgrade Verification

After any upgrade, run this checklist:

```bash
# 1. All services healthy
docker compose ps
# or: kubectl get pods -n blockmesh

# 2. Admin API responds
curl http://localhost:8081/health
# Expected: {"status":"ok"}

# 3. Gateway health check
curl http://localhost:8080/health/nodes
# Expected: Array with healthy nodes

# 4. Test a proxied request
curl -X POST http://localhost:8080/v1/   -H "Authorization: Bearer $API_KEY"   -H "Content-Type: application/json"   -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
# Expected: {"jsonrpc":"2.0","id":1,"result":"0x..."}

# 5. Dashboard loads
 curl http://localhost
# Expected: HTML with no console errors

# 6. Check logs for errors
docker compose logs --tail=100 gateway | grep -i error
docker compose logs --tail=100 admin | grep -i error
```

---

## Related Documents

- [Deploy](deploy.md) — Deployment procedures
- [Backups](backup.md) — Backup and restore procedures
- [Configure](configure.md) — Environment variable reference
