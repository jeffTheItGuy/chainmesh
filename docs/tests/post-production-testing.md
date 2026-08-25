# Post-Production Testing

Post-production tests validate a live ChainMesh deployment after release, infrastructure change, or incident recovery. These are not run in CI — they run against real environments.

---

## Table of Contents

1. [When to Run](#when-to-run)
2. [Smoke Tests](#smoke-tests)
3. [Health & Latency Baseline](#health--latency-baseline)
4. [Load Testing](#load-testing)
5. [Failover & Chaos Tests](#failover--chaos-tests)
6. [Security Validation](#security-validation)
7. [Data Integrity Checks](#data-integrity-checks)
8. [Post-Production Checklist](#post-production-checklist)
9. [Automation](#automation)

---

## When to Run

| Trigger | Tests to Run |
|---------|-------------|
| After every deployment | Smoke tests |
| Weekly (scheduled) | Health checks + latency baseline |
| After infrastructure changes | Full smoke + load test |
| After incident recovery | Full suite + chaos validation |
| Before high-traffic events | Load test + capacity review |

---

## Smoke Tests

Fast checks that the deployment is fundamentally healthy. Should complete in < 60 seconds.

### Prerequisites

```bash
export GATEWAY_URL="https://api.chainmesh.example.com/v1/"
export ADMIN_URL="https://admin.chainmesh.example.com"
export ADMIN_SECRET="your-live-admin-secret"
export TEST_API_KEY="your-live-test-tenant-key"
```

### Test Script

```bash
#!/bin/bash
# tests/smoke/smoke.sh
set -e

echo "=== ChainMesh Smoke Tests ==="

# 1. Admin API health
curl -sf "${ADMIN_URL}/health" | grep -q '"status":"ok"' && echo "✓ Admin API healthy"

# 2. Gateway health/nodes (no auth required)
curl -sf "${ADMIN_URL}/health/nodes" > /dev/null && echo "✓ Node health endpoint reachable"

# 3. Authenticated RPC call
RESPONSE=$(curl -sf -X POST "${GATEWAY_URL}"   -H "Authorization: Bearer ${TEST_API_KEY}"   -H "Content-Type: application/json"   -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}')
echo "${RESPONSE}" | grep -q '"result"' && echo "✓ Authenticated RPC works"

# 4. Cache behavior
RESPONSE1=$(curl -s -X POST "${GATEWAY_URL}"   -H "Authorization: Bearer ${TEST_API_KEY}"   -H "Content-Type: application/json"   -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'   -D - | grep "X-Cache:")
echo "${RESPONSE1}" | grep -q "MISS" && echo "✓ Cache miss on first call"

RESPONSE2=$(curl -s -X POST "${GATEWAY_URL}"   -H "Authorization: Bearer ${TEST_API_KEY}"   -H "Content-Type: application/json"   -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'   -D - | grep "X-Cache:")
echo "${RESPONSE2}" | grep -q "HIT" && echo "✓ Cache hit on second call"

# 5. Rate limit headers present
HEADERS=$(curl -s -X POST "${GATEWAY_URL}"   -H "Authorization: Bearer ${TEST_API_KEY}"   -H "Content-Type: application/json"   -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'   -D -)
echo "${HEADERS}" | grep -q "X-RateLimit-Remaining-Minute" && echo "✓ Rate limit headers present"

# 6. Dashboard loads
curl -sf "https://chainmesh.example.com" > /dev/null && echo "✓ Dashboard reachable"

echo "=== All smoke tests passed ==="
```

Run it:

```bash
chmod +x tests/smoke/smoke.sh
./tests/smoke/smoke.sh
```

---

## Health & Latency Baseline

Capture metrics to detect drift over time.

### Script

```bash
#!/bin/bash
# tests/smoke/baseline.sh

GATEWAY_URL="https://api.chainmesh.example.com/v1/"
TEST_API_KEY="your-live-test-tenant-key"
SAMPLES=20

echo "Collecting ${SAMPLES} latency samples..."

TOTAL=0
MIN=999999
MAX=0

for i in $(seq 1 $SAMPLES); do
    START=$(date +%s%N)
    curl -sf -X POST "${GATEWAY_URL}"         -H "Authorization: Bearer ${TEST_API_KEY}"         -H "Content-Type: application/json"         -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' > /dev/null
    END=$(date +%s%N)

    LATENCY=$(( (END - START) / 1000000 ))  # ms
    TOTAL=$((TOTAL + LATENCY))

    [ $LATENCY -lt $MIN ] && MIN=$LATENCY
    [ $LATENCY -gt $MAX ] && MAX=$LATENCY

    echo "  Sample $i: ${LATENCY}ms"
done

AVG=$((TOTAL / SAMPLES))

echo ""
echo "=== Baseline Results ==="
echo "Average: ${AVG}ms"
echo "Min:     ${MIN}ms"
echo "Max:     ${MAX}ms"
echo ""
echo "Alert thresholds:"
echo "  AVG > 100ms  → investigate"
echo "  AVG > 300ms  → page on-call"
```

---

## Load Testing

Verify the gateway handles expected traffic without degrading.

### Using k6

```javascript
// tests/load/gateway_load.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '2m', target: 100 },   // Ramp up
        { duration: '5m', target: 100 },   // Steady state
        { duration: '2m', target: 200 },   // Stress
        { duration: '2m', target: 0 },     // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<250'],   // 95th percentile < 250ms
        http_req_failed: ['rate<0.01'],     // Error rate < 1%
    },
};

const GATEWAY_URL = __ENV.GATEWAY_URL || 'http://localhost:8080/v1/';
const API_KEY = __ENV.TEST_API_KEY;

export default function () {
    const payload = JSON.stringify({
        jsonrpc: '2.0',
        method: 'eth_chainId',
        params: [],
        id: 1,
    });

    const res = http.post(GATEWAY_URL, payload, {
        headers: {
            'Authorization': `Bearer ${API_KEY}`,
            'Content-Type': 'application/json',
        },
    });

    check(res, {
        'status is 200': (r) => r.status === 200,
        'has result': (r) => JSON.parse(r.body).result !== undefined,
        'cache header present': (r) => r.headers['X-Cache'] !== undefined,
    });

    sleep(1);
}
```

Run it:

```bash
k6 run --env GATEWAY_URL=https://api.chainmesh.example.com/v1/        --env TEST_API_KEY=your-key        tests/load/gateway_load.js
```

### Expected Results

| Metric | Baseline | Under Load | Alert If |
|--------|----------|------------|----------|
| p50 latency | < 50ms | < 100ms | > 200ms |
| p95 latency | < 120ms | < 250ms | > 500ms |
| Error rate | 0% | < 1% | > 5% |
| Cache hit rate | > 40% | > 30% | < 20% |
| Rate limit 429s | 0 | Expected | Sudden spike |

---

## Failover & Chaos Tests

Verify resilience by introducing controlled failures.

### Test: Primary Endpoint Failure

```bash
# 1. Note current primary endpoint
ADMIN_URL="https://admin.chainmesh.example.com"
ADMIN_SECRET="your-secret"

curl -s "${ADMIN_URL}/health/nodes" -H "X-Admin-Secret: ${ADMIN_SECRET}"

# 2. Temporarily block primary endpoint (simulated via firewall or provider maintenance mode)
#    Or update the network config to point primary to a blackhole URL

# 3. Verify requests still succeed (fallback to secondary)
for i in {1..10}; do
    curl -sf -X POST "${GATEWAY_URL}"         -H "Authorization: Bearer ${TEST_API_KEY}"         -H "Content-Type: application/json"         -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' > /dev/null
    echo "Request $i: OK"
    sleep 1
done

# 4. Restore primary endpoint
```

### Test: Redis Failure (Rate Limiter)

```bash
# 1. Stop Redis (Docker Compose)
docker compose stop redis

# 2. Verify gateway returns 503 (fail closed)
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${GATEWAY_URL}"     -H "Authorization: Bearer ${TEST_API_KEY}"     -H "Content-Type: application/json"     -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}')

if [ "$RESPONSE" -eq 503 ]; then
    echo "✓ Gateway correctly fails closed when Redis is down"
else
    echo "✗ Expected 503, got $RESPONSE"
fi

# 3. Restart Redis
docker compose start redis
```

### Test: Postgres Failure (Telemetry)

```bash
# 1. Stop Postgres
docker compose stop postgres

# 2. Verify gateway still serves requests (telemetry drops, no blocking)
#    Check Prometheus: blockmesh_gateway_telemetry_dropped_total should increase

# 3. Restart Postgres
docker compose start postgres
```

---

## Security Validation

Run these after any auth-related deployment.

### Test: Invalid API Key Rejected

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST "${GATEWAY_URL}"     -H "Authorization: Bearer bm_live_INVALIDKEY0000000000000000"     -H "Content-Type: application/json"     -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
# Expected: 401
```

### Test: Missing Admin Secret Rejected

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST "${ADMIN_URL}/tenants"     -H "Content-Type: application/json"     -d '{"name":"test"}'
# Expected: 403
```

### Test: SSRF Protection

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST "${ADMIN_URL}/blockchain"     -H "X-Admin-Secret: ${ADMIN_SECRET}"     -H "Content-Type: application/json"     -d '{"name":"bad","rpc_endpoint_1":"http://127.0.0.1:8545","enabled":true}'
# Expected: 400 (SSRF blocked)
```

---

## Data Integrity Checks

Verify that telemetry and block ingestion are working.

### Check: Recent Blocks Ingested

```bash
curl -s "${ADMIN_URL}/blocks" | jq '. | length'
# Expected: 50 (or however many your ingestor retains)
```

### Check: Usage Records Written

```bash
# Query today's usage for the test tenant
curl -s "${ADMIN_URL}/tenants/${TEST_TENANT_ID}/usage"     -H "X-Admin-Secret: ${ADMIN_SECRET}" | jq '. | length'
# Expected: > 0 after making test requests
```

### Check: Audit Logs Recording

```bash
curl -s "${ADMIN_URL}/audit-logs?limit=5"     -H "X-Admin-Secret: ${ADMIN_SECRET}" | jq '.[0].action'
# Expected: Recent actions like CREATE_TENANT, UPDATE_BLOCKCHAIN_CONFIG
```

---

## Post-Production Checklist

Use this after every deployment:

- [ ] Smoke tests pass (all 6 checks)
- [ ] Baseline latency within normal range
- [ ] No new errors in gateway logs (`docker compose logs gateway | grep -i error`)
- [ ] No new errors in admin logs
- [ ] All nodes healthy (`/health/nodes`)
- [ ] Dashboard loads without console errors
- [ ] Prometheus metrics endpoint reachable (if exposed)
- [ ] Recent blocks are being ingested
- [ ] Usage telemetry is recording
- [ ] (If applicable) Load test completed successfully

---

## Automation

Consider scheduling smoke tests via GitHub Actions or a cron job:

```yaml
# .github/workflows/smoke.yml
name: Production Smoke Tests
on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  workflow_dispatch:

jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run smoke tests
        run: ./tests/smoke/smoke.sh
        env:
          GATEWAY_URL: ${{ secrets.PROD_GATEWAY_URL }}
          ADMIN_URL: ${{ secrets.PROD_ADMIN_URL }}
          ADMIN_SECRET: ${{ secrets.PROD_ADMIN_SECRET }}
          TEST_API_KEY: ${{ secrets.PROD_TEST_API_KEY }}
```

---

## Related Documents

- [Smoke Tests](../operators/deploy.md#verification-checklist) — Deployment verification
- [Monitoring](../admins/monitoring.md) — Dashboard metrics and alerting
- [Integration Testing](integration-testing.md) — Pre-production cross-service tests
- [Security](../operators/security.md) — Security model and hardening
- [Testing Strategy](README.md) — Overview of the test pyramid
