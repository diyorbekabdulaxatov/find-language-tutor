SHELL := /bin/bash
SQLC ?= $(shell go env GOPATH)/bin/sqlc

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: db-up
db-up: ## Start Postgres + Redis via docker compose
	docker compose up -d
	@echo "waiting for postgres..." && until docker compose exec -T postgres pg_isready -U findtutor >/dev/null 2>&1; do sleep 1; done
	@echo "ready."

.PHONY: db-down
db-down: ## Stop and remove the docker compose stack
	docker compose down

.PHONY: db-reset
db-reset: ## Drop volumes and recreate a clean database
	docker compose down -v
	$(MAKE) db-up
	$(MAKE) migrate-up
	$(MAKE) seed

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	go run ./cmd/migrate down 1

.PHONY: migrate-version
migrate-version: ## Print the current migration version
	go run ./cmd/migrate version

.PHONY: seed
seed: ## Load the demo teacher catalog
	go run ./cmd/seed

.PHONY: sqlc
sqlc: ## Regenerate the sqlc query code
	$(SQLC) generate

.PHONY: run
run: ## Run the HTTP API
	go run ./cmd/api

.PHONY: worker
worker: ## Run the background worker
	go run ./cmd/worker

.PHONY: build
build: ## Compile all binaries into ./bin
	go build -o bin/ ./cmd/...

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy
