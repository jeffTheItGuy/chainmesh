.PHONY: build-gateway build-admin build-ingestor build-web build-all \
        test test-unit test-integration test-docker test-web test-web-docker \
        test-infra-up test-infra-down test-down \
        lint migrate up up-dev install clean-test-results

# ─── Builds ───────────────────────────────────────────────────────────────────

build-gateway:
	docker build -f backend/gateway/Dockerfile -t blockmesh-gateway:latest ./backend

build-admin:
	docker build -f backend/admin/Dockerfile -t blockmesh-admin:latest ./backend

build-ingestor:
	docker build -f backend/ingestor/Dockerfile -t blockmesh-ingestor:latest ./backend

build-web:
	docker build -f web/Dockerfile -t blockmesh-web:latest ./web

build-all: build-gateway build-admin build-ingestor build-web

# ─── Testing ──────────────────────────────────────────────────────────────────

# Fast unit tests — no Docker, no database, no external deps.
test-unit:
	cd backend && go test -race ./shared/util/... ./shared/blockchain/... ./gateway/middleware/...

# Integration tests — requires a running Postgres.
test-integration:
ifndef TEST_DATABASE_URL
	$(error TEST_DATABASE_URL is not set. Start test infra with 'make test-infra-up', then run this again)
endif
	cd backend && TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -race ./admin/... ./shared/storage/postgres/... ./gateway/proxy/... ./shared/telemetry/...

# Safe default: 'make test' now only runs unit tests.
test: test-unit

# Frontend tests — local (requires Node 20+).
test-web:
	cd web && npm run test:ci

# Frontend tests — Docker only. No Postgres/Redis needed.
test-web-docker:
	@mkdir -p test-results
	docker compose -f docker-compose.test.yml run --rm web-test
	@echo "✅ Frontend results in test-results/web/"

# Full CI pipeline — backend (Go + Postgres/Redis) then frontend (Vitest).
test-docker:
	@mkdir -p test-results
	@# 1. Backend tests
	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit test-runner
	@# 2. Frontend tests
	docker compose -f docker-compose.test.yml run --rm web-test
	@# 3. Tear down test infra
	docker compose -f docker-compose.test.yml down
	@echo "✅ Backend results in test-results/logs/ and test-results/summaries/"
	@echo "✅ Frontend results in test-results/web/"

# ─── Test Infrastructure ──────────────────────────────────────────────────────

# Start only the test databases (Postgres on :5433, Redis on :6380).
test-infra-up:
	docker compose -f docker-compose.test.yml up -d postgres-test redis-test
	@echo "Postgres ready at: postgres://test:test@localhost:5433/test?sslmode=disable"
	@echo "Redis ready at:    localhost:6380"

# Stop test databases and wipe their volumes.
test-infra-down:
	docker compose -f docker-compose.test.yml down -v

# Alias for test-infra-down
test-down: test-infra-down

# Remove locally generated test artifacts
clean-test-results:
	rm -rf test-results

# ─── Existing targets (unchanged) ─────────────────────────────────────────────

lint:
	cd backend && golangci-lint run

migrate:
	psql $(DATABASE_URL) -f backend/database/migrations/001_init.up.sql

up:
	docker compose up --build

up-dev:
	docker compose -f docker-compose.dev.yml up --build

install:
	chmod +x scripts/install.sh && ./scripts/install.sh