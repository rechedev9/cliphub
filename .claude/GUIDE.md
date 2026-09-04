# Claude Code Guide

Guidance for using Claude Code safely on ClipHub, plus the unified `zv` CLI command contract that `zv check` enforces.

Claude Code automatically loads `CLAUDE.md` from the repo root.
That file holds project boundaries, Go and TypeScript style, safety rules, and verification expectations.
All style and operational rules live directly in `CLAUDE.md`.

Studio ships no assistant surface of its own; it is a GUI over the same pipeline.
Claude Code is a repository-development tool and is not part of the shipped product.
The former external MCP registration is no longer part of the product.

## Use

Run Claude Code from the repository root in Git Bash, not through the broken bare `bash` WSL shim:

```bash
claude
```

## Product operation

The unified Windows CLI is the primary interface. Studio is not a prerequisite.
Follow this machine-readable loop:

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe flows show stream --format json
.\bin\zv.exe workflows list --format json
.\bin\zv.exe workflows show short --format json
.\bin\zv.exe workflows validate short --format json -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
.\bin\zv.exe workflows run short -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
```

For a FACEIT profile, validate the exact request without network access, then
remove `--dry-run` to persist the current match/demo index:

```powershell
./bin/zv faceit index --profile https://www.faceit.com/en/players/m0NESY --from 2026-01-01 --to 2026-07-22 --out data/faceit/m0nesy-2026.json --dry-run --format json
./bin/zv workflows show faceit-index
./bin/zv workflows show faceit-index --format json
./bin/zv workflows validate faceit-index --format json -- --profile https://www.faceit.com/en/players/m0NESY --out data/faceit/m0nesy-2026.json --dry-run --format json
./bin/zv workflows run faceit-index -- --profile https://www.faceit.com/en/players/m0NESY --out data/faceit/m0nesy-2026.json --dry-run --format json
```

The real command reads only `FACEIT_API_KEY`. Until FACEIT approves Download
API access, open the persisted `room_url` values and download demos manually;
then continue with `demo players -> demo parse -> demo moments -> demo select`.
FACEIT statistics rank matches for review only; the `.dem` remains the source of
truth for every recording decision.

Run `.\scripts\build.ps1` first when `bin\zv.exe` is missing or stale. Keep
`--dry-run --format json` for planning; remove both flags only when the user
requested the real capture/render. Real execution streams human-readable stage
progress. Task-specific skills under `.claude/skills/` use the same CLI for
granular workflows and QA.

`flows show` is the first-stop journey guide: it exposes each decision boundary,
safe/dry-run command, expensive stage, artifact, and both delivery profiles.
For demos where the user chooses plays, use `demo players -> demo parse -> demo
moments -> demo select -> record -> shorts render`. For streams use `stream
fetch -> stream variants -> stream plan -> review -> stream render`. A local MP4 skips fetch.
Do not skip the selection/review boundary before HLAE or an expensive render
pass.

The demo journey also exposes two agent gates. `creative-brief` asks only for
unanswered format, HUD/killfeed, effect, transition, kill-numbering,
intro/outro, music, and thumbnail choices before expensive work.
`thumbnail-selection` applies when covers are enabled, shows generated
candidates, and requires a selection or an explicit delegation before the pack
is considered upload-ready. With `--covers=false`, there is no thumbnail gate.

The JSON dry-run is one resolved document with `executed: false`, exact stage
argv, and output paths. Real `short` and `record` calls auto-fill missing
HLAE/CS2 paths from the same detection shown by `capabilities`; do not repeat
those paths in agent-generated commands unless overriding detection.
Detection selects the highest installed numeric HLAE version, and capture work
must keep it aligned with the latest official AdvancedFX release.
The `output.publish_dir` field points to the required upload-ready
`<run>\shortslistosparasubir` folder; `output.shorts_dir` contains intermediates.

The former external ClipHub MCP registration has been removed. Use the
unified `zv` CLI for repository workflows, or drive Studio through its own
interface.

## Safety defaults

- `.claude/settings.json` runs with `defaultMode: bypassPermissions` and blanket `Bash(*)`/`Read(*)`/`Edit(*)`/`Write(*)` allows: no prompts, no ask list, no deny list. `zv check` enforces exactly that shape.
- Nothing in the harness blocks secret reads or destructive Git/filesystem commands; the guard rails are the rules in `CLAUDE.md` (credentials only in env, commit/push only on explicit request, never persist Steam credentials).
- `scripts/go-gate.sh` formats changed Go files unless `--no-format` is passed.
  In a very dirty repo, use `--no-format` or format explicit files first.
- `scripts/go-gate.sh` also runs `zv check`, so repo-local skills,
  the workflow catalog, and workflow docs stay aligned with the unified CLI
  contract.
- `scripts/fix-loop.ps1` runs the same project check on Windows.
- `make test` runs the same project check for Unix-like local loops.

## Local checks

```bash
scripts/go-tools-check.sh
scripts/go-gate.sh --no-format
scripts/go-gate.sh --race --security --build
```

`go-tools-check.sh` verifies optional tools:

- `goimports`
- `staticcheck`
- `govulncheck`
- `gosec`

## Project skills

Repo-local skills live under `.claude/skills/`. Claude Code loads every one of them; `zv skills` catalogs and validates all of them except skills whose frontmatter carries `metadata.zv-catalog: "false"`:

- `zackvideo-cheater-pov-reels`: create suspected-cheater reels by pairing killer POV before each target death.
- `zackvideo-cs2-utility-shorts`: parse, audit, record, render, and review CS2 utility Shorts.
- `zackvideo-lineup-audit`: correct utility destinations through manual lineup catalogs.
- `zackvideo-music-scripted-shorts`: create 24fps Lua-scripted Shorts with CC0 music and rhythm sync.
- `zackvideo-shorts-production`: generate, polish, and QA professional CS2 Shorts packs.
- `zackvideo-stream-clips`: plan and render stream VOD clips after the creative brief gate.
- `zackvideo-youtube-shorts-publish`: review publish packs, prepare YouTube Shorts metadata, and guide manual publication in YouTube Studio.

`frontend-design` and `verify-cliphub` are Claude Code skills outside the `zv` catalog.

The unified CLI can discover the same repo-local skills:

```bash
./bin/zv skills list
./bin/zv skills show zackvideo-cheater-pov-reels
./bin/zv skills show zackvideo-cs2-utility-shorts
./bin/zv skills show zackvideo-lineup-audit
./bin/zv skills show zackvideo-music-scripted-shorts
./bin/zv skills show zackvideo-shorts-production
./bin/zv skills show zackvideo-stream-clips
./bin/zv skills show zackvideo-youtube-shorts-publish
./bin/zv skills check
./bin/zv check
./bin/zv check --format json
./bin/zv skills list --format json
./bin/zv skills show zackvideo-cheater-pov-reels --format json
./bin/zv skills show zackvideo-cs2-utility-shorts --format json
./bin/zv skills show zackvideo-lineup-audit --format json
./bin/zv skills show zackvideo-music-scripted-shorts --format json
./bin/zv skills show zackvideo-shorts-production --format json
./bin/zv skills show zackvideo-stream-clips --format json
./bin/zv skills show zackvideo-youtube-shorts-publish --format json
./bin/zv skills check --format json
./bin/zv workflows list
./bin/zv workflows list --format json
./bin/zv flows list --format json
./bin/zv flows show demo --format json
./bin/zv flows show stream --format json
./bin/zv workflows show short
./bin/zv workflows show short --format json
./bin/zv workflows show capabilities
./bin/zv workflows show capabilities --format json
./bin/zv workflows show demo-parse
./bin/zv workflows show demo-parse --format json
./bin/zv workflows show demo-players
./bin/zv workflows show demo-players --format json
./bin/zv workflows show demo-moments
./bin/zv workflows show demo-moments --format json
./bin/zv workflows show demo-select
./bin/zv workflows show demo-select --format json
./bin/zv workflows show demo-probe
./bin/zv workflows show demo-probe --format json
./bin/zv workflows show demo-voice
./bin/zv workflows show demo-voice --format json
./bin/zv workflows show utility-audit
./bin/zv workflows show utility-audit --format json
./bin/zv workflows show record
./bin/zv workflows show record --format json
./bin/zv workflows show compose-final
./bin/zv workflows show compose-final --format json
./bin/zv workflows show music-analyze
./bin/zv workflows show music-analyze --format json
./bin/zv workflows show shorts-render
./bin/zv workflows show shorts-render --format json
./bin/zv workflows show stream-fetch
./bin/zv workflows show stream-fetch --format json
./bin/zv workflows show stream-variants
./bin/zv workflows show stream-variants --format json
./bin/zv workflows show stream-plan
./bin/zv workflows show stream-plan --format json
./bin/zv workflows show stream-render
./bin/zv workflows show stream-render --format json
./bin/zv workflows show analysis-tactical
./bin/zv workflows show analysis-tactical --format json
./bin/zv workflows show analysis-rounds
./bin/zv workflows show analysis-rounds --format json
./bin/zv workflows show analysis-tendencies
./bin/zv workflows show analysis-tendencies --format json
./bin/zv workflows show analysis-tactical-data
./bin/zv workflows show analysis-tactical-data --format json
./bin/zv workflows show analysis-viewer
./bin/zv workflows show analysis-viewer --format json
./bin/zv workflows show gallery-open
./bin/zv workflows show gallery-open --format json
./bin/zv workflows show serve
./bin/zv workflows show serve --format json
./bin/zv workflows show skills-check
./bin/zv workflows show skills-check --format json
./bin/zv workflows show workflows-check
./bin/zv workflows show workflows-check --format json
./bin/zv workflows show project-check
./bin/zv workflows show project-check --format json
./bin/zv workflows show flows-run
./bin/zv workflows show flows-run --format json
./bin/zv workflows validate short --format json -- testdata/foo.dem --prompt "all kills 76561198000000000" --dry-run
./bin/zv workflows validate demo-parse --format json -- --demo testdata/foo.dem --steamid 76561198000000000 --segment-mode utility --out plan.json
./bin/zv workflows validate record --format json -- --killplan plan.json --demo testdata/foo.dem --out data/runs/run-004/recording --dry-run --hud deathnotices
./bin/zv workflows validate stream-plan --format json -- --input stream.mp4 --out data/runs/stream/edit-plan.json --dry-run
./bin/zv workflows validate stream-render --format json -- --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --dry-run
./bin/zv short testdata/foo.dem --prompt "all kills 76561198000000000" --dry-run
./bin/zv capabilities --format json
./bin/zv demo parse --demo testdata/foo.dem --steamid 76561198000000000 --out plan.json
./bin/zv demo players --demo testdata/foo.dem
./bin/zv demo probe --demo testdata/foo.dem --out data/runs/agent-doc/playability.json --format json --dry-run
./bin/zv demo voice --demo testdata/foo.dem --steamid 76561198000000000 --out data/runs/agent-doc/voice-probe.json --format json --dry-run
./bin/zv demo moments --killplan testdata/agent-killplan.json --format json
./bin/zv demo select --killplan testdata/agent-killplan.json --segments seg-001 --out data/runs/agent-doc/selected-plan.json --dry-run --format json
./bin/zv utility audit --plan plan-utility.json --lineup-catalog data/lineups --out utility-audit.csv
./bin/zv record --killplan plan.json --demo testdata/foo.dem --out data/runs/run-004/recording
./bin/zv compose final --recording-result data/runs/run-004/recording/recording-result.json --out data/runs/run-004/final.mp4
./bin/zv music analyze --input data/music/track.mp4 --out data/runs/run-004/rhythm.json
./bin/zv shorts render --recording-result data/runs/run-004/recording/recording-result.json --out data/runs/run-004/shorts --publish-dir data/runs/run-004/shortslistosparasubir
./bin/zv stream fetch --url https://www.twitch.tv/videos/123456789 --out data/runs/stream/source.mp4 --dry-run
./bin/zv stream variants
# Independent preflight examples; these do not create their --out artifacts.
./bin/zv stream plan --input stream.mp4 --out data/runs/stream/edit-plan.json --dry-run
./bin/zv stream render --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --dry-run
# Persist the approved stream chain in order before the real render.
./bin/zv stream plan --input stream.mp4 --out data/runs/stream/edit-plan.json
./bin/zv stream render --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream
./bin/zv analysis tactical --demo match.dem --out data/analysis/match-tactical.json --positions data/analysis/match-positions.zvpos --dry-run --format json
./bin/zv analysis rounds --tactical testdata/agent-tactical.json --side T --format json
./bin/zv analysis tendencies --tactical testdata/agent-tactical.json --team t-start --format json
./bin/zv analysis tactical-data --demo testdata/foo.dem --out data/runs/run-004/tactical.json --start 1000 --end 2000
./bin/zv analysis view --json data/analysis/MarcusN1-deaths.json
./bin/zv gallery open --path data/runs/run-004/shortslistosparasubir/index.html
# Chain the whole demo journey safely; --killplan skips parsing, while --demo remains required for capture validation.
./bin/zv flows run demo --demo testdata/agent-demo.fixture --killplan testdata/agent-killplan.json --run-dir data/runs/agent-doc --dry-run --format json
# The stream journey form needs real media, so it is documented here in prose only:
# "zv flows run stream --input <stream.mp4> --run-dir <dir> --dry-run" chains the
# creative gate, plan, and render phases over the persisted edit plan.
./bin/zv serve
./bin/zv workflows run short -- testdata/foo.dem --prompt "all kills 76561198000000000" --dry-run
./bin/zv workflows run capabilities -- --format json
./bin/zv workflows run demo-parse -- --demo testdata/foo.dem --steamid 76561198000000000 --out plan.json
./bin/zv workflows run demo-players -- --demo testdata/foo.dem
./bin/zv workflows run demo-moments -- --killplan testdata/agent-killplan.json --format json
./bin/zv workflows run demo-select -- --killplan testdata/agent-killplan.json --segments seg-001 --out data/runs/agent-doc/selected-plan.json --dry-run --format json
./bin/zv workflows run demo-probe -- --demo testdata/foo.dem --out data/runs/agent-doc/playability.json --dry-run --format json
./bin/zv workflows run demo-voice -- --demo testdata/foo.dem --steamid 76561198000000000 --out data/runs/agent-doc/voice-probe.json --dry-run --format json
./bin/zv workflows run utility-audit -- --plan plan-utility.json --lineup-catalog data/lineups --out utility-audit.csv
./bin/zv workflows run record -- --killplan plan.json --demo testdata/foo.dem --out data/runs/run-004/recording
./bin/zv workflows run compose-final -- --recording-result data/runs/run-004/recording/recording-result.json --out data/runs/run-004/final.mp4
./bin/zv workflows run music-analyze -- --input data/music/track.mp4 --out data/runs/run-004/rhythm.json
./bin/zv workflows run shorts-render -- --recording-result data/runs/run-004/recording/recording-result.json --out data/runs/run-004/shorts --publish-dir data/runs/run-004/shortslistosparasubir
./bin/zv workflows run stream-fetch -- --url https://www.twitch.tv/videos/123456789 --out data/runs/stream/source.mp4 --dry-run
./bin/zv workflows run stream-variants
./bin/zv workflows run stream-plan -- --input stream.mp4 --out data/runs/stream/edit-plan.json --dry-run
./bin/zv workflows run stream-render -- --input stream.mp4 --plan data/runs/stream/edit-plan.json --out data/runs/stream --dry-run
./bin/zv workflows run analysis-tactical -- --demo match.dem --out data/analysis/match-tactical.json --positions data/analysis/match-positions.zvpos --dry-run --format json
./bin/zv workflows run analysis-rounds -- --tactical testdata/agent-tactical.json --side T --format json
./bin/zv workflows run analysis-tendencies -- --tactical testdata/agent-tactical.json --team t-start --format json
./bin/zv workflows run analysis-tactical-data -- --demo testdata/foo.dem --out data/runs/run-004/tactical.json --start 1000 --end 2000
./bin/zv workflows run analysis-viewer -- --json data/analysis/MarcusN1-deaths.json
./bin/zv workflows run gallery-open -- --path data/runs/run-004/shortslistosparasubir/index.html
./bin/zv workflows run flows-run -- demo --demo testdata/agent-demo.fixture --killplan testdata/agent-killplan.json --run-dir data/runs/agent-doc --dry-run --format json
./bin/zv workflows run serve
./bin/zv workflows run skills-check
./bin/zv workflows run skills-check -- --format json
./bin/zv workflows run workflows-check
./bin/zv workflows run workflows-check -- --format json
./bin/zv workflows run project-check
./bin/zv workflows run project-check -- --format json
./bin/zv workflows check
./bin/zv workflows check --format json
```

`zv check` is the full project contract. It validates repo-local skills, the
workflow catalog, and the active docs/scripts that document the CLI.
`workflows list --format json` and `workflows show <name> --format json`
include `command` for the direct canonical command, `run_command` for execution,
and `validate_command` for zero-side-effect preflight. The `arguments` object
describes positionals, every required/value/boolean flag, conditional
requirements, and enum-like allowed values/defaults; `safety` describes
read-only, dry-run, and long-running behavior. These fields are derived from the
same command contract enforced at execution time, so an agent does not need to
guess from prose or probe the CLI with invalid calls. A JSON preflight always
reports `scope: "arguments"` and `executed: false`; runtime tool/file readiness
is intentionally outside that claim.
