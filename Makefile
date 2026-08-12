SHELL := /bin/bash

.PHONY: help run build test generate clean docker-local-up docker-local-down docker-local-restart

help: ## Hiển thị hướng dẫn các lệnh Make
	@echo "Danh sách lệnh Make khả dụng:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Chạy API local server
	@go run ./cmd/api

build: ## Build binary ứng dụng
	@go build -v -o bin/api ./cmd/api

test: ## Chạy toàn bộ unit test
	@go test -v ./...

generate: ## Generate mocks và stringers
	@go generate ./...

clean: ## Dọn dẹp file build
	@rm -rf bin/

docker-local-up: ## Khởi chạy toàn bộ container local (MySQL, MinIO)
	@bash scripts/docker-compose.local.sh up

docker-local-down: ## Dừng các container local
	@bash scripts/docker-compose.local.sh down

docker-local-restart: ## Restart lại các container local
	@bash scripts/docker-compose.local.sh
