---
name: backend-system
description: Backend development for System module (User, Role, Menu, Task).
---

# Backend System Module Skill

This skill allows you to work on the System module in the backend.

## 1. Scope
- **User Management**: Authentication, profiles, password management.
- **Role Management**: RBAC, permissions.
- **Menu Management**: Dynamic menu generation.
- **Scheduled Tasks**: Cron jobs for system maintenance.

## 2. Key Paths
- **Models**: `internal/model/system/`
- **Repositories**: `internal/repository/system/`
- **Services**: `internal/service/system/`
- **Handlers**: `internal/handler/system/`
- **Routes**: `internal/router/system_routes.go`

## 3. Error Codes
- **Prefix**: `SYS_*` (10xxxxx) and `USER_*` (11xxxxx).
- **Files**: `internal/pkg/errors/codes.go`.

## 4. Dependencies
- Depends on `backend-common` (Wire, GORM, Gin).
