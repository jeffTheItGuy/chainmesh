# Security Guide

This document covers BlockMesh's security model, hardening recommendations, and threat mitigations.

---

## Table of Contents

1. [Threat Model](#threat-model)
2. [Authentication](#authentication)
3. [API Key Security](#api-key-security)
4. [Rate Limiting & DoS Protection](#rate-limiting--dos-protection)
5. [Network Security](#network-security)
6. [Database Security](#database-security)
7. [Redis Security](#redis-security)
8. [Audit Logging](#audit-logging)
9. [Operational Security](#operational-security)
10. [Security Checklist](#security-checklist)

---

## Threat Model

### What BlockMesh Protects Against

| Threat | Mitigation |
|--------|-----------|
| **Unauthorized API access** | Bearer token auth, bcrypt key hashing, key rotation |
| **Timing attacks on secrets** | Constant-time comparison for admin secret |
| **Rate limit bypass** | Redis-backed counters with atomic INCR+EXPIRE (Lua script) |
| **Credential exposure in logs** | API keys never logged; endpoint URLs redacted in health responses |
| **Upstream node enumeration** | URL paths hidden in `/health/nodes`; only host shown |
| **Database credential leaks** | Connection strings via env vars, not config files |
| **Replay attacks** | Request IDs for tracing; no session tokens for API keys |
| **DoS via large payloads** | 2MB body limit on gateway; 1MB on admin API |
| **SSRF via malicious RPC URLs** | Endpoint validation blocks loopback, private, and link-local IPs |

### What BlockMesh Does NOT Protect Against (Out of Scope)

| Threat | Responsibility |
|--------|---------------|
| **Compromised host / container escape** | Infrastructure provider (you) |
| **Man-in-the-middle on upstream RPC** | Use HTTPS endpoints; verify certificates |
| **DDoS at network layer** | Use Cloudflare, AWS Shield, or similar |
| **Postgres/Redis compromise** | Network segmentation, strong passwords |
| **Supply chain attacks** | Verify image signatures, pin versions |

---

## Authentication

### Admin Authentication

**Mechanism:** `X-Admin-Secret` header compared via `subtle.ConstantTimeCompare`

**Why constant-time:** Standard string comparison short-circuits on first mismatch, leaking timing information that enables byte-by-byte secret recovery. `ConstantTimeCompare` always takes the same time regardless of match position.

**Code reference:**
```go
// backend/admin/main.go
func adminAuth(secret string, db *postgres.DB, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        provided := r.Header.Get("X-Admin-Secret")
        if !util.ConstantTimeEqual(provided, secret) {
            // Failed attempts are logged to audit_logs
            writeErr(w, http.StatusForbidden, "forbidden")
            return
        }
        next(w, r)
    }
}
```

**Requirements:**
- Minimum 32 characters, random generation
- Stored in secret manager, never in git
- Admin API refuses to start if unset
- Failed attempts are recorded in the `audit_logs` table with IP and User-Agent

### Tenant Authentication

**Mechanism:** `Authorization: Bearer <api-key>`

**Flow:**
1. Extract key from header
2. Extract the 12-character prefix
3. Query `api_keys` for rows matching that prefix with `revoked_at IS NULL`
4. For each candidate, verify the full key against the stored bcrypt hash
5. On match: update `last_used_at = NOW()`
6. Return tenant context

**Code reference:**
```go
// backend/shared/storage/postgres/tenant_store.go
prefix := util.APIKeyPrefix(key)
rows, err := d.pool.Query(ctx, `
    SELECT id, key_hash, tenant_id
    FROM api_keys
    WHERE key_prefix = $1
      AND revoked_at IS NULL
`, prefix)
// ... for each row, verify with bcrypt ...
```

---

## API Key Security

### Key Format

```
bm_live_<32-character-hex>
```

- Prefix: `bm_live_` — identifies key type and environment
- Entropy: 128 bits (16 random bytes, hex-encoded)
- Example: `bm_live_4f8a2c1d9e3b7a60f8e2d4b6c9a1e5f7`

### Storage

| What | Where | Notes |
|------|-------|-------|
| Full key | **Nowhere** | Never stored |
| bcrypt hash | `api_keys.key_hash` | Used for verification |
| 12-char prefix | `api_keys.key_prefix` | Display only (e.g., "bm_live_4f8a") |
| Revocation status | `api_keys.revoked_at` | NULL = active, timestamp = revoked |

### Key Rotation

**When to rotate:**
- Suspected leak
- Employee departure
- Scheduled rotation policy (e.g., quarterly)

**Rotation process:**
1. Generate new key
2. Insert new `api_keys` row with `name = "rotated"`
3. Set `revoked_at = NOW()` on all previous active keys for tenant
4. Return new key to admin (shown once)

**Code reference:**
```go
// RotateAPIKey in tenant_store.go
tx.Exec(ctx, `UPDATE api_keys SET revoked_at = NOW() WHERE tenant_id = $1 AND revoked_at IS NULL`, tenantID)
tx.Exec(ctx, `INSERT INTO api_keys (...) VALUES (...)`, tenantID, "rotated", newHash, newPrefix)
```

**Important:** Old keys stop working immediately. There is no grace period.

---

## Rate Limiting & DoS Protection

### Limits Enforced

| Limit | Window | Redis Key Pattern | TTL |
|-------|--------|-------------------|-----|
| RPS | Per second | `rl:rps:<tenant>:<unix>` | 2s |
| RPM | Per minute | `rl:min:<tenant>:<YYYY-MM-DDTHH:MM>` | 120s |
| Daily | Per day | `rl:day:<tenant>:<YYYY-MM-DD>` | 90,000s (25h) |

### Atomic Counter (Race Condition Protection)

```lua
-- atomicIncrExpireScript
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return current
```

Without this, two concurrent requests could both see `current=0`, both increment to 1, but only one sets EXPIRE — leaving a key that never expires.

### Fail-Closed Behavior

If Redis is unavailable:
- Rate limiter returns `503 Service Unavailable`
- Gateway rejects the request
- Prometheus counter `RateLimitErrorsTotal` increments

**Why fail closed:** Allowing requests when rate limiting is broken creates a bypass vulnerability.

---

## Network Security

### Endpoint Redaction

RPC endpoint URLs in health responses show only the host, hiding API keys in paths:

```go
// backend/gateway/main.go
func redactURL(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return "redacted-endpoint"
    }
    return u.Host + "/***"
}
```

**Example:**
```json
{
  "url": "eth-mainnet.g.alchemy.com/**",
  "healthy": true,
  "latency_ms": 82
}
```

### SSRF Protection

All RPC endpoints added via the Admin API are validated before storage:

- Only `http` and `https` schemes are allowed
- Hostnames are resolved and all returned IPs are checked
- Loopback, private, link-local, multicast, and unspecified IPs are rejected

This prevents administrators from accidentally (or maliciously) configuring endpoints that target internal services.

### User-Agent Header

All upstream requests include `User-Agent: BlockMesh-Gateway/1.0` to prevent WAF/Cloudflare blocks on public RPC endpoints.

### TLS / HTTPS

| Layer | Recommendation |
|-------|---------------|
| Client → BlockMesh | Required in production (Traefik/Let's Encrypt or cert-manager) |
| BlockMesh → Upstream RPC | Always use HTTPS endpoints |
| Internal services | Use Docker/K8s network isolation; TLS optional within cluster |

### Port Exposure

**Docker Compose (production):**
```yaml
# Only expose Traefik publicly
traefik:
  ports:
    - "80:80"
    - "443:443"
# Do NOT expose these:
# gateway:
#   ports:
#     - "8080:8080"  # Security risk
# admin:
#   ports:
#     - "8081:8081"  # Security risk
```

**Kubernetes:**
- Gateway/Admin: ClusterIP only, exposed via Ingress
- Postgres/Redis: ClusterIP, no external exposure

---

## Database Security

### Connection Security

| Setting | Development | Production |
|---------|-------------|------------|
| SSL mode | `disable` or `prefer` | `require` or `verify-full` |
| Password | Simple | 64+ character random |
| User privileges | Owner | Limited to application tables |

### Postgres Hardening

```sql
-- Create dedicated application user (not superuser)
CREATE USER blockmesh_app WITH PASSWORD '...';
GRANT CONNECT ON DATABASE blockmesh TO blockmesh_app;
GRANT USAGE ON SCHEMA public TO blockmesh_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO blockmesh_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO blockmesh_app;
```

### Sensitive Data in Database

| Data | Sensitivity | Protection |
|------|------------|------------|
| `api_keys.key_hash` | High | bcrypt, irreversible |
| `api_keys.key_prefix` | Low | First 12 chars only |
| `tenants.name` | Low | Business data |
| `request_logs` | Medium | Contains method/status, not payloads |
| `blockchain_configs.rpc_endpoint_*` | High | Provider API keys in URLs |
| `audit_logs` | Medium | Immutable record of admin actions |

**Note:** `blockchain_configs` stores full RPC URLs including provider API keys. Access control on the admin API is critical.

---

## Redis Security

### Authentication

Enable Redis AUTH in production:

```conf
# redis.conf
requirepass your-strong-redis-password
```

Update `REDIS_ADDR` to include auth:
```bash
REDIS_ADDR=redis:6379  # If using Docker network, auth may be optional
# Or with explicit auth:
REDIS_ADDR=blockmesh:password@redis:6379
```

### Network Isolation

```yaml
# docker-compose.yml
services:
  redis:
    networks:
      - internal
    # No ports exposed externally

networks:
  internal:
    internal: true  # No external access
```

### Memory Limits

```conf
# redis.conf
maxmemory 256mb
maxmemory-policy allkeys-lru
```

Prevents Redis from consuming all host memory under cache pressure.

---

## Audit Logging

BlockMesh records an immutable audit trail of all admin actions and authentication events to the `audit_logs` table.

### What Is Logged

| Event | Action | Details Captured |
|-------|--------|-----------------|
| Admin auth failure | `ADMIN_AUTH_FAILURE` | IP, User-Agent, reason |
| Tenant created | `CREATE_TENANT` | Name, plan, quotas, network ID |
| Tenant updated | `UPDATE_TENANT` | Changed fields |
| Tenant deleted | `DELETE_TENANT` | Tenant ID |
| API key rotated | `ROTATE_API_KEY` | Tenant ID |
| Network created | `CREATE_BLOCKCHAIN_CONFIG` | Name, chain ID |
| Network updated | `UPDATE_BLOCKCHAIN_CONFIG` | Changed fields |
| Network deleted | `DELETE_BLOCKCHAIN_CONFIG` | Network ID |

### Querying Audit Logs

Administrators can query audit logs via the Admin API:

```bash
curl "http://localhost:8081/audit-logs?limit=50&offset=0" \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

### Design

- Writes are fire-and-forget (3-second timeout) so they never block the admin API response
- Records include client IP and User-Agent for forensics
- The table is append-only with no update or delete paths in the application

---

## Operational Security

### Secret Management

| Secret | Storage | Rotation |
|--------|---------|----------|
| `ADMIN_SECRET` | Secret manager (K8s secrets, Vault, 1Password) | On suspected compromise or quarterly |
| `POSTGRES_PASSWORD` | Same as above | On suspected compromise or annually |
| API keys | Database (bcrypt hashed) | On leak or per tenant policy |
| Provider RPC URLs | Database (`blockchain_configs`) | On provider key rotation |

### Log Sanitization

BlockMesh does not log:
- Full API keys (only prefixes in error contexts)
- Admin secrets
- Request bodies (only method name and size)

**What IS logged:**
- Request ID, tenant ID, method, status, latency
- Endpoint host (not full URL)
- Error messages (without sensitive data)

### Dashboard Security

- Credentials stored in `sessionStorage` (cleared on tab close)
- No persistent cookies
- Viewer mode requires no credentials (read-only blocks/health)
- Admin mode requires `ADMIN_SECRET` on every API call

### Container Security

```dockerfile
# Use non-root user
FROM golang:1.22-alpine AS builder
# ... build ...
FROM gcr.io/distroless/static-debian12
USER nonroot:nonroot
COPY --from=builder /app/gateway /gateway
ENTRYPOINT ["/gateway"]
```

Current images should run as non-root. Verify:
```bash
docker run --rm blockmesh-gateway:latest id
# Expected: uid=65532(nonroot) gid=65532(nonroot)
```

---

## Security Checklist

Before going to production, verify:

- [ ] `ADMIN_SECRET` is 32+ random characters, stored in secret manager
- [ ] `POSTGRES_PASSWORD` is strong and unique
- [ ] Postgres uses SSL (`sslmode=require` or better)
- [ ] Redis uses AUTH and network isolation
- [ ] Only Traefik/Nginx ports (80/443) are exposed publicly
- [ ] Gateway (8080) and Admin (8081) are NOT exposed directly
- [ ] `.env` and `secrets.yaml` are in `.gitignore`
- [ ] Automated backups are configured and tested
- [ ] Log rotation is configured
- [ ] Container images run as non-root
- [ ] API keys are rotated from default/demo keys
- [ ] Rate limits are configured for all tenants
- [ ] Upstream RPC endpoints use HTTPS
- [ ] Domain uses valid TLS certificate (Let's Encrypt or custom)
- [ ] Security headers are set (CSP, HSTS, X-Frame-Options)
- [ ] Prometheus metrics endpoint is not publicly exposed (or is authenticated)
- [ ] Audit logging is enabled and monitored

---

## Reporting Security Issues

If you discover a security vulnerability:

1. **Do not open a public issue**
2. Email `security@yourdomain.com` with:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)
3. Allow 90 days for disclosure after fix

---

## Related Documents

- [Configure](configure.md) — Environment variable reference
- [Deploy](deploy.md) — Deployment procedures
- [Architecture Overview](../architecture/overview.md) — Component security boundaries
- [Data Flow](../architecture/data-flow.md) — Authentication and rate limiting in detail
