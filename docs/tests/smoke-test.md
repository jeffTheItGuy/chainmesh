<!-- smoke-test.md -->
# Smoke Testing

Smoke tests verify that a deployed ChainMesh environment is fundamentally healthy.

The current smoke implementation runs **inside the k3s cluster** and hits ClusterIP services directly. It does not go through external DNS, Cloudflare, or public ingress URLs.

---

## Implemented Smoke Script

The smoke test is implemented in:

```text
tests/smoke/smoke-k3s.sh
```

The GitHub Actions workflow is:

```text
.github/workflows/smoke-test.yml
```

---

## Smoke Checks

The smoke script performs up to six checks.

| Check | What it verifies |
|---|---|
| Gateway reachable | The gateway ClusterIP service responds over HTTP |
| Authenticated RPC | A valid API key can make a JSON-RPC call and receive a result |
| Cache MISS, first call | A cold cacheable request returns `X-Cache: MISS` |
| Cache HIT, second call | The same request returns `X-Cache: HIT` |
| Rate limit headers | Rate-limit headers are present on a successful RPC response |
| Web dashboard | The web service responds, if a web service exists in the namespace |

The web dashboard check is skipped if no web service is found.

If `TEST_API_KEY` is not set, authenticated checks are skipped.

---

## Cache Check Behavior

The smoke script uses a guaranteed-cold cache key for MISS/HIT validation:

```json
{
  "jsonrpc": "2.0",
  "method": "eth_getBalance",
  "params": ["<random-address>", "latest"],
  "id": 1
}
```

This avoids false failures caused by warm shared cache entries.

---

## Running the Smoke Test via GitHub Actions

The smoke workflow is manually triggered.

Workflow:

```text
.github/workflows/smoke-test.yml
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
3. Copies `tests/smoke/smoke-k3s.sh` to the k3s node
4. Runs the smoke script on the node
5. Fetches the results file
6. Publishes results to the GitHub Actions job summary
7. Fails the job if any smoke check fails

---

## Running the Smoke Script Manually

Run the script on the k3s node:

```bash
NAMESPACE=chainmesh \
TEST_API_KEY="your-test-api-key" \
RESULTS_FILE="/tmp/chainmesh-smoke/results.md" \
bash tests/smoke/smoke-k3s.sh
```

Environment variables:

| Variable | Purpose | Default |
|---|---|---|
| `KUBECONFIG` | Path to k3s kubeconfig | `/etc/rancher/k3s/k3s.yaml` |
| `NAMESPACE` | Kubernetes namespace | `chainmesh` |
| `TEST_API_KEY` | API key used for authenticated checks | empty |
| `RESULTS_FILE` | Markdown results output path | `/tmp/chainmesh-smoke/results.md` |

---

## Smoke Results

The smoke script writes a markdown summary to `RESULTS_FILE`.

Example output shape:

```markdown
## ✅ ChainMesh Smoke Test — in-cluster

**Namespace:** `chainmesh` · **Gateway svc:** `chainmesh-gateway-svc` · **Passed:** 6/6

| Check | Status | Detail |
|---|---|---|
| Gateway reachable | ✅ Pass | HTTP 401 |
| Authenticated RPC | ✅ Pass | returned result |
| Cache MISS (1st call) | ✅ Pass | X-Cache: MISS |
| Cache HIT (2nd call) | ✅ Pass | X-Cache: HIT |
| Rate limit headers | ✅ Pass | present |
| Web dashboard | ✅ Pass | HTTP 200 |
```

---

## Latency Baseline Script

A separate latency baseline script is available:

```text
tests/smoke/baseline.sh
```

It sends a fixed number of authenticated requests and reports average, minimum, and maximum latency.

Required environment variables:

```bash
export GATEWAY_URL="http://gateway-cluster-ip:port/v1/"
export TEST_API_KEY="your-test-api-key"
```

Run it:

```bash
bash tests/smoke/baseline.sh
```

Alert thresholds used by the script:

| Condition | Meaning |
|---|---|
| Average latency > 100ms | Investigate |
| Average latency > 300ms | Critical; script exits non-zero |

---

## What Smoke Tests Do Not Cover

The current smoke suite does **not** cover:

- External public URLs
- Admin API health
- Admin `/health/nodes`
- Full data-integrity verification
- Failover behavior
- Redis failure behavior
- Postgres failure behavior
- Security regression testing
- Load testing

Those areas are either covered elsewhere or are not currently automated.

---

## Related Documents

- [readme.md](readme.md)
- [results.md](results.md)
- [unit-test.md](unit-test.md)
- [integrations-test.md](integrations-test.md)
- [load-test.md](load-test.md)