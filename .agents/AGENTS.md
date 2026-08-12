# Workspace Rules & Architectural Guidelines (`.agents/AGENTS.md`)

This file contains the mandatory rules and simple design principles for the `gss` project.

## 🏛️ 1. Simplified 3-Layer Architecture

All code MUST strictly follow the clean, pragmatic 3-layer layout:

1. **Domain Models (`internal/domain/`)**:
   - Pure Go structs (`user.go`) representing domain entities.
   - Generated mocks (`domain/mocks/`) for repositories and services.
2. **Repository Layer (`internal/repository/`)**:
   - Interface contracts (`UserRepository`) & GORM MySQL implementations.
   - Handles database queries and transaction management.
3. **Service Layer (`internal/service/`)**:
   - Contains core business logic (`AuthService`).
   - Implements service interfaces and depends on repository interfaces.
4. **Handler Layer (`internal/handler/`)**:
   - HTTP transport adapters (`Gin` framework handlers, DTOs, OpenAPI routes).
   - Delegates business logic execution to services.
5. **Configuration (`configs/`)**:
   - Root package `configs/` manages application configuration using Viper.

---

## 🧪 2. Test-Driven Development (TDD) Standards

1. **Table-Driven Tests**: Write structured unit tests for services (`internal/service/*_test.go`) and HTTP handlers (`internal/handler/*_test.go`).
2. **Mocking**: Use `go.uber.org/mock/mockgen` for mocking interfaces.
3. **Parallel Execution**: Enable `t.Parallel()` on subtests for optimal performance.

---

## ⚙️ 3. Code Generation & Commands

- Run `make generate` (`go generate ./...`) whenever repository or service interface signatures change.
- Keep dependencies cleanly injected via constructor functions (`New...`).
