# Adding Tenants

Tenants are the clients of your gateway. Each tenant gets an API key, a rate-limit
quota, and (optionally) a pinned blockchain network. All tenant traffic flows
through the gateway's `/v1/` endpoint, authenticated with the tenant's API key.

---

## Prerequisites

- The admin service running (default: `http://localhost:8081`, or `http://localhost/api/` behind nginx)
- Your `ADMIN_SECRET` exported:

```bash
export ADMIN_SECRET=your-admin-secret
```
- **At least one blockchain network configured** — a tenant must route somewhere.
  If you omit the network when creating a tenant, it uses the default network
  (the earliest enabled one). See *Adding Blockchain Networks* if you haven't set one up.

---

## Option 1 — Admin Console (recommended)

1. Sign in to the console with the admin secret.
2. Open **Access → Tenants**.
3. Click **New tenant**.
4. Fill in the form:
   - **Name** — display name, e.g. `Acme Corp`
   - **Plan** — `free`, `basic`, `pro`, or `enterprise` (informational label)
   - **Quota (req/sec)** — per-second rate limit
   - **Quota (req/min)** — per-minute rate limit
   - **Quota (daily)** — daily request cap
   - **Blockchain Network** — which network this tenant routes to (or `Default`)
5. Click **Create tenant**.
6. **Copy the API key immediately.** It is shown only once and stored only as a
   SHA-256 hash — it cannot be retrieved later.

---

## Option 2 — Admin API

### Create a tenant

```bash
curl -X POST http://localhost:8081/tenants \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "plan": "pro",
    "quota_rps": 50,
    "quota_rpm": 1000,
    "quota_daily": 100000,
    "blockchain_network_id": "<NETWORK_ID>"
  }'
```

The response includes the plaintext API key **exactly once**:

```json
{
  "id": "9f2c8a92-4b7e-4d21-9c3a-2e8d5f0b1a77",
  "name": "Acme Corp",
  "api_key": "bm_live_4f8a2c1d9e3b7a60",
  "quota_rps": 50,
  "quota_rpm": 1000,
  "quota_daily": 100000,
  "plan": "pro",
  "blockchain_network_id": "<NETWORK_ID>",
  "created_at": "2026-08-19T12:00:00Z"
}
```

> Store `api_key` securely now. The database only keeps a SHA-256 hash, so it
> can never be shown again — the only way to recover access is to rotate the key.

### Field reference

| Field                   | Type   | Required | Default                       | Notes |
| ----------------------- | ------ | -------- | ----------------------------- | ----- |
| `name`                  | string | yes      | —                             | Display name |
| `plan`                  | string | no       | `free`                        | Label only; quotas come from the `quota_*` fields |
| `quota_rps`             | int    | no       | `10`                          | Requests per second (`0` disables this limit) |
| `quota_rpm`             | int    | no       | `100`                         | Requests per minute |
| `quota_daily`           | int    | no       | `10000`                       | Requests per day |
| `blockchain_network_id` | uuid   | no       | default network               | Pin to a specific network; empty = default |

### Recommended quota presets

| Plan       | `quota_rps` | `quota_rpm` | `quota_daily` |
| ---------- | ----------- | ----------- | ------------- |
| `free`     | 10          | 100         | 10,000        |
| `basic`    | 25          | 500         | 50,000        |
| `pro`      | 50          | 1,000       | 100,000       |
| `enterprise` | 200       | 5,000       | 1,000,000     |

---

## Using a tenant's API key

All gateway requests go to `/v1/` with a `Bearer` token:

```bash
curl -s http://localhost:8080/v1/ \
  -H "Authorization: Bearer bm_live_4f8a2c1d9e3b7a60" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
```

```json
{ "jsonrpc": "2.0", "id": 1, "result": "0x12a05f2" }
```

Rate-limit state is exposed via response headers:

| Header | Meaning |
| ------ | ------- |
| `X-RateLimit-Limit-Minute`     | Per-minute quota |
| `X-RateLimit-Remaining-Minute` | Requests left in the current minute |
| `X-RateLimit-Reset`            | Unix time when the minute window resets |
| `Retry-After`                  | Seconds to wait when throttled (`429`) |

---

## Managing tenants

### Edit a tenant

```bash
curl -X PUT http://localhost:8081/tenants/<TENANT_ID> \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "plan": "enterprise",
    "quota_rps": 200,
    "quota_rpm": 5000,
    "quota_daily": 1000000,
    "blockchain_network_id": "<NETWORK_ID>"
  }'
```

Omit any field to keep its current value. Quota changes take effect immediately —
no key rotation or restart required.

### Rotate a tenant's API key

Use this if a key is leaked or on a schedule. The old key is revoked immediately.

```bash
curl -X POST http://localhost:8081/tenants/<TENANT_ID>/rotate-key \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

```json
{ "api_key": "bm_live_7c1e9d4a2b8f3056" }
```

### Delete a tenant

```bash
curl -X DELETE http://localhost:8081/tenants/<TENANT_ID> \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

Deleting a tenant cascades: its API keys, usage records, and request logs are
removed automatically.

---

## Viewing usage

```bash
# Today
curl "http://localhost:8081/tenants/<TENANT_ID>/usage" \
  -H "X-Admin-Secret: $ADMIN_SECRET"

# A specific day
curl "http://localhost:8081/tenants/<TENANT_ID>/usage?day=2026-08-18" \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

Returns request volume broken down by JSON-RPC method:

```json
[
  { "tenant_id": "9f2c…", "method": "eth_blockNumber", "count": 4120, "bytes_in": 189520, "period": "2026-08-18T00:00:00Z" },
  { "tenant_id": "9f2c…", "method": "eth_getBalance",  "count": 930,  "bytes_in": 44640,  "period": "2026-08-18T00:00:00Z" }
]
```

---

## How tenants are enforced

| Component | Behavior |
| --------- | -------- |
| Gateway auth  | Rejects requests without a valid `Bearer` API key (`401`) |
| Rate limiter  | Checks per-second, per-minute, and daily limits in Redis before proxying |
| Rate limiter  | Returns `429` with `Retry-After` when any limit is exceeded |
| Proxy         | Routes the request to the tenant's pinned network (or the default) |
| Telemetry     | Records per-method usage and request logs for the dashboard |

A quota of `0` disables that specific limit (e.g. `quota_daily: 0` = no daily cap).

---

## Security notes

- **Keys are never stored in plaintext.** Only a SHA-256 hash and a short visible
  prefix are kept in the `api_keys` table.
- **Keys are shown once** — at creation and at rotation. There is no "view key"
  action; rotate instead.
- **Rotate immediately on suspected leak.** The old key stops working the moment
  rotation completes.
- The `api_key` field on the tenant object is deliberately hidden (`json:"-"`) in
  all list/get responses.

---

## Troubleshooting

| Symptom | Cause / Fix |
| ------- | ----------- |
| Tenant creation fails: `no blockchain network available — configure one first` | No enabled networks exist. Add one first, or pass a valid `blockchain_network_id`. |
| Gateway returns `401 unauthorized` | Invalid or revoked API key. Check the key, or rotate it if it was revoked. |
| Gateway returns `429 rate limit exceeded` | A quota was hit. Check `Retry-After`; raise the tenant's quota if appropriate. |
| `429` with `rate limiter unavailable` | Redis is down. The limiter fails closed — restore Redis. |
| Can't remember a tenant's API key | Keys can't be retrieved. Use `rotate-key` to issue a new one. |
| Usage shows nothing | No requests recorded for that day, or the wrong `day` format (must be `YYYY-MM-DD`). |