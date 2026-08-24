#!/bin/sh
set -e

# ─── Clean slate ─────────────────────────────────────────────────────────────
# Remove contents but not the mount point itself (it's a Docker volume)
if [ -d /app/test-results ]; then
    find /app/test-results -mindepth 1 -delete 2>/dev/null || true
fi
mkdir -p /app/test-results/logs
mkdir -p /app/test-results/summaries

# ─── Migrations ──────────────────────────────────────────────────────────────
echo "=== Migrations ===" | tee /app/test-results/logs/test.log
migrate -path ./database/migrations -database "$TEST_DATABASE_URL" up

# ─── Unit Tests ────────────────────────────────────────────────────────────────
echo "=== Unit Tests ===" | tee -a /app/test-results/logs/test.log
go test ./shared/util/... ./shared/blockchain/... ./gateway/middleware/... \
  -v 2>&1 | tee /app/test-results/logs/unit.log

# ─── Integration Tests (Postgres) ────────────────────────────────────────────
echo "=== Integration Tests (Postgres) ===" | tee -a /app/test-results/logs/test.log
TEST_DATABASE_URL="$TEST_DATABASE_URL" go test ./admin/... ./shared/storage/postgres/... ./gateway/proxy/... ./shared/telemetry/... \
  -v 2>&1 | tee /app/test-results/logs/integration.log

# ─── Integration Tests (Redis) ─────────────────────────────────────────────────
echo "=== Integration Tests (Redis) ===" | tee -a /app/test-results/logs/test.log
REDIS_ADDR="$TEST_REDIS_ADDR" go test ./shared/storage/redis/... \
  -v 2>&1 | tee /app/test-results/logs/redis.log

# ─── Coverage & JSON Report ────────────────────────────────────────────────────
echo "=== Coverage & JSON Report ===" | tee -a /app/test-results/logs/test.log
go test ./... \
  -coverprofile=/app/test-results/logs/coverage.out \
  -json > /app/test-results/summaries/test.json \
  2> /app/test-results/logs/test-errors.log || true

go tool cover -html=/app/test-results/logs/coverage.out \
  -o /app/test-results/summaries/coverage.html

go tool cover -func=/app/test-results/logs/coverage.out \
  > /app/test-results/summaries/coverage-summary.txt

echo "=== Done ===" | tee -a /app/test-results/logs/test.log