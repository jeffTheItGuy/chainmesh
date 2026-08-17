.PHONY: build-gateway build-admin build-ingestor build-web test lint migrate up up-dev install

build-gateway:
	docker build -f backend/gateway/Dockerfile -t blockmesh-gateway:latest ./backend

build-admin:
	docker build -f backend/admin/Dockerfile -t blockmesh-admin:latest ./backend

build-ingestor:
	docker build -f backend/ingestor/Dockerfile -t blockmesh-ingestor:latest ./backend

build-web:
	docker build -f web/Dockerfile -t blockmesh-web:latest ./web

build-all: build-gateway build-admin build-ingestor build-web

test:
	cd backend && go test -race ./...

lint:
	cd backend && golangci-lint run

migrate:
	psql $(DATABASE_URL) -f migrations/001_init.up.sql

up:
	docker compose up --build

up-dev:
	docker compose -f docker-compose.dev.yml up --build

install:
	chmod +x scripts/install.sh && ./scripts/install.sh
