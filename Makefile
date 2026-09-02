.PHONY: build-gateway build-admin build-ingestor build-web build-all \
	test test-unit test-integration test-integration-down \
	test-integration-logs test-docker test-web test-web-docker \
	test-infra-up test-infra-down test-down \
	lint migrate up up-dev install clean-test-results

####### Builds ###############################

build-gateway:
	docker build -f backend/gateway/Dockerfile -t blockmesh-gateway:latest ./backend

build-admin:
	docker build -f backend/admin/Dockerfile -t blockmesh-admin:latest ./backend

build-ingestor:
	docker build -f backend/ingestor/Dockerfile -t blockmesh-ingestor:latest ./backend

build-web:
	docker build -f web/Dockerfile -t blockmesh-web:latest ./web

build-all: build-gateway build-admin build-ingestor build-web

######## Testing  ###############################################################

test-unit:
	cd backend && go test -race ./shared/util/... ./shared/blockchain/... ./gateway/middleware/...

test-integration:
	@mkdir -p test-results
	docker compose -f docker-compose.integration.yml down -v 2>/dev/null || true
	docker compose -f docker-compose.integration.yml up --build --abort-on-container-exit
	docker compose -f docker-compose.integration.yml down -v

test-integration-logs:
	@echo "=== Integration Test Summary ==="
	@cat test-results/integration-summary.log 2>/dev/null || echo "No summary found. Run 'make test-integration' first."
	@echo ""
	@echo "Full log location: test-results/integration-full.log"

test-integration-down:
	docker compose -f docker-compose.integration.yml down -v

test: test-unit

test-web:
	cd web && npm run test:ci

test-web-docker:
	@mkdir -p test-results
	docker compose -f docker-compose.test.yml run --rm web-test
	@echo "✅ Frontend results in test-results/web/"

test-docker:
	@mkdir -p test-results
	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit test-runner
	docker compose -f docker-compose.test.yml run --rm web-test
	docker compose -f docker-compose.test.yml down
	@echo "✅ Backend results in test-results/logs/ and test-results/summaries/"
	@echo "✅ Frontend results in test-results/web/"

###### Test Infrastructure  #################################################################

test-infra-up:
	docker compose -f docker-compose.test.yml up -d postgres-test redis-test
	@echo "Postgres ready at: postgres://test:test@localhost:5433/test?sslmode=disable"
	@echo "Redis ready at:    localhost:6380"

test-infra-down:
	docker compose -f docker-compose.test.yml down -v

test-down: test-infra-down

clean-test-results:
	rm -rf test-results

####### Existing Targets ##################################################

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