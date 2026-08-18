# cmd/zv

Unified CLI. Feature binaries remain behavioral owners; `zv` is the stable command surface. ~70 files.

## WHERE TO LOOK

| Task | Files |
|------|--------|
| Dispatch | `app.go`, `group_commands.go`, `delegation.go` |
| Workflow catalog | `workflow_catalog.go`, `workflows_commands.go`, `workflow_validation.go` |
| Demo/stream flows | `flow_commands.go`, `flow_run.go` |
| `short` orchestration | `short_command.go`, `short_prompt.go` |
| `zv check` | `check_commands.go`, `command_validation.go`, `skill_validation.go`, `doc_validation.go`, `agent_*.go` |
| Claude/Codex perms | `check_config.go` |
| Anticheat CLI | `anticheat_commands.go` → `internal/anticheat` |
| FACEIT index | `faceit_command.go` → `internal/faceit` |
| Demo review | `demo_review_commands.go`, `demo_probe.go`, `voice_commands.go` |
| Tactical CLI | `analysis_commands.go` |
| Record / stream wrappers | `record_command.go`, `stream_commands.go` |
| Public presets | `supported_presets.go`, `presets_command.go` |

Legacy pass-throughs (`parser`, `editor`, `recorder`, …) live in `check_config.go`. `zv-stream` and `zv-demo-players` have no raw-bin alias.

## CONVENTIONS

- Validate argv, keep `--dry-run --format json` until media is approved, then reuse that argv.
- `workflows run` / `flows run` stay media-free unless the user explicitly approved a live capture/render.
- `short --format json` requires `--dry-run`.
- Tests: `ZV_FAKE_SUBCOMMAND=1` stubs delegation; `--fake` is placeholder media; do not mix the two.

## ANTI-PATTERNS

- Inventing flags from README prose
- Resurrecting the retired ClipHub MCP server (`check` forbids it)
- Exposing `clean-pov-60` / `full-hud-60` on the public preset catalog
- Auto-retrying `demo_incompatible:`
- FACEIT key as a flag, or calling the unapproved Download API
