COMPOSE_ENV_FILES := $(if $(wildcard .env.local),--env-file .env.local,) $(if $(wildcard .env),--env-file .env,)

.PHONY: api db-up db-down db-migrate db-seed dev-up storage-up storage-down stripe-setup test

api:
	go run ./cmd/api

dev-up:
	docker compose $(COMPOSE_ENV_FILES) up -d postgres seaweedfs

db-up:
	docker compose $(COMPOSE_ENV_FILES) up -d postgres

db-down:
	docker compose $(COMPOSE_ENV_FILES) down

storage-up:
	docker compose $(COMPOSE_ENV_FILES) up -d seaweedfs

storage-down:
	docker compose $(COMPOSE_ENV_FILES) stop seaweedfs

db-migrate:
	go run ./cmd/db migrate up

db-seed:
	go run ./cmd/db seed

stripe-setup:
	go run ./cmd/stripe-setup $(STRIPE_SETUP_ARGS)

test:
	go test ./...
