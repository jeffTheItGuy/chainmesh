# Quick Start

Get up and running with the BlockMesh API in under 5 minutes.

---

## What You Need

1. A BlockMesh API key (provided by your administrator)
2. `curl` or any HTTP client
3. A blockchain network configured on the gateway

---

## Your First Request

All API calls go through the gateway endpoint. Replace `<api-key>` with your actual key.

### Check the Chain ID

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_chainId",
    "params": [],
    "id": 1
  }'
```

**Expected response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1"
}
```

The `result` is a hex-encoded chain ID. `0x1` = Ethereum Mainnet.

---

## Common Operations

### Get Account Balance

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_getBalance",
    "params": ["0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb", "latest"],
    "id": 2
  }'
```

### Get Latest Block Number

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_blockNumber",
    "params": [],
    "id": 3
  }'
```

### Send a Raw Transaction

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_sendRawTransaction",
    "params": ["0xf86c..."],
    "id": 4
  }'
```

---

## Response Format

All responses follow the JSON-RPC 2.0 specification:

| Field | Type | Description |
|-------|------|-------------|
| `jsonrpc` | string | Always `"2.0"` |
| `id` | number | Matches your request ID |
| `result` | any | Successful result (omitted on error) |
| `error` | object | Error details (omitted on success) |

**Success:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1"
}
```

**Error:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params"
  }
}
```

---

## Caching Behavior

BlockMesh caches certain methods to reduce upstream load and improve latency:

| Method | Cache TTL | Typical Latency |
|--------|-----------|-----------------|
| `eth_chainId` | 24 hours | < 5ms (cache hit) |
| `eth_blockNumber` | 2 seconds | < 5ms (cache hit) |
| `eth_getBalance` | 30 seconds | < 5ms (cache hit) |
| `eth_gasPrice` | 15 seconds | < 5ms (cache hit) |
| `eth_maxPriorityFeePerGas` | 15 seconds | < 5ms (cache hit) |
| Other methods | No cache | ~50–200ms |

Cache hits include the header `X-Cache: HIT`. Cache misses include `X-Cache: MISS`.

---

## Checking Your Usage

Contact your administrator to view your daily usage. They can query:

```bash
curl "http://localhost:8081/tenants/<tenant-id>/usage?day=2026-08-19" \
  -H "X-Admin-Secret: <admin-secret>"
```

---

## SDK Examples

### JavaScript / TypeScript

```typescript
const API_KEY = 'bm_live_...'
const GATEWAY = 'http://localhost:8080/v1/'

async function callRPC(method: string, params: any[] = []) {
  const res = await fetch(GATEWAY, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      jsonrpc: '2.0',
      method,
      params,
      id: Date.now(),
    }),
  })
  return res.json()
}

// Usage
const balance = await callRPC('eth_getBalance', [
  '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
  'latest',
])
console.log(balance.result)
```

### Python

```python
import requests

API_KEY = 'bm_live_...'
GATEWAY = 'http://localhost:8080/v1/'

def call_rpc(method, params=None):
    if params is None:
        params = []
    res = requests.post(GATEWAY, headers={
        'Authorization': f'Bearer {API_KEY}',
        'Content-Type': 'application/json',
    }, json={
        'jsonrpc': '2.0',
        'method': method,
        'params': params,
        'id': 1,
    })
    return res.json()

# Usage
balance = call_rpc('eth_getBalance', [
    '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
    'latest',
])
print(balance['result'])
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    reqBody, _ := json.Marshal(map[string]any{
        "jsonrpc": "2.0",
        "method":  "eth_getBalance",
        "params":  []string{"0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb", "latest"},
        "id":      1,
    })

    req, _ := http.NewRequest("POST", "http://localhost:8080/v1/", bytes.NewReader(reqBody))
    req.Header.Set("Authorization", "Bearer bm_live_...")
    req.Header.Set("Content-Type", "application/json")

    resp, _ := http.DefaultClient.Do(req)
    defer resp.Body.Close()

    var result map[string]any
    json.NewDecoder(resp.Body).Decode(&result)
    fmt.Println(result)
}
```

---

## Next Steps

- **[Authentication](authentication.md)** — How API keys work and best practices
- **[Rate Limits](rate-limits.md)** — Understanding quotas and handling 429s
- **[Errors](errors.md)** — Complete error reference and troubleshooting
