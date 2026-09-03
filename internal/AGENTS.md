# internal/

Go domain packages. Root `CLAUDE.md` owns product policy; this file is the package map.

## OVERVIEW

53 flat packages. Demo plan is durable JSON; recording/render consume it. Anticheat and tactical are side lanes and must not write `job.Status`. For the AI-agent-oriented target design, keep packages aligned with `docs/AI_AGENT_ARCHITECTURE.md`: durable plans are contracts, execution packages consume approved plans, and artifact/provenance state must be inspectable by a fresh agent session.

## WHERE TO LOOK

| Task | Package | Notes |
|------|---------|-------|
| Parse `.dem` → plan | `parser`, `rules`, `killplan` | `killplan.SchemaVersion=1.2`; reject unknown versions |
| Score / select plays | `moments` | Default variant `viral-60-clean` |
| HLAE/CS2 scripts + validate | `recording` | HUD is capture-time. Mutes demo voice (both teams on GOTV). Contract `observer-steamid-input-v2`; V1 read-only |
| Concat captured clips | `composition` | No re-edit; assumes `recording.SegmentClip` |
| Full Demo 16:9 overlays | `demooverlay` | Intro roster + outro scoreboard on native HUD; FACEIT optional |
| 9:16 shorts + pack | `editor`, `renderplan` | Public CLI presets: `viral-60-clean`, `viral-aggressive-60` |
| Kill→beat sync | `rhythm` | Editor applies it; workers do not call it |
| Stream/VOD plan + render | `streamclips`, `streamcli`, `vodfetch` | Persisted `EditPlan` is canonical |
| Multitrack editor | `mediaassets`, `timelineplan`, `timelinerender` | Persisted `timelineplan.Document` is canonical; preview evaluates the same stack FFmpeg composites |
| Local API | `httpapi` | Plus HTMX workbench assets |
| Job handlers | `workers`, `tasks`, `job` | One capture lane. Record `MaxRetry(0)` |
| SQLite/memory repositories, schema migrations | `store` | `OpenSQLite`/`NewMemory` return one `Repositories` bundle; schema is versioned in `migrate.go` (`PRAGMA user_version`, FKs on, one tx per step); never `ALTER` ad hoc in a constructor. Memory and SQLite share `contract_test.go`; a divergence is a bug, not a test fixture |
| Startup repair of interrupted work | `reconcile` | `InterruptedWork` runs before the HTTP server: fails queued/recording/composing jobs, queued/rendering render states, active generate runs, stream renders, editor renders; walks `job.Statuses()` so a new status is swept by construction |
| Guided generate state | `generateintent` | Shared HTTP+worker store; record task gets an immutable copy |
| Artifact keys / FS root | `artifacts`, `storage`, `filecommit` | Keys only in `artifacts`; no I/O there |
| Tool detect (HLAE/CS2/ffmpeg) | `capturetools` | Same resolver for CLI and orchestrator |
| Verification lever | `verify` | Windows-first `zv verify` doctor against live Studio; Linux fail-closes `hlae_cs2_windows_studio` |
| Folder parse (no queue) | `batch` | `zv batch` only; no Asynq |
| Pipeline errors | `obs` | Journal is authoritative; do not mutate in-memory counters |
| CheaterDetect | `anticheat` | No parser import. Anomaly report, never guilt |
| FACEIT index | `faceit` | CLI only. Key never serializes. Stats ≠ clip source |
| CS2 share code | `sharecode` | Offline decode only. Shape check in `web/lib/sharecode.ts` |
| CS2 Game Coordinator wire | `steamgc` | Encode/decode only. No Steam session, no network |
| Share code, history, fetch | `steamresolve` | Auth-code account + Web API walk + Valve CDN download. No go-steam |
| go-steam GC session | `steamclient` | Orchestrator only. Do not import from `httpapi` |
| Proto registration clash | `allowproto` | First internal import in `zv-orchestrator` |
| Tactical scan | `tactical`, `tacticalplan`, `radarmap` | `tacticalplan` must not import a parser |
| Smoke catalog | `lineups`, `utilityaudit` | Audit is CLI-only |
| Sponsor plate / font | `keydropbanner`, `mediafont` | Stream schema revalidates banner codes; do not import assets into it |
| Publish copy / trends | `youtubeinsights`, `youtubetrends` | Madrid TZ. Trends key redacted. No popularity claim |
| Voice refs | `voiceprofile` | HTTP only; no narration worker |
| Demo voice probe | `voicecomms` | Reads `svc_VoiceData`. Lists/extracts POV team only; overlay uses IngameTick |
| Path / CLI safety | `pathguard` | Blocks `--out` aliasing inputs |
| TUI DTOs | `tuiclient` | No parser/editor import |
| Full Demo round sidecar | `recapplan` | `match_recap` plan consumed by capture/overlay |
| `.dem.zst` upload unwrap | `demozstd` | Decompress before parse; no parser import |
| Studio telemetry events | `telemetry` | Privacy-minimized event contract; see `docs/TELEMETRY_RUNBOOK.md` |

## CONVENTIONS

- No nested Go packages. One directory = one package.
- Durable docs (`killplan`, `moments`, `streamclips.EditPlan`, `tacticalplan.Document`, `timelineplan.Document`) are the contracts later stages must honor.
- Agent-ready stage outputs should expose schema version, input refs, decision basis, effective config, safety gates, resume policy, QA status, and provenance when that stage is meant to be driven by CLI/API automation.
- `cmd/zv-orchestrator` still owns the inline queue; repositories live in `store` and startup sweeps in `reconcile`. Do not create vague service layers around them.
- Queue uniqueness is the logical scope from `tasks.UniqueScope` (one capture per job, one compose per job, one render per job+variant), not the payload bytes; admission that accepts work must claim it in the row (`recording`, `composing`, editor `rendering`) with a discard compensation.
- Every HTTP error body carries `code`; `service_unavailable` is reserved for the Studio proxy meaning "orchestrator unreachable" and must never be emitted by Go.

## ANTI-PATTERNS

- Anticheat/tactical writing production job status
- Mixing HUD modes in one reel
- Editing `anticheat/baseline_default.json` sample counts by hand
- Importing `faceit` into `httpapi`/`workers`
