SHELL := /bin/bash

.PHONY: docker-local-up docker-local-down docker-local-restart 


docker-local-up:
	@bash scripts/docker-compose.local.sh up

docker-local-down:
	@bash scripts/docker-compose.local.sh down

docker-local-restart:
	@bash scripts/docker-compose.local.sh

