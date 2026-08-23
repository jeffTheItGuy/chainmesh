# Managing Blockchain Networks

Guide for adding, testing, updating, and removing blockchain networks in BlockMesh.

---

## What Is a Network?

A network configuration defines how BlockMesh connects to upstream blockchain RPC nodes. Each network includes:

- **Name** — Human-readable identifier (e.g., "Ethereum Mainnet")
- **RPC Endpoint 1** — Primary node URL (required)
- **RPC Endpoint 2** — Fallback node URL (optional)
- **Chain ID** — Network identifier (e.g., "1", "137")
- **Enabled** — Whether the gateway should use this network

---

## Adding a Network

### Prerequisites

- At least one reachable RPC endpoint (HTTPS recommended)
- The endpoint must support `eth_chainId`
- **SSRF protection is enforced:** Loopback, private, link-local, and multicast IP addresses are rejected. Only public `http` and `https` endpoints are accepted.

### Via Web Dashboard

1. Sign in to the dashboard as admin
2. Navigate to **Infrastructure → Blockchain Networks**
3. Click **Add network**
4. Fill in the form:
   - **Network Name** — e.g., "Ethereum Mainnet"
   - **RPC Endpoint 1** — Primary URL (e.g., `https://ethereum-rpc.publicnode.com`)
   - **RPC Endpoint 2** — Optional fallback
   - **Chain ID** — Optional, auto-detected via test
   - **Enabled** — Check to activate immediately
5. Click **Test Connection** to verify connectivity
6. Click **Add network**

### Via Admin API

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

**Response:**
```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "Ethereum Mainnet",
  "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
  "rpc_endpoint_2": "https://cloudflare-eth.com",
  "chain_id": "1",
  "enabled": true,
  "created_at": "2026-08-19T14:30:00Z",
  "updated_at": "2026-08-19T14:30:00Z"
}
```

---

## Testing Connections

Before adding or after updating endpoints, verify connectivity:

### Dashboard

Click **Test Connection** in the network form. The dashboard will:
1. Send `eth_chainId` to the primary endpoint
2. If provided, test the secondary endpoint
3. Display the detected chain ID or error message

### API

```bash
curl -X POST http://localhost:8081/blockchain/test \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "rpc_endpoint_1": "https://ethereum-rpc.publicnode.com",
    "rpc_endpoint_2": "https://cloudflare-eth.com"
  }'
```

**Success response:**
```json
{
  "connected": true,
  "chain_id": "1"
}
```

**Failure response:**
```json
{
  "connected": false,
  "error": "connection timeout"
}
```

---

## Listing Networks

### Dashboard

The Blockchain Networks page shows all networks with their chain ID, endpoints, and enabled status.

### API

```bash
curl http://localhost:8081/blockchain \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

---

## Updating a Network

### Dashboard

1. Find the network in the table
2. Click **Edit**
3. Modify endpoints, chain ID, or enabled status
4. Click **Update network**

### API

```bash
curl -X PUT http://localhost:8081/blockchain/<network-id> \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ethereum Mainnet",
    "rpc_endpoint_1": "https://new-endpoint.example.com",
    "rpc_endpoint_2": "https://backup.example.com",
    "chain_id": "1",
    "enabled": true
  }'
```

**Important:** Both `name` and `rpc_endpoint_1` are required in the update payload.

---

## Enabling/Disabling Networks

Disabled networks are ignored by the gateway:

- Existing tenant assignments remain but fall back to the default network
- Health checks stop for disabled networks
- The ingestor stops polling disabled networks

To disable:

```bash
curl -X PUT http://localhost:8081/blockchain/<network-id> \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ethereum Mainnet",
    "rpc_endpoint_1": "https://...",
    "enabled": false
  }'
```

---

## Deleting a Network

**Warning:** Deletion unlinks the network from tenants and blocks first, then removes the configuration.

### Dashboard

1. Find the network in the table
2. Click **Delete**
3. Confirm the deletion

### API

```bash
curl -X DELETE http://localhost:8081/blockchain/<network-id> \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

**Behavior:**
- `tenants.blockchain_network_id` → set to `NULL` for affected tenants
- `blocks.network_id` → set to `NULL` for affected blocks
- Network config is then deleted

Affected tenants will fall back to the default network on their next request.

---

## Health Checks

BlockMesh probes each endpoint every 10 seconds:

- **Probe method:** `eth_chainId`
- **Timeout:** 10 seconds
- **Failure threshold:** 3 consecutive failures marks endpoint unhealthy
- **Recovery:** 1 successful probe marks endpoint healthy

### Viewing Health Status

**Dashboard:** Navigate to **Infrastructure → Node status**

**API:**
```bash
curl http://localhost:8080/health/nodes
```

**Response (endpoint URLs redacted):**
```json
[
  {
    "network_id": "a1b2c3d4...",
    "nodes": [
      {
        "url": "ethereum-rpc.publicnode.com/**",
        "healthy": true,
        "latency_ms": 82,
        "last_check": "2026-08-19T14:30:00Z",
        "consecutive_fails": 0,
        "total_requests": 15234,
        "total_failures": 12
      }
    ]
  }
]
```

---

## Routing Behavior

When a request arrives, the gateway routes it as follows:

1. **Tenant has assigned network?** → Use that network
2. **No assignment?** → Use earliest enabled network (by `created_at`)
3. **Network has multiple endpoints?** → Route to fastest healthy endpoint
4. **Primary endpoint fails?** → Retry with next healthy endpoint
5. **All endpoints fail?** → Return `502 Bad Gateway`

---

## Recommended Endpoint Providers

| Network | Free Public Endpoints | Paid Providers |
|---------|----------------------|----------------|
| Ethereum Mainnet | publicnode.com, cloudflare-eth.com | Alchemy, Infura, QuickNode |
| Sepolia | sepolia.publicnode.com | Alchemy, Infura |
| Polygon | polygon.publicnode.com | Alchemy, Infura |
| Arbitrum | arbitrum.publicnode.com | Alchemy, Infura |

**Tips:**
- Use at least 2 endpoints from different providers for redundancy
- Public endpoints may have stricter rate limits than paid ones
- Always use HTTPS endpoints in production

---

## Multi-Network Setup Example

Configure multiple networks for different use cases:

```bash
# Ethereum Mainnet
curl -X POST http://localhost:8081/blockchain ... -d '{
  "name": "Ethereum Mainnet",
  "rpc_endpoint_1": "https://eth-mainnet.g.alchemy.com/v2/...",
  "rpc_endpoint_2": "https://cloudflare-eth.com",
  "chain_id": "1",
  "enabled": true
}'

# Sepolia Testnet
curl -X POST http://localhost:8081/blockchain ... -d '{
  "name": "Sepolia Testnet",
  "rpc_endpoint_1": "https://eth-sepolia.g.alchemy.com/v2/...",
  "chain_id": "11155111",
  "enabled": true
}'

# Polygon
curl -X POST http://localhost:8081/blockchain ... -d '{
  "name": "Polygon Mainnet",
  "rpc_endpoint_1": "https://polygon-rpc.com",
  "rpc_endpoint_2": "https://polygon.publicnode.com",
  "chain_id": "137",
  "enabled": true
}'
```

Then assign tenants to specific networks as needed.

---

## Troubleshooting

### "no blockchain network configured" (503)

- No networks exist → Add at least one network
- All networks disabled → Enable at least one network

### "blockchain network unavailable" (503)

- All endpoints for the tenant's network are unhealthy
- Check `/health/nodes` and verify endpoints are reachable
- Endpoints may recover automatically (health checks every 10s)

### Connection test fails

- Verify the URL is correct and accessible from the BlockMesh server
- Check if the endpoint requires authentication (API key in URL)
- Some providers block requests without proper `User-Agent` — BlockMesh sends `BlockMesh-Gateway/1.0`
- Firewall or WAF may be blocking the gateway IP
- **SSRF protection:** Loopback (`127.0.0.1`, `::1`), private (`10.x.x.x`, `192.168.x.x`, `172.16-31.x.x`), link-local, and multicast addresses are rejected. Only public endpoints are accepted.

---

## Related Documents

- [Managing Tenants](add-tenants.md) — Create and assign tenants to networks
- [Monitoring](monitoring.md) — Health checks and observability
- [Security](../operators/security.md) — Endpoint security and redaction
