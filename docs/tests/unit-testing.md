<!-- unit-testing.md -->
# Unit Testing

Unit tests verify individual functions, packages, and components in isolation. They should run quickly and should not require live external services.

---

## Go Backend

Backend unit tests live alongside the source code.

```text
backend/
  admin/
    admin_test.go
  gateway/
    gateway_test.go
  shared/
    util/
      util_test.go
```

### Running Backend Unit Tests

From the repository root, run the fast backend unit tests:

```bash
make test-unit
```

The safe default test suite also runs the backend unit tests:

```bash
make test
```

### Containerized Test Pipeline

The containerized test pipeline runs the backend and frontend suites and writes artifacts to `test-results/`.

Run it with:

```bash
make test-docker
```

Artifacts are written under:

```text
test-results/
test-results/logs/
test-results/summaries/
```

### Backend Packages Currently Covered

| Package | Tested behavior |
|---|---|
| `gateway/middleware` | Auth, rate limiting, request ID |
| `shared/blockchain` | Health checks, endpoint selection |
| `shared/util` | Validation helpers |
| `shared/storage/postgres` | Postgres-backed persistence behavior |
| `shared/storage/redis` | Redis-backed rate limiting and cache operations |
| `shared/telemetry` | Telemetry behavior |
| `admin` | Admin API behavior, audit logging |
| `gateway/proxy` | Proxy behavior, cache logic, routing |
| `gateway` | Gateway behavior |
| `ingestor` | Block ingestion behavior |

---

## Frontend

Frontend tests use Vitest.

### Currently Tested Frontend Areas

| File | Tested behavior |
|---|---|
| `src/auth.ts` | Role resolution, session storage behavior, clearing session state |
| `src/api.ts` | Auth error handling and network error handling |
| `src/components/ToastProvider.tsx` | Toast lifecycle behavior |
| `src/TenantsSection.tsx` | Create/edit mode toggle and field pre-fill behavior |

The frontend suite intentionally focuses on high-value logic rather than markup-only components.

### Running Frontend Tests

Run frontend tests locally:

```bash
make test-web
```

Run frontend tests using Docker:

```bash
make test-web-docker
```

The containerized frontend test runner writes artifacts to:

```text
test-results/web/
test-results/web/vitest-results.json
test-results/web/vitest.log
test-results/web/coverage/
```

### Frontend Testing Conventions

- Test user-visible behavior and state changes.
- Do not test markup-only components unless they contain meaningful logic.
- Do not write snapshot tests.
- Mock network requests where necessary.
- Prefer testing public behavior over implementation details.

### Known Frontend Gaps

The following frontend areas are not currently fully tested:

- API client error classification beyond 401/network failure paths
- Confirmation flows in complex forms
- Side effects in complex forms
- Additional form validation flows in `TenantsSection.tsx`
- Other dashboard sections that do not yet have dedicated high-value unit tests

---

## What Not to Test

Avoid low-value tests for:

- Pure presentational components with no logic
- Skeleton loaders
- Static icon markup
- Simple data pass-through components
- Components where the only behavior is rendering props

These are better covered by manual QA, visual review, or higher-level cluster validation.

---

## Related Documents

- [readme.md](readme.md)
- [results.md](results.md)
- [integration-testing.md](integration-testing.md)
- [smoke-test.md](smoke-test.md)
- [load-test.md](load-test.md)