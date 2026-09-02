<!-- load-test.md -->
# Load Testing

Load testing validates that the ChainMesh gateway can handle expected traffic without excessive latency or error rate.

The load test uses k6.

---

## Implemented Load Script

The k6 load-test script is:

```text
tests/load/gateway_load.js
```

The GitHub Actions workflow is:

```text
.github/workflows/load-test.yml
```

A convenience runner is also available:

```text
tests/load/run-load-test.sh
```

---

## What the Load Test Does

The load test sends authenticated JSON-RPC requests to the gateway.

Request method:

```text
eth_chainId
```

Request shape:

```json
{
  "jsonrpc": "2.0",
  "method": "eth_chainId",
  "params": [],
  "id": 1
}
```

Each virtual user sleeps for one second between requests.

---

## Load Stages

| Stage | Duration | Target VUs |
|---|---:|---:|
| Ramp up | 2m | 100 |
| Steady state | 5m | 100 |
| Stress | 2m | 200 |
| Ramp down | 2m | 0 |

---

## Implemented Thresholds

The load test fails if either threshold is breached.

| Metric | Threshold |
|---|---|
| p95 latency | `< 250ms` |
| Error rate | `< 1%` |

Implemented k6 thresholds:

```javascript
thresholds: {
  http_req_duration: ['p(95)<250'],
  http_req_failed: ['rate<0.01'],
}
```

---

## Implemented Checks

Each request is checked for:

| Check | Meaning |
|---|---|
| `status is 200` | HTTP response status is 200 |
| `has result` | JSON-RPC response contains a `result` field |
| `cache header present` | `X-Cache` header is present |

---

## Load Summary

The k6 script generates a markdown summary.

By default, the summary is written to:

```text
/tmp/chainmesh-load/load-summary.md
```

The summary includes:

- p95 latency
- Average latency
- Error rate
- Total request count
- Pass/fail status for thresholds

Example summary shape:

```markdown
## ✅ ChainMesh Load Test Results

| Metric | Value | Threshold | Status |
|---|---:|---|---|
| p(95) Latency | 120.35 ms | < 250 ms | ✅ Pass |
| Avg Latency | 88.10 ms | — | — |
| Error Rate | 0.00% | < 1% | ✅ Pass |

*Total requests: 123456*
```

---

## Running Locally

If k6 is installed locally, run:

```bash
k6 run \
  --env GATEWAY_URL="http://localhost:8080/v1/" \
  --env TEST_API_KEY="your-test-api-key" \
  --env SUMMARY_PATH="./load-summary.md" \
  tests/load/gateway_load.js
```

Or use the helper script:

```bash
GATEWAY_URL="http://localhost:8080/v1/" \
TEST_API_KEY="your-test-api-key" \
./tests/load/run-load-test.sh
```

---

## Running via GitHub Actions

The load-test workflow is manually triggered.

Workflow:

```text
.github/workflows/load-test.yml
```

Trigger:

```yaml
on:
  workflow_dispatch:
```

Required secrets:

| Secret | Purpose |
|---|---|
| `REMOTE_SSH` | SSH private key for the k3s node |
| `REMOTE_SERVER` | Hostname or address of the k3s node |
| `REMOTE_USER` | SSH user on the k3s node |
| `PROD_TEST_API_KEY` | Live test tenant API key |

The workflow:

1. Checks out the repository
2. Configures SSH
3. Copies `tests/load/gateway_load.js` to the k3s node
4. Installs k6 on the node if needed
5. Resolves the gateway ClusterIP
6. Runs the k6 load test inside the cluster
7. Fetches the markdown summary
8. Publishes the summary to the GitHub Actions job summary
9. Fails the job if thresholds are breached

---

## Environment Variables

| Variable | Purpose | Default |
|---|---|---|
| `GATEWAY_URL` | Gateway URL to load test | `http://localhost:8080/v1/` |
| `TEST_API_KEY` | API key used for authenticated requests | empty |
| `SUMMARY_PATH` | Output path for markdown summary | `/tmp/chainmesh-load/load-summary.md` |

---

## What the Load Test Does Not Currently Measure

The current load test does **not** measure or enforce:

- Cache hit rate
- p50 latency threshold
- Rate-limit 429 spike behavior
- Redis saturation
- Postgres saturation
- Node CPU/memory saturation
- Per-endpoint latency breakdown

Those may be added later if the load-test suite is expanded.

---

## Related Documents

- [readme.md](readme.md)
- [results.md](results.md)
- [unit-test.md](unit-test.md)
- [integrations-test.md](integrations-test.md)
- [smoke-test.md](smoke-test.md)