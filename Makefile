.PHONY: build-gateway build-admin build-ingestor build-web test lint migrate deploy up install

build-gateway:
	docker build -f backend/Gateway.Dockerfile -t blockmesh-gateway:latest ./backend

build-admin:
	docker build -f backend/Admin.Dockerfile -t blockmesh-admin:latest ./backend

build-ingestor:
	docker build -f backend/Ingestor.Dockerfile -t blockmesh-ingestor:latest ./backend

build-web:
	docker build -f web/Dockerfile -t blockmesh-web:latest ./web

build-all: build-gateway build-admin build-ingestor build-web

test:
	cd backend && go test -race ./...

lint:
	cd backend && golangci-lint run

migrate:
	psql $(DATABASE_URL) -f migrations/001_init.up.sql

deploy:
	kubectl apply -k deployments/base/

up:
	docker compose up --build

install:
	chmod +x scripts/install.sh && ./scripts/install.sh
