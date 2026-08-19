# Managing Tenants

Complete guide for creating, updating, and managing API tenants in BlockMesh.

---

## What Is a Tenant?

A tenant represents a customer, application, or team that consumes the BlockMesh API. Each tenant gets:

- A unique API key for authentication
- Per-tenant rate limits (RPS, RPM, daily)
- Optional assignment to a specific blockchain network
- Usage tracking and metering

---

## Creating a Tenant

### Via Web Dashboard

1. Sign in to the dashboard as admin
2. Navigate to **Access → Tenants**
3. Click **New tenant**
4. Fill in the form:
   - **Name** — Display name (required)
   - **Plan** — Tier label: `free`, `basic`, `pro`, `enterprise`
   - **Quota (req/sec)** — RPS limit (0 = unlimited)
   - **Quota (req/min)** — RPM limit
   - **Quota (daily)** — Daily request cap
   - **Blockchain Network** — Specific network or "Default (auto-assign)"
5. Click **Create tenant**
6. **Copy the API key immediately** — it is shown only once

### Via Admin API

```bash
curl -X POST http://localhost:8081/tenants \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "quota_rps": 10,
    "quota_rpm": 1000,
    "quota_daily": 100000,
    "plan": "pro",
    "blockchain_network_id": "<network-uuid>"
  }'
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Acme Corp",
  "api_key": "bm_live_4f8a2c1d9e3b7a60f8e2d4b6c9a1e5f7",
  "quota_rpm": 1000,
  "quota_rps": 10,
  "quota_daily": 100000,
  "plan": "pro",
  "blockchain_network_id": "...",
  "created_at": "2026-08-19T14:30:00Z"
}
```

**Important:** The `api_key` is returned **only at creation**. Store it securely — it cannot be retrieved later.

---

## Default Quotas

If omitted, these defaults apply:

| Field | Default |
|-------|---------|
| `quota_rpm` | 100 |
| `quota_rps` | 10 |
| `quota_daily` | 10000 |
| `plan` | `free` |
| `blockchain_network_id` | Earliest enabled network |

---

## Listing Tenants

### Dashboard

The Tenants page shows all tenants with their plan, quota, assigned network, and creation date.

### API

```bash
curl http://localhost:8081/tenants \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

**Response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corp",
    "quota_rpm": 1000,
    "quota_rps": 10,
    "quota_daily": 100000,
    "plan": "pro",
    "blockchain_network_id": "...",
    "created_at": "2026-08-19T14:30:00Z"
  }
]
```

---

## Updating a Tenant

### Dashboard

1. Find the tenant in the Tenants table
2. Click **Edit**
3. Modify fields as needed
4. Click **Update tenant**

### API

```bash
curl -X PUT http://localhost:8081/tenants/<tenant-id> \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp Updated",
    "quota_rpm": 2000,
    "quota_rps": 20,
    "quota_daily": 200000,
    "plan": "enterprise",
    "blockchain_network_id": "<new-network-uuid>"
  }'
```

**Note:** Omit fields you don't want to change. The existing value is preserved.

---

## Deleting a Tenant

**Warning:** Deletion is permanent and cascades to all associated data (API keys, usage records, request logs).

### Dashboard

1. Find the tenant in the Tenants table
2. Click **Delete**
3. Confirm the deletion

### API

```bash
curl -X DELETE http://localhost:8081/tenants/<tenant-id> \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

**Response:**
```json
{ "deleted": true }
```

---

## Rotating API Keys

If a key is compromised or you need to rotate credentials:

### Dashboard

1. Find the tenant in the Tenants table
2. Click **Rotate**
3. Confirm the rotation
4. **Copy the new key immediately** — shown only once

### API

```bash
curl -X POST http://localhost:8081/tenants/<tenant-id>/rotate-key \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

**Response:**
```json
{ "api_key": "bm_live_a1b2c3d4e5f6..." }
```

**Behavior:**
- All previous active keys for this tenant are revoked immediately
- The new key is shown exactly once
- There is **no grace period** — old keys stop working instantly

---

## Viewing Tenant Usage

### Dashboard

1. Navigate to **Metering → Usage lookup**
2. Select a tenant from the dropdown
3. Optionally pick a specific date (defaults to today)
4. Click **Look up**

### API

```bash
# Today's usage
curl http://localhost:8081/tenants/<tenant-id>/usage \
  -H "X-Admin-Secret: $ADMIN_SECRET"

# Specific day
curl "http://localhost:8081/tenants/<tenant-id>/usage?day=2026-08-19" \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

**Response:**
```json
[
  {
    "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
    "method": "eth_getBalance",
    "count": 15420,
    "bytes_in": 1234567,
    "period": "2026-08-19T14:00:00Z"
  }
]
```

---

## Tenant Plans

Plans are informational labels used for billing tiers. Common setups:

| Plan | Typical Quotas | Use Case |
|------|---------------|----------|
| `free` | 10 RPS / 100 RPM / 10K daily | Development, testing |
| `basic` | 25 RPS / 500 RPM / 50K daily | Small applications |
| `pro` | 50 RPS / 2K RPM / 200K daily | Production workloads |
| `enterprise` | Custom | High-volume, dedicated |

Plans have **no built-in behavioral difference** — they are labels for your billing integration.

---

## Assigning Networks

Tenants can be pinned to a specific blockchain network:

1. Create or edit a tenant
2. Select a network from the **Blockchain Network** dropdown
3. Save

If no network is selected, the tenant uses the **default network** (earliest enabled network by creation date).

To change a tenant's network, update `blockchain_network_id` via the API or Dashboard.

---

## Best Practices

- **Use descriptive names** — Include company or app name
- **Set appropriate quotas** — Start conservative, increase as needed
- **Rotate keys quarterly** — Or immediately if compromised
- **Monitor usage** — Check for unexpected spikes that may indicate abuse
- **Assign networks intentionally** — Don't rely on defaults in production
- **Clean up inactive tenants** — Delete tenants that are no longer needed

---

## Related Documents

- [Adding Networks](add-networks.md) — Configure blockchain networks
- [Monitoring](monitoring.md) — Dashboard stats and observability
- [Configuration](../operators/configure.md) — Environment variables
