# Adding Blockchain Networks

BlockMesh routes tenant traffic through the blockchain networks you configure.
Each network has a **primary** RPC endpoint and an optional **failover** endpoint.
The gateway health-checks both every 10 seconds and automatically routes requests
to the fastest healthy node.

---

## Prerequisites

- The admin service running (default: `http://localhost:8081`, or `http://localhost/api/` behind nginx)
- Your `ADMIN_SECRET` exported:

```bash
export ADMIN_SECRET=your-admin-secret
```

---

## Option 1 — Admin Console (recommended)

1. Sign in to the console with the admin secret.
2. Open **Infrastructure → Blockchain Networks**.
3. Click **Add network**.
4. Fill in the form:
   - **Network Name** — display name, e.g. `Ethereum Mainnet`
   - **RPC Endpoint 1** — primary endpoint (required)
   - **RPC Endpoint 2** — failover endpoint (optional but recommended)
   - **Chain ID** — optional; auto-filled if you test the connection
   - **Enabled** — leave checked so the gateway picks it up
5. Click **Test Connection** to verify the endpoint responds to `eth_chainId`.
   On success the Chain ID field is filled automatically.
6. Click **Add network** to save.

The gateway reloads its network list every 15 seconds — no restart required.

---

## Option 2 — Admin API

### Create a network

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

The response contains the new network record — **save the `id`** if you plan to
pin tenants to this network:

```json
{
  "id": "6f1c8a92-4b7e-4d21-9c3a-2e8d5f0b1a77",
  "name": "Ethereum Mainnet",
  "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
  "rpc_endpoint_2": "https://cloudflare-eth.com",
  "chain_id": "1",
  "enabled": true,
  "created_at": "2026-08-19T12:00:00Z",
  "updated_at": "2026-08-19T12:00:00Z"
}
```

### Test a connection before saving

```bash
curl -X POST http://localhost:8081/blockchain/test \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
    "rpc_endpoint_2": "https://cloudflare-eth.com"
  }'
```

Success:

```json
{ "connected": true, "chain_id": "1" }
```

Failure:

```json
{ "connected": false, "error": "dial tcp: lookup rpc.example.com: no such host" }
```

### List / update / delete

```bash
# List all networks
curl http://localhost:8081/blockchain \
  -H "X-Admin-Secret: $ADMIN_SECRET"

# Update a network (full payload required)
curl -X PUT http://localhost:8081/blockchain/<NETWORK_ID> \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ethereum Mainnet",
    "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
    "rpc_endpoint_2": "https://cloudflare-eth.com",
    "chain_id": "1",
    "enabled": true
  }'

# Delete a network
curl -X DELETE http://localhost:8081/blockchain/<NETWORK_ID> \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

Deleting a network safely unlinks any tenants and blocks referencing it first —
tenants fall back to the default network (the earliest enabled one).

---

## Option 3 — Seed with SQL

Useful for reproducible deployments:

```sql
INSERT INTO blockchain_configs (name, rpc_endpoint_1, rpc_endpoint_2, chain_id, enabled)
VALUES
  ('Ethereum Mainnet', 'https://ethereum-rpc.publicnode.com', 'https://cloudflare-eth.com', '1', true),
  ('Sepolia Testnet',  'https://rpc.sepolia.org', 'https://ethereum-sepolia-rpc.publicnode.com', '11155111', true),
  ('Polygon PoS',      'https://polygon-bor-rpc.publicnode.com', 'https://polygon-rpc.com', '137', true),
  ('Arbitrum One',     'https://arb1.arbitrum.io/rpc', 'https://arbitrum-one-rpc.publicnode.com', '42161', true),
  ('Base',             'https://mainnet.base.org', 'https://base-rpc.publicnode.com', '8453', true)
ON CONFLICT DO NOTHING;
```

---

## Example Networks

Well-known public RPC endpoints suitable for development and testing:

| Network          | Chain ID   | RPC Endpoint 1 (primary)                 | RPC Endpoint 2 (failover)                     |
| ---------------- | ---------- | ---------------------------------------- | --------------------------------------------- |
| Ethereum Mainnet | `1`        | `https://ethereum-rpc.publicnode.com`    | `https://cloudflare-eth.com`                  |
| Sepolia Testnet  | `11155111` | `https://rpc.sepolia.org`                | `https://ethereum-sepolia-rpc.publicnode.com` |
| Polygon PoS      | `137`      | `https://polygon-bor-rpc.publicnode.com` | `https://polygon-rpc.com`                     |
| Arbitrum One     | `42161`    | `https://arb1.arbitrum.io/rpc`           | `https://arbitrum-one-rpc.publicnode.com`     |
| Optimism         | `10`       | `https://mainnet.optimism.io`            | `https://optimism-rpc.publicnode.com`         |
| Base             | `8453`     | `https://mainnet.base.org`               | `https://base-rpc.publicnode.com`             |
| BNB Smart Chain  | `56`       | `https://bsc-dataseed.binance.org`       | `https://bsc-rpc.publicnode.com`              |

> **Production note:** public endpoints are rate-limited and may drop you at any
> time. For production tenants, use provider URLs with your own keys
> (Alchemy / Infura / QuickNode). The node-health endpoint redacts URL paths,
> so embedded provider keys are never exposed in the browser.

---

## How networks are used

Once a network is saved and enabled:

| Component  | Behavior                                                                       |
| ---------- | ------------------------------------------------------------------------------ |
| Gateway    | Reloads configs every 15s; health-checks each endpoint every 10s via `eth_chainId` |
| Gateway    | Routes each request to the **fastest healthy** endpoint, failing over automatically |
| Gateway    | Returns `503 Service Unavailable` if a tenant's network has no healthy nodes    |
| Ingestor   | Polls `eth_getBlockByNumber` every 12s and stores recent blocks per network     |
| Admin      | Networks appear in the tenant form's "Blockchain Network" dropdown              |

### Assigning networks to tenants

- **Explicit:** create or edit a tenant with `blockchain_network_id` set to the network's ID.
- **Default:** omit the field and the tenant uses the default network
  (the earliest enabled network by `created_at`).

```bash
curl -X POST http://localhost:8081/tenants \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "plan": "pro",
    "quota_rps": 10,
    "quota_rpm": 1000,
    "quota_daily": 100000,
    "blockchain_network_id": "<NETWORK_ID>"
  }'
```

### Verifying node health

```bash
curl -s http://localhost:8080/health/nodes | jq '.[0].nodes[0]'
```

```json
{
  "url": "ethereum-rpc.publicnode.com/***",
  "healthy": true,
  "latency_ms": 82,
  "last_check": "2026-08-19T12:00:00Z",
  "consecutive_fails": 0,
  "total_requests": 4120,
  "total_failures": 3
}
```

A node is marked unhealthy after **3 consecutive failed health checks** and
recovers automatically once checks succeed again.

---

## Troubleshooting

| Symptom | Cause / Fix |
| ------- | ----------- |
| Tenant creation fails: `no blockchain network available — configure one first` | No networks exist yet (or none are enabled). Add one first. |
| Test Connection fails with a timeout | Endpoint is unreachable or blocked by a firewall/WAF. Verify the URL responds to `curl -X POST <url> -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'`. |
| Test Connection fails: `invalid rpc response` | Endpoint returned non-JSON-RPC data (often an HTML error page from a proxy or WAF). |
| Node shows **Down** in the console | Three consecutive health-check failures. Check the endpoint; it recovers automatically. |
| Requests return `503 blockchain network unavailable` | The tenant's network has no healthy nodes, or the network was deleted/disabled. |
| Requests return `503` for a new tenant with no explicit network | No default network exists — enable at least one network. |