---
name: store-method
description: Add or change a repository method in internal/store on both the SQLite and the memory implementation, with a shared contract test, and add a schema migration when needed. Use when adding or changing a method on a repository interface in internal/store, adding a table or column to jobs.db, or when a *_contract_test.go fails for only one implementation.
---

# Store method

The rule is [SQLite store](../../rules/store-sqlite.md).

## Steps

1. Add the method to its repository interface in `internal/store`.
2. Implement it on the SQLite type (`sqlite_*.go`) and the memory type (`memory_*.go`).
3. For a schema change, append a new step to `migrations` in `internal/store/migrate.go` and put its function in a sibling `migrate_*.go` file. Do not edit a shipped step.
4. Add a case to the shared contract test that runs the same assertions on memory and SQLite. Use the existing helper for that interface (`jobRepositoriesUnderTest`, `libraryRepositoriesUnderTest`). If the interface has no shared contract yet, add one in this change.

## Checks

```bash
go test ./internal/store -run TestName -count=1
go test ./internal/store -count=1
go vet ./...
```

Replace `TestName` with the contract you extended (for example `TestJobRepositoryContract` or `TestLibraryRepositoryContract`). For a migration, `-run TestMigrate` runs the migration tests in `internal/store/migrate_test.go`.

The store change is finished when the new case runs against both implementations and `go test ./internal/store -count=1` and `go vet ./...` are green.
