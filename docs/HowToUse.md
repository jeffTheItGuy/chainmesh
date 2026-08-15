# How to Use BlockMesh

BlockMesh is a multi-tenant blockchain API gateway. This guide covers everything from getting your first API key to checking usage.

---

## Table of Contents

1. [Get an API Key](#get-an-api-key)
2. [Make Your First RPC Call](#make-your-first-rpc-call)
3. [Check Your Usage](#check-your-usage)
4. [Rate Limits & Caching](#rate-limits--caching)
5. [Common Errors](#common-errors)

---

## Get an API Key

### Via the Dashboard (Recommended)

1. Open the admin dashboard at `http://localhost:3000` (or your configured domain).
2. Click **+ Create Tenant**.
3. Enter a name (e.g., `My App`) and a quota in requests per minute (e.g., `1000`).
4. If your admin set an `ADMIN_SECRET`, enter it in the field.
5. Click **Create**.
6. A green banner appears with your new API key. Click **Copy** immediately — it is shown only once.

The key looks like this:
```
bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6
```

### Via the API

```bash
curl -X POST http://localhost:8081/tenants \
  -H "Content-Type: application/json" \
  -H "X-Admin-Secret: your-secret-here" \
  -d '{"name":"My App","quota_rpm":1000}'
```

Response:
```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "My App",
  "api_key": "bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6",
  "quota_rpm": 1000,
  "created_at": "2026-08-15T15:00:00Z"
}
```

**Copy the `api_key` now.** It is never returned again.

---

## Make Your First RPC Call

Every request goes to the gateway with your API key in the `Authorization` header.

### Example: Get the latest block number

```bash
curl -H "Authorization: Bearer bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6" \
  -X POST http://localhost:8080/v1/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_blockNumber",
    "params": [],
    "id": 1
  }'
```

Response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1234abc"
}
```

### Example: Get an account balance

```bash
curl -H "Authorization: Bearer bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6" \
  -X POST http://localhost:8080/v1/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_getBalance",
    "params": ["0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb", "latest"],
    "id": 1
  }'
```

### Example: Get a block by number

```bash
curl -H "Authorization: Bearer bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6" \
  -X POST http://localhost:8080/v1/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_getBlockByNumber",
    "params": ["latest", true],
    "id": 1
  }'
```

---

## Check Your Usage

### Via the Dashboard

1. Go to the **Tenants** section.
2. Find your tenant in the table.
3. Click **View Usage**.
4. A usage table appears showing method, count, bytes in, and time period.

### Via the API

You can query usage by `api_key` (easiest) or by `tenant` UUID.

**By API key:**
```bash
curl "http://localhost:8081/usage?api_key=bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6"
```

**By tenant ID:**
```bash
curl "http://localhost:8081/usage?tenant=a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

**For a specific day:**
```bash
curl "http://localhost:8081/usage?api_key=bm_live_f8a2b1c4d5e6f7g8h9i0j1k2l3m4n5o6&day=2026-08-15"
```

Response:
```json
[
  {
    "tenant_id": "a1b2c3d4-...",
    "method": "eth_getBalance",
    "count": 1240,
    "bytes_in": 89000,
    "period": "2026-08-15T14:00:00Z"
  }
]
```

---

## Rate Limits & Caching

### Rate Limits

Each tenant has a `quota_rpm` (requests per minute). If you exceed it, you get:

```json
{"error":"rate limit exceeded"}
```

HTTP status: `429 Too Many Requests`

**What to do:** Wait a few seconds and retry. If you consistently hit the limit, ask your admin to increase your quota.

### Caching

BlockMesh caches certain RPC methods to reduce upstream load and latency:

| Method | TTL | Behavior |
|--------|-----|----------|
| `eth_chainId` | 24 hours | Never changes |
| `eth_blockNumber` | 2 seconds | Fresh but safe |
| `eth_getBalance` | 30 seconds | Reasonably fresh |

Check the `X-Cache` header in responses:
- `X-Cache: HIT` — Served from Redis in < 5ms
- `X-Cache: MISS` — Fetched from upstream, then cached

---

## Common Errors

### `401 Unauthorized`

- Missing or malformed `Authorization` header
- Header must start with `Bearer ` (note the space)
- API key does not exist or was revoked

**Fix:**
```bash
curl -H "Authorization: Bearer bm_live_..." ...
```

### `429 Too Many Requests`

- You exceeded your `quota_rpm`
- Counter resets every minute

**Fix:** Back off and retry. Consider caching responses client-side.

### `502 Bad Gateway`

- All upstream RPC endpoints failed
- Network issue between BlockMesh and the blockchain node

**Fix:** Check that `RPC_ENDPOINT_1` and `RPC_ENDPOINT_2` are reachable. Your admin may need to update endpoints in `.env`.

### `400 Invalid JSON`

- Request body is not valid JSON
- Missing required JSON-RPC fields (`jsonrpc`, `method`, `id`)

**Fix:** Validate your JSON payload before sending.

---

## Tips for Production Use

1. **Reuse connections** — Use HTTP keep-alive in your client to avoid connection overhead.
2. **Cache client-side** — Don't re-query `eth_chainId` on every page load.
3. **Handle 429s gracefully** — Implement exponential backoff.
4. **Monitor usage** — Poll `/usage` weekly to understand your traffic patterns.
5. **Secure your key** — Treat your API key like a password. Rotate it if leaked.
