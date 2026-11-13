---
name: store-method
description: Add or change a repository method in both the SQLite and memory stores, with a shared contract test. Use when internal/store gains or changes a method.
---

# Store method

The rule is [SQLite store](../../rules/store-sqlite.md).

## Steps

1. Add the method to its interface in `internal/store/store.go`, or to the embedded interface it comes from.
2. Implement it on the SQLite type (`sqlite_*.go`) and the memory type (`memory_*.go`).
3. A schema change is a new step appended to `migrations` in `internal/store/migrate.go`. Put the function in a sibling file when `internal/store/migrate.go` is near 1000 lines. A shipped step stays as it was.
4. Add a case to the shared contract that runs the same assertions on memory and SQLite. Use the existing helper for that interface (`jobRepositoriesUnderTest`, `libraryRepositoriesUnderTest`). A stream or editor-asset method that has no shared case yet gets one in this change.

## Checks

```bash
go test ./internal/store -run TestName -count=1
go test ./internal/store -count=1
go vet ./...
```

Replace `TestName` with the contract you extended (for example `TestJobRepositoryContract` or `TestLibraryRepositoryContract`). A migration change also runs `go test ./internal/store -run TestMigrate -count=1`. That regex matches the `TestMigrateV*` tests in `internal/store/migrate_test.go`. The package test above also runs `TestOpenSQLiteRefusesADatabaseFromANewerBuild`.

The store change is finished when `go test ./internal/store -count=1` and `go vet ./...` are green.
