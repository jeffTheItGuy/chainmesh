# Backups and Maintenance

This guide covers backing up and restoring BlockMesh data, along with routine maintenance tasks.

---

## Table of Contents

1. [What to Back Up](#what-to-back-up)
2. [PostgreSQL Backups](#postgresql-backups)
3. [Redis Backups](#redis-backups)
4. [Automated Backups](#automated-backups)
5. [Disaster Recovery](#disaster-recovery)
6. [Routine Maintenance](#routine-maintenance)

---

## What to Back Up

| Component | Priority | Method | Notes |
|-----------|----------|--------|-------|
| **PostgreSQL** | Critical | `pg_dump` | All persistent state: tenants, keys, logs, blocks, configs |
| **Redis** | Optional | `redis-cli SAVE` | Ephemeral: rate limits, cache. Optional unless you want warm cache |
| **Environment files** | Critical | Version control or secret manager | `.env`, Kubernetes secrets |
| **Custom configs** | Medium | Git or config management | `docker-compose.yml` overrides, K8s manifests |

**What you do NOT need to back up:**
- Container images (rebuild or pull from registry)
- Compiled frontend assets (rebuild from source)
- Prometheus metrics (ephemeral time-series data)

---

## PostgreSQL Backups

### Manual Backup

```bash
# Docker Compose
docker exec blockmesh-postgres-1 pg_dump   -U blockmesh   -d blockmesh   --no-owner   --no-privileges   > backup-$(date +%F-%H%M).sql
```

**Flags explained:**
- `--no-owner` — Skip ownership commands (useful for restore to different user)
- `--no-privileges` — Skip privilege grants

**Compressed backup:**
```bash
docker exec blockmesh-postgres-1 pg_dump -U blockmesh -d blockmesh |   gzip > backup-$(date +%F-%H%M).sql.gz
```

### Manual Restore

**Warning:** Restore overwrites existing data. Test on a separate instance first.

```bash
# 1. Stop services that write to the database
docker compose stop gateway admin ingestor

# 2. Drop and recreate the database
docker exec -i blockmesh-postgres-1 psql -U blockmesh -d postgres <<EOF
DROP DATABASE IF EXISTS blockmesh;
CREATE DATABASE blockmesh;
EOF

# 3. Restore from backup
cat backup-2026-08-19.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh

# 4. Restart services
docker compose start gateway admin ingestor
```

**Compressed restore:**
```bash
gunzip -c backup-2026-08-19.sql.gz |   docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh
```

### Point-in-Time Recovery (PITR)

For production deployments, enable WAL archiving for point-in-time recovery:

```bash
# In postgres container or custom image
# postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'cp %p /archive/%f'
max_wal_size = 1GB
```

This is beyond the scope of basic Docker Compose. Consider managed Postgres (AWS RDS, GCP Cloud SQL, DigitalOcean Managed Postgres) for production PITR.

---

## Redis Backups

Redis data is ephemeral by design — rate limit counters and cache entries. Backups are optional.

### When to Back Up Redis

- You want to preserve a warm cache after restart
- You have custom Redis persistence configured (RDB/AOF)
- You're migrating to a new Redis instance

### Manual Backup

```bash
# Trigger RDB save
docker exec blockmesh-redis-1 redis-cli SAVE

# Copy dump file
docker cp blockmesh-redis-1:/data/dump.rdb redis-backup-$(date +%F).rdb
```

### Restore

```bash
# Stop Redis
docker compose stop redis

# Copy backup into volume
docker cp redis-backup-2026-08-19.rdb blockmesh-redis-1:/data/dump.rdb

# Start Redis (loads dump.rdb on startup)
docker compose start redis
```

---

## Automated Backups

### Docker Compose with Cron

Add to host crontab (`crontab -e`):

```bash
# Daily backup at 2 AM, keep 7 days
0 2 * * * docker exec blockmesh-postgres-1 pg_dump -U blockmesh -d blockmesh |   gzip > /backups/blockmesh/blockmesh-$(date +\%F).sql.gz &&   find /backups/blockmesh -name "*.sql.gz" -mtime +7 -delete
```

**Directory setup:**
```bash
sudo mkdir -p /backups/blockmesh
sudo chown $USER:$USER /backups/blockmesh
```

### Kubernetes with CronJob

```yaml
# deployments/base/backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: blockmesh-backup
  namespace: blockmesh
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: pg-dump
            image: postgres:15-alpine
            command:
            - /bin/sh
            - -c
            - |
              pg_dump                 -h postgres                 -U blockmesh                 -d blockmesh                 | gzip > /backups/blockmesh-$(date +%F).sql.gz
            env:
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: blockmesh-secrets
                  key: postgres-password
            volumeMounts:
            - name: backups
              mountPath: /backups
          volumes:
          - name: backups
            persistentVolumeClaim:
              claimName: backup-pvc
          restartPolicy: OnFailure
```

### Offsite Backup

For production, sync backups to S3-compatible storage:

```bash
# After local backup
aws s3 cp /backups/blockmesh/blockmesh-$(date +%F).sql.gz   s3://your-backup-bucket/blockmesh/

# Or with rclone
rclone copy /backups/blockmesh/ remote:backups/blockmesh/
```

---

## Disaster Recovery

### Scenario 1: Database Corruption

1. **Stop all services:**
   ```bash
   docker compose down
   ```

2. **Remove corrupted volume:**
   ```bash
   docker volume rm blockmesh_pgdata
   ```

3. **Recreate volume and restore:**
   ```bash
   docker compose up -d postgres
   sleep 10  # Wait for Postgres to initialize
   cat backup-2026-08-19.sql | docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh
   docker compose up -d
   ```

### Scenario 2: Complete Server Loss

1. **Provision new server** with Docker and Docker Compose
2. **Clone repository:** `git clone ... && cd blockmesh`
3. **Restore `.env` from backup or secret manager**
4. **Start Postgres only:** `docker compose up -d postgres`
5. **Restore database:** `cat backup.sql | docker exec -i ... psql ...`
6. **Start remaining services:** `docker compose up -d`
7. **Verify:** Run the [verification checklist](deploy.md#verification-checklist)

### Scenario 3: Accidental Tenant Deletion

Tenant deletion cascades to API keys, usage, and request logs. **There is no soft delete.**

**Recovery options:**
- Restore from backup to a separate instance, extract the tenant data, re-insert
- If you have the API key prefix, you can identify the tenant from logs but cannot recover the full key

**Prevention:**
- Take backup before bulk operations
- Use database-level `BEFORE DELETE` triggers for audit (custom addition)

---

## Routine Maintenance

### Log Rotation

Docker Compose logs grow unbounded. Configure log rotation:

```yaml
# docker-compose.yml
services:
  gateway:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "3"
```

### Database Maintenance

**Vacuum and analyze (monthly):**
```bash
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "VACUUM ANALYZE;"
```

**Check table bloat:**
```bash
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "SELECT schemaname, tablename, n_tup_ins, n_tup_upd, n_tup_del FROM pg_stat_user_tables;"
```

**Monitor connection usage:**
```bash
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "SELECT count(*) as active_connections FROM pg_stat_activity WHERE datname = 'blockmesh';"
```

### Redis Maintenance

**Check memory usage:**
```bash
docker exec blockmesh-redis-1 redis-cli INFO memory
```

**Check key count:**
```bash
docker exec blockmesh-redis-1 redis-cli DBSIZE
```

**Evict stale rate limit keys manually (rarely needed — they have TTLs):**
```bash
docker exec blockmesh-redis-1 redis-cli --scan --pattern "ratelimit:*" | xargs docker exec blockmesh-redis-1 redis-cli DEL
```

### Request Log Retention

`request_logs` grows indefinitely. For production, implement retention:

```sql
-- Run monthly via cron or pg_cron
DELETE FROM request_logs WHERE created_at < NOW() - INTERVAL '90 days';
VACUUM request_logs;
```

**Or partition by month** (recommended for high-volume deployments):
```sql
-- Create monthly partitions
CREATE TABLE request_logs_2026_08 PARTITION OF request_logs
  FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
```

### Materialized View Refresh

The `request_logs_rollup_1m` view refreshes automatically every 60 seconds. If you need manual refresh:

```bash
docker exec blockmesh-postgres-1 psql -U blockmesh -d blockmesh -c "REFRESH MATERIALIZED VIEW CONCURRENTLY request_logs_rollup_1m;"
```

---

## Backup Verification

Test your backups regularly. A backup you haven't restored is a hope, not a plan.

```bash
# 1. Start a temporary Postgres container
docker run -d --name blockmesh-backup-test   -e POSTGRES_PASSWORD=test   -e POSTGRES_DB=blockmesh   postgres:15-alpine

# 2. Wait and restore
docker exec -i blockmesh-backup-test psql -U postgres -d blockmesh < backup-2026-08-19.sql

# 3. Verify tables exist
docker exec blockmesh-backup-test psql -U postgres -d blockmesh -c "\dt"

# 4. Spot-check data
docker exec blockmesh-backup-test psql -U postgres -d blockmesh -c "SELECT COUNT(*) as tenant_count FROM tenants;"

# 5. Clean up
docker stop blockmesh-backup-test
docker rm blockmesh-backup-test
```

---

## Related Documents

- [Deploy](deploy.md) — Deployment procedures
- [Configure](configure.md) — Environment variable reference
- [Upgrade](upgrade.md) — Zero-downtime upgrades
