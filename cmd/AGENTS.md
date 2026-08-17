# cmd/

Twelve `package main` binaries → `bin/zv*`. Contract: thin flags + `os.Exit`; domain stays in `internal/`. Several bins already violate that — do not add more leak.

## WHERE TO LOOK

| Binary | Role | Logic lives |
|--------|------|-------------|
| `zv` | Unified CLI dispatcher | This tree + `cmd/zv/AGENTS.md` |
| `zv-parser` | `.dem` → kill plan / utility-audit | `internal/parser` |
| `zv-demo-players` | Roster | **Leak:** own `demoinfocs` parse, not `internal/parser` |
| `zv-recorder` | HLAE/CS2 capture | **Leak:** launch/teardown in `main.go` (~1415). Plan/validate in `internal/recording` |
| `zv-editor` | FFmpeg/Lua shorts + pack | `internal/editor` |
| `zv-composer` | Concat → `final.mp4` | `internal/composition` |
| `zv-stream` | Thin `streamcli.Run` | `internal/streamcli` |
| `zv-rhythm` | Beats/onsets | `internal/rhythm` |
| `zv-orchestrator` | `zv serve` | **Leak:** SQLite + `inline_queue.go`. HTTP/workers in `internal/` |
| `zv-tui` | Bubble Tea client | Views here; HTTP in `internal/tuiclient` |
| `zv-tactical-data` | Tick-window JSON export | `internal/tactical` |
| `zv-analysis-viewer` | Loopback HTML death viewer | **Leak:** no `internal/` import |

Build list is `scripts/build.ps1` / `Makefile` — both must list the same 12.

## CONVENTIONS

- New surface: add a `zv` subcommand that delegates. Do not invent a 13th binary without a product decision.
- Recorder: `--dry-run` never launches CS2; `--fake` writes placeholder MP4s. Never kill `cs2.exe` by image name.
- Orchestrator: one-worker capture lane. Desktop packaged path starts `zv-orchestrator.exe` directly, not `zv serve`.

## ANTI-PATTERNS

- Business rules in a new `cmd/` file when an `internal/` package already owns the type
- Pass-through aliases that skip `zv` catalog/validation
- Reusing a failed recording `--out` (fresh namespace required)
