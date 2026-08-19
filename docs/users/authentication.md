# Authentication

BlockMesh uses API key authentication for all gateway requests. This document explains how authentication works, how to keep your keys secure, and what to do if a key is compromised.

---

## API Key Format

```
bm_live_<32-character-hex>
```

- **Prefix:** `bm_live_` — identifies a live/production key
- **Entropy:** 128 bits (16 random bytes, hex-encoded)
- **Example:** `bm_live_4f8a2c1d9e3b7a60f8e2d4b6c9a1e5f7`

Your administrator creates keys via the Admin API or Dashboard. The full key is shown **exactly once** at creation — store it immediately.

---

## Authenticating Requests

Include your API key in the `Authorization` header using the Bearer scheme:

```
Authorization: Bearer bm_live_4f8a2c1d9e3b7a60f8e2d4b6c9a1e5f7
```

### Example

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer bm_live_..." \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "eth_chainId",
    "params": [],
    "id": 1
  }'
```

---

## How It Works

1. You send the full API key in the `Authorization` header
2. BlockMesh hashes the key using SHA-256
3. The hash is looked up in the `api_keys` database table
4. If found and not revoked, the request is authenticated
5. The `last_used_at` timestamp is updated
6. Your tenant context (quotas, network assignment) is loaded

**Important:** The full key is never stored. Only its SHA-256 hash and a 12-character prefix are kept in the database.

---

## Security Best Practices

### Do

- **Store keys in environment variables or secret managers** — never hardcode in source
- **Use HTTPS in production** — prevents key interception in transit
- **Rotate keys regularly** — especially after team changes
- **Use separate keys per environment** — development, staging, production
- **Monitor usage** — unexpected spikes may indicate a leak

### Don't

- **Commit keys to git** — use `.env` files and `.gitignore`
- **Log keys** — logging frameworks may capture headers
- **Share keys between applications** — each app should have its own key
- **Send keys over unencrypted channels** — no HTTP in production

---

## Key Rotation

If you suspect a key is compromised, contact your administrator immediately. They can:

1. Revoke the old key (stops all traffic instantly)
2. Generate a new key
3. Provide you with the new key (shown once)

There is **no grace period** — revoked keys stop working immediately.

---

## Troubleshooting Authentication Errors

### `401 Unauthorized`

| Cause | Solution |
|-------|----------|
| Missing `Authorization` header | Add `Authorization: Bearer <key>` |
| Malformed header (no `Bearer` prefix) | Use `Bearer <key>`, not just `<key>` |
| Key revoked | Contact administrator for a new key |
| Key doesn't exist | Verify you're using the correct key |

### `403 Forbidden`

This status is returned by the Admin API, not the gateway. You sent an admin request without the `X-Admin-Secret` header. API consumers do not need this header.

---

## Request IDs

BlockMesh supports request tracing via the `X-Request-ID` header. You can provide your own ID or let the gateway generate one.

```bash
curl -X POST http://localhost:8080/v1/ \
  -H "Authorization: Bearer <api-key>" \
  -H "X-Request-ID: req-12345" \
  -H "Content-Type: application/json" \
  -d '{...}'
```

The response will include the same `X-Request-ID` header, useful for correlating logs across distributed systems.

---

## Related Documents

- [Quick Start](quickstart.md) — Making your first API call
- [Rate Limits](rate-limits.md) — Quota enforcement and headers
- [Errors](errors.md) — Complete error code reference
