# Contributing to BlockMesh

Thank you for your interest in contributing to BlockMesh! This guide covers everything you need to set up your development environment, follow project conventions, and submit your first contribution.

---

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Getting Started](#getting-started)
3. [Development Environment](#development-environment)
4. [Project Structure](#project-structure)
5. [Coding Standards](#coding-standards)
6. [Branching & Commits](#branching--commits)
7. [Testing](#testing)
8. [Database Migrations](#database-migrations)
9. [Documentation](#documentation)
10. [Pull Request Process](#pull-request-process)
11. [Release Process](#release-process)
12. [Security Issues](#security-issues)

---

## Code of Conduct

By participating in this project, you agree to:

- Be respectful and constructive in all discussions
- Focus on what is best for the project and its users
- Accept constructive criticism gracefully
- Not engage in harassment, trolling, or inflammatory behavior

Violations may result removal from the project community.

---

## Getting Started

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://golang.org/dl/) | 1.22+ | Backend services |
| [Node.js](https://nodejs.org/) | 20+ | Frontend tooling |
| [PostgreSQL](https://postgresql.org/) | 15+ | Primary database |
| [Redis](https://redis.io/) | 7+ | Caching & rate limiting |
| [Docker](https://docker.com/) | 24.0+ | Containerized development |
| [Docker Compose](https://docs.docker.com/compose/) | v2+ | Multi-service orchestration |
| [Air](https://github.com/cosmtrek/air) | latest | Go hot reloading (optional) |

### Fork & Clone

```bash
# 1. Fork the repository on GitHub, then clone your fork
git clone https://github.com/<your-username>/blockmesh.git
cd blockmesh

# 2. Add upstream remote
git remote add upstream https://github.com/yourname/blockmesh.git

# 3. Verify remotes
git remote -v
```

---

## Development Environment

### Quick Start (Docker)

```bash
# Start infrastructure
docker compose up -d postgres redis

# Wait for Postgres to be ready
sleep 10

# Run migrations
docker exec -i blockmesh-postgres-1 psql -U blockmesh -d blockmesh < backend/database/migrations/001_init.up.sql
# ... apply remaining migrations in order

# Set environment
export ADMIN_SECRET=dev-secret-not-for-production
export DATABASE_URL=postgres://blockmesh:dev@postgres:5432/blockmesh?sslmode=disable
export REDIS_ADDR=redis:6379
```

### Running Services

Open separate terminals for each service:

```bash
# Terminal 1 — Gateway
cd backend/gateway && go run main.go manager.go

# Terminal 2 — Admin API
cd backend/admin && go run main.go

# Terminal 3 — Ingestor
cd backend/ingestor && go run main.go

# Terminal 4 — Frontend
cd frontend && npm install && npm run dev
```

### Hot Reloading

**Go (Air):**
```bash
cd backend/gateway
air -c .air.toml
```

**Frontend (Vite):**
```bash
cd frontend
npm run dev
```

For full setup details, see [Developer Setup Guide](setup.md).

---

## Project Structure

```
blockmesh/
├── backend/
│   ├── gateway/              # Public JSON-RPC proxy (:8080)
│   ├── admin/                # Management API (:8081)
│   ├── ingestor/             # Block indexer
│   ├── shared/               # Common packages
│   │   ├── blockchain/       # JSON-RPC client with health checks
│   │   ├── logger/           # Structured logging (slog)
│   │   ├── metrics/          # Prometheus instrumentations
│   │   ├── model/            # Data structures
│   │   ├── requestid/        # Context-scoped request IDs
│   │   ├── statsrollup/      # Materialized view refresher
│   │   ├── storage/          # Postgres & Redis clients
│   │   ├── telemetry/        # Async usage recording
│   │   └── util/             # API key generation, hashing
│   ├── database/migrations/  # Schema evolution SQL
│   └── api/openapi.yaml      # OpenAPI 3.0 specification
│
├── frontend/                 # React dashboard
│   ├── src/
│   ├── package.json
│   ├── vite.config.ts
│   └── Dockerfile
│
├── docs/                     # Documentation
└── docker-compose.yml        # Full stack orchestration
```

---

## Coding Standards

### Go

- **Formatting:** Run `gofmt -w .` before committing (or use your editor's Go format-on-save)
- **Linting:** Use `go vet ./...`
- **Naming:** Follow standard Go conventions (exported identifiers use CamelCase, unexported use camelCase)
- **Errors:** Always wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- **Logging:** Use the shared `logger` package (`log/slog` with JSON handler)
- **Context:** Always pass `context.Context` as the first parameter
- **No panics in library code:** Return errors instead

### TypeScript / React

- **Formatting:** Prettier (configured in `.prettierrc`)
  ```bash
  npm run format
  ```
- **Linting:** ESLint with TypeScript and React Hooks plugins
  ```bash
  npm run lint
  ```
- **Type checking:**
  ```bash
  npm run typecheck
  ```
- **Style:** Functional components with hooks only. No class components (except ErrorBoundary).
- **Naming:** PascalCase for components, camelCase for functions/variables, UPPER_SNAKE for constants

### SQL Migrations

- Sequential numbering: `NNN_description.sql`
- Must be backward-compatible (additive changes only)
- New columns must have defaults or be nullable
- Include both `.up.sql` and `.down.sql` where applicable

---

## Branching & Commits

### Branch Naming

| Type | Prefix | Example |
|------|--------|---------|
| Feature | `feature/` | `feature/batch-rpc-support` |
| Bug fix | `fix/` | `fix/redis-connection-leak` |
| Docs | `docs/` | `docs/upgrade-guide` |
| Chore | `chore/` | `chore/upgrade-go-1.23` |

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat` — New feature
- `fix` — Bug fix
- `docs` — Documentation only
- `refactor` — Code change that neither fixes a bug nor adds a feature
- `perf` — Performance improvement
- `test` — Adding or updating tests
- `chore` — Build process, tooling, or dependency changes

**Examples:**
```
feat(gateway): add batch JSON-RPC support

Allow arrays of JSON-RPC requests in a single POST /v1/ call.
Each request is validated and processed independently.

Closes #42
```

```
fix(redis): prevent rate limit key expiration race condition

Use atomic Lua script (INCR + conditional EXPIRE) instead of
separate INCR and EXPIRE calls.

Fixes #108
```

---

## Testing

### Before Submitting

All contributions must pass:

```bash
# Backend
cd backend
go test ./...
go vet ./...

# Frontend
cd frontend
npm run typecheck
npm run lint
npm run build
```

### Adding Tests

- Place Go test files alongside source: `*_test.go`
- Exclude test files from Air hot reload (already configured in `.air.toml`)
- Test both success and error paths
- For database-dependent tests, use Docker Compose to spin up a test Postgres

### Running Specific Tests

```bash
# Single package
go test ./backend/shared/blockchain/...

# Single test function
go test -run TestRateLimit ./backend/shared/storage/redis/...

# With race detection
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Database Migrations

When your change requires a schema modification:

### Creating a Migration

```bash
# Determine next number
ls backend/database/migrations/ | sort -V | tail -1

# Create files
touch backend/database/migrations/009_your_feature.up.sql
touch backend/database/migrations/009_your_feature.down.sql
```

### Rules

1. **Never modify an existing migration** — always create a new one
2. **Make migrations idempotent** where possible (`IF NOT EXISTS`, `IF EXISTS`)
3. **Test the down migration** — it must cleanly undo the up migration
4. **No destructive changes in minor releases** — column drops, type changes, and renames require a major version bump
5. **Add indexes concurrently** for tables that may have data in production:
   ```sql
   CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_new_index ON table_name(column);
   ```

### Example

```sql
-- 009_add_tenant_metadata.up.sql
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tenants_metadata ON tenants USING GIN(metadata);
```

```sql
-- 009_add_tenant_metadata.down.sql
DROP INDEX IF EXISTS idx_tenants_metadata;
ALTER TABLE tenants DROP COLUMN IF EXISTS metadata;
```

---

## Documentation

Documentation lives in `docs/` and is organized by audience:

| Directory | Audience |
|-----------|----------|
| `docs/architecture/` | System design and internals |
| `docs/operators/` | Deployment, config, backups |
| `docs/admins/` | Network and tenant management |
| `docs/users/` | API consumers |
| `docs/developers/` | Contributors (you are here) |

### When to Update Docs

- **New feature** → Update relevant user/admin docs + add API spec changes to `openapi.yaml`
- **New config variable** → Update `docs/operators/configure.md`
- **New migration** → Update `docs/architecture/schema.md`
- **Breaking change** → Update `docs/operators/upgrade.md`
- **New endpoint** → Update `backend/api/openapi.yaml`

### Style

- Use tables for structured reference info
- Include code examples for any API or CLI operation
- End with a "Related Documents" section linking to adjacent pages

---

## Pull Request Process

### 1. Create Your Branch

```bash
git checkout -b feature/my-new-feature upstream/main
```

### 2. Make Your Changes

- Keep changes focused — one logical change per PR
- Add tests for new functionality
- Update documentation as needed
- Ensure all checks pass locally

### 3. Commit and Push

```bash
git add .
git commit -m "feat(scope): description"
git push origin feature/my-new-feature
```

### 4. Open a Pull Request

- Target: `main` branch on upstream
- Title: Follow conventional commit format
- Description template:

```markdown
## Summary
<!-- What does this PR do? -->

## Motivation
<!-- Why is this change needed? Link to issue if applicable. -->

## Changes
<!-- Bullet list of changes -->

## Testing
<!-- How was this tested? -->

## Checklist
- [ ] Tests pass (`go test ./...`, `npm run typecheck`, `npm run lint`)
- [ ] Documentation updated
- [ ] No breaking changes (or clearly noted)
- [ ] Migrations are backward-compatible
- [ ] OpenAPI spec updated (if API changed)
```

### 5. Review

- At least one maintainer review required
- CI must pass (lint, typecheck, tests, build)
- Address all review comments
- Squash commits if asked by maintainer

### 6. Merge

- Maintainers merge via **squash and merge**
- Your branch will be deleted after merge

---

## Release Process

Releases follow [Semantic Versioning](https://semver.org/):

| Change | Version Bump |
|--------|-------------|
| Breaking API or config change | Major (x.0.0) |
| New feature, backward-compatible | Minor (0.x.0) |
| Bug fix, docs, chores | Patch (0.0.x) |

Only maintainers create releases. The process:

1. Update `CHANGELOG.md`
2. Tag the release: `git tag vX.Y.Z`
3. Push tag: `git push upstream vX.Y.Z`
4. Build and push container images
5. Publish GitHub release with notes

---

## Security Issues

**Do not open a public issue for security vulnerabilities.**

Instead:
1. Email `security@yourdomain.com`
2. Include description, reproduction steps, and potential impact
3. Allow 90 days for a fix before public disclosure

See [Security Guide](../operators/security.md) for the project's security model.

---

## Questions?

- Open a GitHub discussion for general questions
- Reference existing docs: [Architecture Overview](../architecture/overview.md), [Data Flow](../architecture/data-flow.md), [Schema](../architecture/schema.md)
- For setup issues, see [Developer Setup Guide](setup.md)

---

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](../../LICENSE).

---

*Last updated: 2026-08-19*