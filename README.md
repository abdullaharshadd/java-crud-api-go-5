# java-crud-api (Go port)

A user-management CRUD HTTP API, migrated from Java/Spring to Go using the standard library. The original Spring project is `abdullaharshadd/java-crud-api`, with Java package `com.smartContact`. It exposes create, read, update and delete operations on a `User` resource backed by a relational database.

> **Status:** The migration has only been build-checked. The unit tests have **not** been run. Runtime behavior has **not** been compared with the original Spring application. The 83% figure is the migration tool's own estimate, not a measured result. Treat this branch as unverified until you complete the review items below.

## Verification Status

| Check | Result |
|---|---|
| Build | ✅ Passed |
| Target unit tests | ⚠️ Not run |
| Behavior compared with original | ⚠️ Not run |
| Modules migrated | 8 / 8 (3 flagged low-confidence) |

## Tech Stack

- **Language:** Go. See `go.mod` for the module path and Go version.
- **HTTP:** Go standard library (`net/http`). Routing is defined in `cmd/server/router.go`.
- **Configuration:** environment variables, loaded in `internal/config/config.go`.
- **Persistence:** a relational database reached through `DATABASE_URL`. Data access is in `internal/repository/user.go`.

## Prerequisites

- The Go toolchain, at the version declared in `go.mod`.
- A running relational database reachable through `DATABASE_URL`. Check `internal/repository/user.go` and `go.mod` to see which driver is used.
- The `user` table must already exist. See [Known Limitations](#known-limitations): the Spring app created this table automatically, and that behavior has not been verified in the Go port.

## Getting Started

The commands below assume the repository is checked out at `/app`. If you cloned it elsewhere, replace `/app` with your path.

### 1. Install dependencies and build

```sh
cd /app && go mod tidy && go mod download && go build -o /app/bin/server ./cmd/server
```

### 2. Resolve dependencies

This command was run during the migration. It is already part of step 1, and running it again is harmless:

```sh
go mod tidy
```

### 3. Set environment variables

```sh
export DATABASE_URL="<your database connection string>"
export PORT="<port to listen on>"
export JWT_SECRET="<secret value>"
```

Check `internal/config/config.go` for the exact format each variable expects and for any defaults.

### 4. Database setup

No database setup command was produced by the migration. Before you run the server, create the schema that the original JPA `User` entity generated (table, columns, unique constraint and column lengths). Base it on the model in `internal/model/user.go` and the queries in `internal/repository/user.go`.

### 5. Run

```sh
/app/bin/server
```

## Running Tests

```sh
cd /app && go test ./...
```

The only test file is `internal/service/errors_test.go`. The tests have **not** been run as part of the migration, so their pass/fail status is unknown. Nothing currently tests the handlers, the repository or the service logic in `internal/service/user.go`.

## Environment Variables

| Variable | Purpose | Notes |
|---|---|---|
| `DATABASE_URL` | Database connection string | Required. Format depends on the driver; check `internal/config/config.go` and `internal/repository/user.go`. |
| `PORT` | Port the HTTP server listens on | Check `internal/config/config.go` for a default. |
| `JWT_SECRET` | Secret for JWT signing/verification | Detected as required. Check `internal/config/config.go` and `internal/httpapi/handler.go` to confirm how it is used. |

## Architecture Overview

```
cmd/server/
  main.go            Entry point: loads config, wires dependencies, starts the HTTP server
  router.go          Route registration (maps paths/methods to handlers)
internal/config/
  config.go          Environment-variable configuration
internal/httpapi/
  handler.go         HTTP handlers (replaces the Spring UserController)
  errors.go          Mapping of errors to HTTP responses
internal/model/
  user.go            User domain type (replaces the JPA User entity + Lombok)
  errormessage.go    Error response payload type
internal/service/
  user.go            Business logic (replaces UserServiceImp)
  errors.go          Service-level error definitions
  errors_test.go     Tests for service errors
internal/repository/
  user.go            Explicit data access (replaces the Spring Data UserDao)
```

Requests flow in one direction: `router.go` passes them to `httpapi` handlers, the handlers call `service`, and `service` calls `repository`, which talks to the database. Errors returned by the service layer are converted to HTTP responses in `internal/httpapi/errors.go`, using the payload defined in `internal/model/errormessage.go`.

## Migration Notes

These are the main changes from the original Spring codebase:

- **Dependency injection:** Spring auto-wiring has been replaced by explicit construction and wiring in `cmd/server/main.go`.
- **Controllers:** the Spring `@RestController` (`UserController`) is now standard-library HTTP handlers in `internal/httpapi/handler.go`. Routes are registered explicitly in `cmd/server/router.go`.
- **Exception handling:** Spring exception handling has been replaced by Go error values (`internal/service/errors.go`) and explicit error-to-HTTP mapping (`internal/httpapi/errors.go`).
- **Repository:** the Spring Data `UserDao` interface had its implementation generated at runtime from method names. It has been replaced by hand-written data access in `internal/repository/user.go`.
- **Entity:** the JPA/Lombok `User` entity is now a plain Go struct in `internal/model/user.go`. Lombok-generated constructors and accessors no longer exist; use the struct fields directly.
- **Configuration:** Spring `application.properties`/profiles have been replaced by environment variables read in `internal/config/config.go`.

## Known Limitations

These components could not be migrated mechanically:

1. **Automatic schema generation (`User` entity).**
   - **Original behavior:** Hibernate (`ddl-auto`) created the table at startup from the JPA annotations. Go has no equivalent annotation-driven mechanism.
   - **Risk:** unless the Go code explicitly creates the table, a fresh database will have no schema.
   - **Fix:** confirm whether startup runs a `CREATE TABLE IF NOT EXISTS` (or an equivalent migration) with the same table name, column names, unique constraint and column lengths. If it does not, add one.

2. **Lombok annotations (`User` entity).**
   - **Original behavior:** Lombok generated constructors and accessors at compile time. These were replaced by a plain Go struct.
   - **Fix:** confirm that the password field is never included in logs, JSON responses or string representations.

3. **Runtime-generated repository (`UserDao`).**
   - **Original behavior:** Spring Data derived queries from method names. It also provided a persistence context with dirty checking, so modified entities were saved implicitly. Neither exists in Go.
   - **Fix:** every query in `internal/repository/user.go` is hand-written and must be checked against the original semantics, including uniqueness and not-found handling. Any update the original relied on implicit dirty checking for must now be an explicit write.

4. **Cascade / orphan removal on `User.contacts`.**
   - **Original behavior:** `userDao.save(user)` implicitly persisted, updated and removed related contacts through JPA cascade and `orphanRemoval`. There is no direct equivalent in Go.
   - **Fix:** child-record persistence must be explicit and wrapped in a transaction. Audit every code path that originally called `save` on a user with modified contacts.

No setup commands failed during migration.

## Manual Review Required

Before relying on this branch, verify the following:

- [ ] **`internal/model/user.go`** (low confidence; ported from the `User` entity)
  - Field names, types and JSON tags match the original API contract.
  - Database column mapping matches the original table.
  - The password is excluded from serialization.
- [ ] **`internal/service/user.go`** (low confidence; ported from `UserServiceImp`)
  - Business rules, validation and error cases match the original.
  - Writes that previously relied on JPA dirty checking or cascades are now explicit.
- [ ] **`internal/httpapi/handler.go`** (low confidence; ported from `UserController`)
  - Routes, HTTP methods and status codes match the original endpoints.
  - Request/response bodies match the original endpoints.
  - Error responses match the original.
  - Cross-check the routes against `cmd/server/router.go`.
- [ ] **`internal/repository/user.go`** (replaces `UserDao`)
  - Each query is correct and matches the original method's semantics.
  - Contact persistence is handled explicitly and transactionally.
- [ ] **Schema creation**
  - Confirm how, and whether, the `user` table (and any contacts table) is created on a fresh database.
- [ ] **`JWT_SECRET` usage**
  - Confirm where it is used and that any authentication behavior matches the original.
- [ ] **Tests**
  - Run `cd /app && go test ./...`.
  - Add tests for the handlers, the service and the repository; currently only `internal/service/errors_test.go` exists.
- [ ] **Behavioral parity**
  - Run the original Spring app and this Go server against equivalent databases.
  - Compare responses endpoint by endpoint.