# Migration Notes

**Model self-assessed confidence:** 83% (not a measured result)  
**Build at time of writing:** passed  
**Target unit tests:** not run  
**Behavior compared with the original:** not run

The final recommendation is in the pull request description.

---

## What was migrated

- `src/main/java/com/smartContact/error/UserNotFoundException.java` → `internal/service/errors.go` (89% confidence)
- `src/main/java/com/smartContact/model/User.java` → `internal/model/user.go` (80% confidence) ⚠️ needs review
- `src/main/java/com/smartContact/model/ErrorMessage.java` → `internal/model/errormessage.go` (89% confidence)
- `src/main/java/com/smartContact/repository/UserDao.java` → `internal/repository/user.go` (85% confidence)
- `src/main/java/com/smartContact/service/UserService.java` → `internal/service/user.go` (85% confidence)
- `src/main/java/com/smartContact/error/RestResponseEntityExceptionHandling.java` → `internal/httpapi/errors.go` (86% confidence)
- `src/main/java/com/smartContact/service/UserServiceImp.java` → `internal/service/user.go` (72% confidence) ⚠️ needs review
- `src/main/java/com/smartContact/Controller/UserController.java` → `internal/httpapi/handler.go` (80% confidence) ⚠️ needs review

## Components that could not be automatically migrated

These components require manual implementation. The migrated code contains
`MIGRATION_NOTE` comments at the relevant locations.

### `User (@Entity schema auto-generation)` in `src/main/java/com/smartContact/model/User.java`
**Reason:** JPA/Hibernate derives and creates the DDL implicitly from annotations at startup (ddl-auto). Most target stacks have no identical annotation-driven implicit mechanism.
**Suggestion:** Make it explicit: define the model in the target ORM (SQLAlchemy, Prisma, TypeORM, GORM, Sequelize, etc.) with identical table and column names, unique constraint and lengths. Then guarantee creation at boot via create_all/sync/AutoMigrate, a CREATE TABLE IF NOT EXISTS executed on startup, or a migration actually run in the startup path.

### `Lombok annotations` in `src/main/java/com/smartContact/model/User.java`
**Reason:** Compile-time code generation specific to Java.
**Suggestion:** Replace with the target's native data-class or record constructs, or with explicit constructors and accessors. Exclude password from string representations.

### `UserDao (runtime-generated proxy / query derivation)` in `src/main/java/com/smartContact/repository/UserDao.java`
**Reason:** Go has no runtime generation of repository implementations from interface method names, and no persistence context with dirty checking.
**Suggestion:** Manual rewrite: write explicit SQL in sqlx (e.g. SELECT ... FROM user WHERE name = ? LIMIT 2) or gorm calls (db.Where("name = ?", name).First(&u)). Implement only the methods used by callers.

### `JPA cascade/orphanRemoval behavior via save(user)` in `src/main/java/com/smartContact/repository/UserDao.java`
**Reason:** Cascading persistence of the User.contacts collection is implicit JPA/Hibernate behavior with no direct sqlx equivalent.
**Suggestion:** Make child persistence explicit in a ContactRepository within a transaction, or use gorm association handling. Audit every userDao.save call that mutates contacts.

## Observer agent findings

The Observer agent monitored the migration and identified these patterns:

- **After 3 modules:** Behavior that the Java framework provides implicitly (Hibernate id generation and schema creation, Jackson null handling) is being translated piecemeal without explicit contracts, so cross-module gaps and serialization changes show up as unresolved findings.
- **After 6 modules:** Confidence is held down because Java/Spring implicit behaviors (null semantics, schema auto-creation, ID generation, framework wiring) are only partly reproduced in Go and not tracked across modules, while UserDao and UserService got no specs, so their 0.85 scores rest on no verified behavior.

## Files requiring manual review

These files were migrated but scored below the confidence threshold.
Review them carefully before merging.

### `src/main/java/com/smartContact/model/User.java`
Confidence: 80%
Issues:
  - [warning] EnsureUserSchema is defined, but nothing in the target calls it. The target currently has no DB bootstrap code at all (searches for 'EnsureUserSchema' and 'sql.Open' found nothing), so the call may simply come in a later migration step.
  - [info] user_id has no AUTO_INCREMENT. Ids depend on the hibernate_sequence table, so whatever repository code inserts users must read and increment that sequence. This matches Hibernate 5 on MySQL but depends on code not written yet.

### `src/main/java/com/smartContact/service/UserServiceImp.java`
Confidence: 72%

### `src/main/java/com/smartContact/Controller/UserController.java`
Confidence: 80%
Issues:
  - [warning] The code itself says cmd/server/router.go still has to mount h.Routes(). A search of the target found no call to NewHandler or Routes, so the endpoints may not be reachable yet.
