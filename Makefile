SHELL := /bin/bash

.PHONY: run docker-local-up docker-local-down docker-local-restart

run:
	@go run ./cmd/api

docker-local-up:
	@bash scripts/docker-compose.local.sh up

docker-local-down:
	@bash scripts/docker-compose.local.sh down

docker-local-restart:
	@bash scripts/docker-compose.local.sh

