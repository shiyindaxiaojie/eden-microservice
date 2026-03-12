---
name: backend-common
description: Core Go backend development guidelines, project structure, patterns, and coding standards for eden-go-registry.
---

# Backend Common Skill

This skill covers the foundational rules for backend development in `eden-go-registry`.

## 1. Project Structure

- `cmd/`: Entry points (main.go).
- `internal/`: Private code.
  - `handler/`: HTTP handlers (Gin).
  - `service/`: Business logic.
  - `repository/`: Data access (GORM).
  - `model/`: Data models.
  - `wire/`: Dependency injection (Google Wire).
- `pkg/`: Public libraries.

## 2. Architecture Patterns

Follow the **Handler -> Service -> Repository** pattern strictly.

- **Dependency Injection**: Use Wire. Define providers in `provider.go` files.
- **Interface-Based**: Service and Repository layers must be interface-based for testability.

## 3. Database Rules (GORM)

- **No Foreign Keys**: Logic must handle data integrity.
- **Field Naming**: Snake case (`created_at`, `is_active`).
- **Types**:
  - `BIGINT UNSIGNED` for IDs.
  - `DATETIME` for time (not TIMESTAMP).
  - `TINYINT` for status/enums.
- **Repository**:
  - One file per table (e.g., `user_repository.go` for `users` table).
  - Return `nil, nil` for not found, **DO NOT** return `gorm.ErrRecordNotFound`.

## 4. Error Handling & Response

- **Error Codes**: Defined in `internal/pkg/errors/codes.go`.
  - Format: `MODULE_TYPE_DETAIL` (e.g., `SYS_PARAM_INVALID`).
- **Response**:
  - Success: `response.Success(c, data)`
  - Error: `response.Error(c, http.Code, "msg", err)` or `response.ErrorWithCode(c, code, details...)`

## 5. Creating a New Module

1.  **Model**: Define struct in `internal/model/<module>/`.
2.  **Repository**: Create interface & implementation in `internal/repository/<module>/`.
3.  **Service**: Create interface & implementation in `internal/service/<module>/`.
4.  **Handler**: Create handler in `internal/handler/<module>/`.
5.  **Router**: Register in `internal/router/<module>_routes.go`.
6.  **Wire**: Add to provider sets in `internal/*/providers.go` and regenerate wire.

## 6. Database & SQL Rules

- **Schema Location**: `scripts/sql/`.
- **Naming Format**: `V{Version}__{Description}.sql` (e.g., `V0.0.1__Initial_Schema.sql`).
- **Design Rules**:
  - **NO Foreign Keys**: Logic must handle data integrity.
  - **NO Triggers/Stored Procedures**.
  - **Idempotency**: Ensure scripts can be run safely (though usually handled by migration tool).
- **Field Naming**: Snake case (`created_at`, `is_active`).
- **Types**:
  - `BIGINT UNSIGNED` for IDs.
  - `DATETIME` for time.
  - `TINYINT` for status/enums.
