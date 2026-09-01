# Errors

Complete reference for all HTTP status codes and JSON-RPC error responses you may encounter when using the BlockMesh API.

---

## HTTP Status Codes

### 2xx Success

| Status | Meaning |
|--------|---------|
| `200 OK` | Request successful. Note: JSON-RPC errors are also returned as HTTP 200 with an `error` field in the body. |

### 4xx Client Errors

| Status | Cause | Response Body | Resolution |
|--------|-------|---------------|------------|
| `400 Bad Request` | Invalid JSON or malformed JSON-RPC request | `{"error":"invalid json"}` or `{"error":"invalid json-rpc request"}` | Check JSON syntax; ensure `jsonrpc: "2.0"` and `method` are present |
| `401 Unauthorized` | Missing or invalid API key | `{"error":"unauthorized"}` | Verify `Authorization: Bearer <key>` header; check key is not revoked |
| `413 Payload Too Large` | Request body exceeds 2MB | `{"error":"request body too large"}` | Reduce payload size or split into smaller requests |
| `429 Too Many Requests` | Rate limit exceeded | `{"error":"rate limit exceeded"}` | Wait for `Retry-After` seconds; implement backoff |

### 5xx Server Errors

| Status | Cause | Response Body | Resolution |
|--------|-------|---------------|------------|
| `502 Bad Gateway` | All upstream RPC nodes failed | `{"error":"upstream unavailable"}` | Check `/health/nodes` (admin only); verify upstream endpoints |
| `503 Service Unavailable` | No blockchain network configured | `{"error":"no blockchain network configured"}` | Admin must add a network via Dashboard or Admin API |
| `503 Service Unavailable` | Blockchain network unavailable | `{"error":"blockchain network unavailable"}` | Network may be disabled or all nodes unhealthy |
| `503 Service Unavailable` | Redis unavailable (rate limiter) | `{"error":"rate limiter unavailable"}` | Wait and retry; contact administrator if persistent |

---

## JSON-RPC Error Responses

When an upstream node returns a JSON-RPC error, BlockMesh passes it through as HTTP 200:

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

### Common JSON-RPC Error Codes

| Code | Message | Meaning |
|------|---------|---------|
| `-32700` | Parse error | Invalid JSON was received |
| `-32600` | Invalid Request | JSON is valid but not a valid Request object |
| `-32601` | Method not found | Method does not exist |
| `-32602` | Invalid params | Invalid method parameters |
| `-32603` | Internal error | Internal JSON-RPC error |
| `-32000` to `-32099` | Server error | Implementation-defined errors |

---

## Error Propagation Flow

```
Client Request
     │
     ▼
┌─────────────┐
│  Gateway    │
│  Middleware │
└──────┬──────┘
       │
   ┌───┴───┐
   ▼       ▼
 401      429    ← Auth / Rate limit failure
   │       │
   ▼       ▼
┌─────────────────┐
│   Proxy Handler │
└───────┬─────────┘
        │
   ┌────┴────┬────────┐
   ▼         ▼        ▼
 400       413      503   ← Invalid request / Too large / No network
   │         │        │
   ▼         ▼        ▼
┌─────────────────────────┐
│   Upstream RPC Call     │
└───────────┬─────────────┘
            │
       ┌────┴────┐
       ▼         ▼
     200       502
   (success)  (all nodes failed)
       │
       ▼
   RPC error in body
   (code, message)
```

---

## Troubleshooting Guide

### "unauthorized" (401)

1. Check the `Authorization` header is present: `Authorization: Bearer <key>`
2. Verify the `Bearer` prefix is included
3. Confirm the key has not been revoked (contact administrator)
4. Ensure no extra whitespace around the key

### "rate limit exceeded" (429)

1. Check `X-RateLimit-Remaining-Minute` — are you at 0?
2. Wait for `Retry-After` seconds before retrying
3. Implement exponential backoff with jitter
4. Consider caching `eth_chainId` and `eth_blockNumber` locally
5. Contact administrator to request higher quotas

### "upstream unavailable" (502)

1. This means all configured RPC endpoints failed
2. If you have admin access, check `/health/nodes`:
   ```bash
   curl http://localhost:8080/health/nodes
   ```
3. Verify your network's endpoints are healthy
4. Check if upstream provider has status page issues
5. Contact administrator to verify network configuration

### "no blockchain network configured" (503)

1. The gateway has no active networks
2. Administrator must add at least one network via:
   - Dashboard: Infrastructure → Blockchain Networks → Add network
   - Admin API: `POST /blockchain`

### "blockchain network unavailable" (503)

1. The tenant's assigned network is disabled or has no healthy nodes
2. Administrator should check network status in Dashboard
3. May resolve automatically when nodes recover (health checks run every 10s)

### "rate limiter unavailable" (503)

1. Redis is unreachable from the gateway
2. This is temporary — retry with exponential backoff
3. If persistent, contact administrator

---

## Request IDs for Debugging

Every response includes an `X-Request-ID` header. Include this when reporting issues:

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <key>" \
  -H "Content-Type: application/json" \
  -d '...' \
  -v 2>&1 | grep X-Request-ID
```

Administrators can trace this ID through gateway logs and request_logs table.

---

## Related Documents

- [Quick Start](quickstart.md) — Making your first API call
- [Authentication](authentication.md) — API key usage and security
- [Rate Limits](rate-limits.md) — Quota handling and backoff strategies
