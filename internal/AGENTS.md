# internal/

Go domain packages. Root `CLAUDE.md` owns product policy; this file is the package map.

## OVERVIEW

44 flat packages. Demo plan is durable JSON; recording/render consume it. Anticheat and tactical are side lanes and must not write `job.Status`. For the AI-agent-oriented target design, keep packages aligned with `docs/AI_AGENT_ARCHITECTURE.md`: durable plans are contracts, execution packages consume approved plans, and artifact/provenance state must be inspectable by a fresh agent session.

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
| Guided generate state | `generateintent` | Shared HTTP+worker store; record task gets an immutable copy |
| Artifact keys / FS root | `artifacts`, `storage`, `filecommit` | Keys only in `artifacts`; no I/O there |
| Tool detect (HLAE/CS2/ffmpeg) | `capturetools` | Same resolver for CLI and orchestrator |
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

## CONVENTIONS

- No nested Go packages. One directory = one package.
- Durable docs (`killplan`, `moments`, `streamclips.EditPlan`, `tacticalplan.Document`, `timelineplan.Document`) are the contracts later stages must honor.
- Agent-ready stage outputs should expose schema version, input refs, decision basis, effective config, safety gates, resume policy, QA status, and provenance when that stage is meant to be driven by CLI/API automation.
- `cmd/zv-orchestrator` owns SQLite repos + inline queue today; moving those into a narrow internal owner is an accepted behavior-preserving refactor seam, but do not create vague service layers.

## ANTI-PATTERNS

- Anticheat/tactical writing production job status
- Mixing HUD modes in one reel
- Editing `anticheat/baseline_default.json` sample counts by hand
- Importing `faceit` into `httpapi`/`workers`
