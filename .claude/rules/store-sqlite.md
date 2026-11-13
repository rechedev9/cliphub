---
paths:
  - "internal/store/**"
---

# SQLite store

Production state is SQLite (`store.OpenSQLite`). `ZV_DATABASE_URL=memory` and most handler tests use `store.NewMemory`. Both are built in `internal/store/store.go`. A behavioral difference between them is a bug the rest of the suite cannot see.

## Migrations

`migrations` in `internal/store/migrate.go` is the ordered history of `jobs.db`. `PRAGMA user_version` is how many steps have run. `migrate` applies each later step in its own transaction together with the version bump, so a crash leaves the previous version.

- Append a new step. Never edit, reorder or remove a shipped step: it does not run again on a database that already passed it.
- `OpenSQLite` refuses a database from a newer build (`TestOpenSQLiteRefusesADatabaseFromANewerBuild`).
- `internal/store/migrate.go` is near the line cap (`TestGoFileLength`). Put a new step's function in a sibling `migrate_*.go` file and append it to the slice.
- Tests: `internal/store/migrate_test.go`.

## Both implementations, one contract

Every repository interface in `internal/store` has a SQLite type in a `sqlite_*.go` file and a memory type in a `memory_*.go` file. Library reads return a copy the caller owns; a missing record is `library.ErrNotFound` and a broken uniqueness rule is `library.ErrConflict`.

A new or changed method lands in both types and in a shared contract test that runs the same cases against both (`*_contract_test.go`, with helpers such as `jobRepositoriesUnderTest` and `libraryRepositoriesUnderTest`). Do not test only one implementation. When the interface has no shared contract yet, add the case in the same PR. The recipe is [Store method](../skills/store-method/SKILL.md).
