DATABASE_URL ?= postgres://user:password@localhost:5432/snaelda?sslmode=disable
POSTGRES_PORT ?= 5432

.PHONY: api db-up db-down db-migrate db-seed test

api:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

db-up:
	POSTGRES_PORT="$(POSTGRES_PORT)" docker compose up -d postgres

db-down:
	docker compose down

db-migrate:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/db migrate up

db-seed:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/db seed

test:
	go test ./...
