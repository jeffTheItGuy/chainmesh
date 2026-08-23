# ChainMesh Documentation

Welcome to the ChainMesh documentation. This is your starting point for understanding, deploying, and operating the ChainMesh blockchain API gateway.

---

## What is ChainMesh?

ChainMesh is a production-grade, self-hosted API gateway that sits between your applications and blockchain RPC nodes. It provides intelligent routing, Redis caching, per-tenant rate limiting, usage metering, and a role-based web dashboard.

**Key capabilities:**
- **Health-aware smart routing** — automatic failover between multiple RPC endpoints
- **Multi-network support** — manage Ethereum, Sepolia, Polygon, and other networks
- **Secure multi-tenancy** — bcrypt hashed API keys, key rotation, constant-time auth
- **Domain-aware caching** — method-specific TTLs (e.g., `eth_chainId` for 24h)
- **Granular rate limiting** — per-tenant RPS, RPM, and daily quotas via Redis
- **Async telemetry** — non-blocking usage recording with pre-aggregated stats views
- **Audit logging** — admin actions and authentication failures recorded to PostgreSQL

---

## Documentation Structure

### For Operators (Running ChainMesh)
| Document | Description |
|----------|-------------|
| [Architecture Overview](architecture/overview.md) | System components, tech stack, and design principles |
| [Data Flow](architecture/data-flow.md) | Request lifecycle, config reload, and health-check mechanics |
| [Database Schema](architecture/schema.md) | Entity relationships, migrations, and query patterns |
| [How to Deploy](../docs/HowToDeploy.md) | Docker Compose or Kubernetes deployment guides |
| [How to Add Networks](../docs/HowToAddNetworks.md) | Configure blockchain networks via UI or API |
| [How to Add Tenants](../docs/HowToAddTenants.md) | Create and manage API tenants |

### For API Consumers (Using ChainMesh)
| Document | Description |
|----------|-------------|
| [How to Use](../docs/HowToUse.md) | Get an API key, make RPC calls, check usage |

### For Administrators (Managing ChainMesh)
| Document | Description |
|----------|-------------|
| [How to Add Networks](../docs/HowToAddNetworks.md) | Configure and test blockchain endpoints |
| [How to Add Tenants](../docs/HowToAddTenants.md) | Tenant lifecycle, quotas, and key rotation |

---

## Quick Navigation

**I want to understand how it works**
→ Start with [Architecture Overview](architecture/overview.md)

**I want to see how a request flows through the system**
→ Read [Data Flow](architecture/data-flow.md)

**I want to understand the database**
→ Check [Database Schema](architecture/schema.md)

**I want to deploy it**
→ Follow [How to Deploy](../docs/HowToDeploy.md)

**I want to use the API**
→ See [How to Use](../docs/HowToUse.md)

---

## API Reference

The ChainMesh Admin API is documented via OpenAPI 3.0. See [`backend/api/openapi.yaml`](../../backend/api/openapi.yaml) for the raw specification.

---

## Support & Contributing

- Found a bug? Open an issue with reproduction steps
- Want to contribute? See our development setup in the main README
- Questions? Check the troubleshooting sections in each guide

---

*Last updated: 2026-08-19*
