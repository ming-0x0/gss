---
name: project_architecture
description: Guidelines and simple patterns for expanding features in the 3-layer Go project layout (domain, repository, service, handler).
---

# Simplified Project Architecture Skill

This skill provides simple rules for developing new features.

## Layer Structure

- **Domain (`internal/domain/`)**: Pure entity structs.
- **Repository (`internal/repository/`)**: Interfaces and GORM DB access.
- **Service (`internal/service/`)**: Business logic implementations.
- **Handler (`internal/handler/`)**: Gin HTTP endpoints & DTOs.

## Workflow for Adding Features

1. Create entity in `internal/domain/<feature>.go`.
2. Create repository interface & impl in `internal/repository/<feature>.go` with `//go:generate`.
3. Create service interface & impl in `internal/service/<feature>.go` with `//go:generate`.
4. Create HTTP handler in `internal/handler/<feature>.go`.
5. Wire dependencies in `internal/app/app.go`.
