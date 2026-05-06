DATABASE_URL ?= postgres://user:password@localhost:5432/snaelda?sslmode=disable
POSTGRES_PORT ?= 5432
S3_ENDPOINT ?= http://localhost:8333
S3_BUCKET ?= snaelda-local
S3_REGION ?= us-east-1
S3_ACCESS_KEY_ID ?= snaelda
S3_SECRET_ACCESS_KEY ?= snaelda-secret

.PHONY: api db-up db-down db-migrate db-seed dev-up storage-up storage-down test

api:
	DATABASE_URL="$(DATABASE_URL)" S3_ENDPOINT="$(S3_ENDPOINT)" S3_BUCKET="$(S3_BUCKET)" S3_REGION="$(S3_REGION)" S3_ACCESS_KEY_ID="$(S3_ACCESS_KEY_ID)" S3_SECRET_ACCESS_KEY="$(S3_SECRET_ACCESS_KEY)" go run ./cmd/api

dev-up:
	POSTGRES_PORT="$(POSTGRES_PORT)" S3_BUCKET="$(S3_BUCKET)" S3_ACCESS_KEY_ID="$(S3_ACCESS_KEY_ID)" S3_SECRET_ACCESS_KEY="$(S3_SECRET_ACCESS_KEY)" docker compose up -d postgres seaweedfs

db-up:
	POSTGRES_PORT="$(POSTGRES_PORT)" docker compose up -d postgres

db-down:
	docker compose down

storage-up:
	S3_BUCKET="$(S3_BUCKET)" S3_ACCESS_KEY_ID="$(S3_ACCESS_KEY_ID)" S3_SECRET_ACCESS_KEY="$(S3_SECRET_ACCESS_KEY)" docker compose up -d seaweedfs

storage-down:
	docker compose stop seaweedfs

db-migrate:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/db migrate up

db-seed:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/db seed

test:
	go test ./...
