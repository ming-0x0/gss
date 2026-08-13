SHELL := /bin/bash

.PHONY: run build test lint generate clean docker-local-up docker-local-down docker-local-restart

# --- Development ---

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -race -count=1

vet:
	go vet ./...

generate:
	go generate ./...

clean:
	rm -rf bin/

# --- Docker ---

docker-local-up:
	@bash scripts/docker-compose.local.sh up

docker-local-down:
	@bash scripts/docker-compose.local.sh down

docker-local-restart:
	@bash scripts/docker-compose.local.sh
