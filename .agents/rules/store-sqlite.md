# SQLite store

Production state is SQLite (`store.OpenSQLite`). `ZV_DATABASE_URL=memory` and most handler tests use `store.NewMemory`. Both are built in `internal/store/store.go`. A behavioral difference between them is a bug the rest of the suite cannot see.

## Migrations

`migrations` in `internal/store/migrate.go` is the ordered history of `jobs.db`. `PRAGMA user_version` is how many steps have run. `migrate` applies each later step in its own transaction together with the version bump, so a crash leaves the previous version. A step never runs twice, and a newer binary never edits an older step. `OpenSQLite` returns an error when `user_version` is newer than this binary (`TestOpenSQLiteRefusesADatabaseFromANewerBuild`).

`internal/store/migrate.go` is near the 1000-line cap (`TestGoFileLength`). New steps go in a sibling file and are appended to the slice. `migrateV23LibraryVoiceRecordings` is in `internal/store/migrate_voice_recordings.go`; `migrateV24PatternDetection` is in `internal/store/migrate_pattern_detection.go`. The tests are `internal/store/migrate_test.go`.

## Both implementations, one contract

Every method on these interfaces has a SQLite method and a memory method. Confirmed pairs:

| Interface | SQLite | Memory |
| --- | --- | --- |
| `JobRepository` (12) | `SQLiteJobRepository` in `internal/store/sqlite_repo.go` | `MemoryJobRepository` in `internal/store/memory_repo.go` |
| `StreamJobRepository` (9) | `internal/store/sqlite_stream_repo.go` | `internal/store/memory_repo.go` |
| `EditorAssetRepository` (5) | `internal/store/sqlite_editor_repo.go` | `internal/store/memory_editor_repo.go` |
| `EditorProjectRepository` (7) | `internal/store/sqlite_editor_repo.go` | `internal/store/memory_editor_repo.go` |
| `LibraryRepository` (42), plus embedded `LibretaRepository` (20), `PlaylistRepository` (5), `PatternRepository` (7), `ProMatchRepository` (8), `VoiceRepository` (9), `VoiceRecordingRepository` (7) | `internal/store/sqlite_library_*.go` | `internal/store/memory_library_*.go` |

The embedded interfaces are declared in `internal/store/libreta.go`, `internal/store/playlists.go`, `internal/store/patterns.go`, `internal/store/library_pro.go`, `internal/store/voice.go` and `internal/store/voice_recording.go`. Library reads return a copy the caller owns; a missing record is `library.ErrNotFound` and a broken uniqueness rule is `library.ErrConflict` (`LibraryRepository` in `internal/store/store.go`).

A new or changed method lands in both types and in a contract that runs the same cases against both (`jobRepositoriesUnderTest`, `libraryRepositoriesUnderTest`, or the same map for that interface). The recipe is [Store method](../skills/store-method/SKILL.md).

Contracts that already run both implementations:

- `TestJobRepositoryContract` — `internal/store/contract_test.go`
- `TestLibraryRepositoryContract` — `internal/store/library_contract_test.go`
- `TestLibrarySharedDemoContract` — `internal/store/library_shared_contract_test.go`
- `TestLibraryProMatchContract` — `internal/store/library_pro_contract_test.go`
- `TestLibretaRepositoryContract` — `internal/store/libreta_contract_test.go`
- `TestPlaylistRepositoryContract` — `internal/store/playlist_contract_test.go`
- `TestPatternRepositoryContract` — `internal/store/patterns_contract_test.go`
- `TestPatternCategoryPlayAndDetectionContract` — `internal/store/pattern_matches_contract_test.go`
- `TestVoiceRepositoryContract` — `internal/store/voice_contract_test.go`
- `TestVoiceRecordingRepositoryContract` — `internal/store/voice_recording_contract_test.go`
- `TestStreamRepoSetSourceProbeKeepsStatusAndOrder` — `internal/store/sqlite_stream_repo_test.go` (this is the stream case that runs both)
- `TestSweepInterruptedEditorRendersFailsOnlyRenderingProjects` — `internal/reconcile/sweep_editor_test.go` (editor projects, both)

`EditorAssetRepository` has both implementations and no shared contract test yet. The other stream methods have both implementations and no shared case beyond `SetSourceProbe`. A change to one of those methods adds the shared case in the same PR.
