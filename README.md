# ClipHub — CS2 demo & stream reels, cut on your PC

<p align="center">
  <img src="web/public/brand/cliphub-mark.svg" alt="ClipHub" width="120" height="120">
</p>

<p align="center">
  <strong>Local-first CS2 highlight pipeline</strong><br>
  Parse demos → plan kills → HLAE/CS2 capture → FFmpeg/Lua render → publish pack
</p>

<p align="center">
  <a href="https://github.com/rechedev9/cliphub/releases"><img src="https://img.shields.io/github/v/release/rechedev9/cliphub?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="https://cliphub.gravityroom.app/"><img src="https://img.shields.io/badge/Website-cliphub.gravityroom.app-fb923c?style=for-the-badge" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-see%20repo-blue.svg?style=for-the-badge" alt="License"></a>
</p>

**ClipHub** is a Windows-local, deterministic pipeline that turns CS2 demos (and stream VODs) into vertical Shorts ready to post.

The **demo is the source of truth**. ClipHub does not invent kills from pixels or “AI vibes”; every recording decision comes from the `.dem` (or a persisted stream edit plan). Capture and render stay on your machine — there is no hosted SaaS backend to sign up for.

If you want a local creator rig for CS2 highlights that feels like a production tool, this is it.

[Website](https://cliphub.gravityroom.app/) · [Releases](https://github.com/rechedev9/cliphub/releases) · [Desktop](desktop/GUIDE.md) · [Web](web/GUIDE.md) · [Agents](CLAUDE.md)

---

## Install

Runtime for packaging: **Windows 10/11**, **Go 1.26.6**, **Node 24**, **pnpm 11.22**.

### End users (Studio)

Download the latest installer from [GitHub Releases](https://github.com/rechedev9/cliphub/releases):

```text
ClipHub.Studio.Setup.<version>.exe
```

Verify checksums with `SHA256SUMS.txt` in the same release. The installer is not code-signed yet — Windows SmartScreen may require **More info → Run anyway**.

### From source (developers)

```powershell
git clone https://github.com/rechedev9/cliphub.git
cd cliphub

# Go runtimes (zv, orchestrator, recorder, editor, …)
.\scripts\build.ps1

# Local Studio (Next + orchestrator)
.\scripts\local-studio.ps1
```

Desktop package:

```powershell
pnpm --dir web install
pnpm --dir desktop install
pnpm --dir desktop run dist   # rebuilds Go, assembles, writes SHA256SUMS
```

---

## Quick start (TL;DR)

### Studio

1. Install **ClipHub Studio** (or run `.\scripts\local-studio.ps1`).
2. Point Studio at CS2 + HLAE (auto-detect under `C:\HLAE-*\HLAE.exe`; never use `C:\HLAE\HLAE.exe` for capture).
3. Upload a `.dem` → pick a player → forge a reel.
4. Approve the creative brief (HUD, effects, music, covers).
5. Publish pack lands under the run’s `shortslistosparasubir/` folder.

### CLI (`zv`)

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe workflows list --format json

# One-shot short when player + selection policy are known
.\bin\zv.exe workflows run short -- match.dem --prompt "all kills 76561198000000000" --dry-run --format json
```

Treat `flows show` / `workflows show` as the executable contract. Prefer `--dry-run --format json` until real capture/render is approved.

---

## Pipeline

```text
Demo path
  .dem → parse/score → kill plan → HLAE/CS2 capture → FFmpeg/Lua → publish pack

Stream path
  stream video → edit plan → human review → stream render → pack
```

| Stage | What happens |
|--------|----------------|
| **Parse** | Demo ticks → players, kills, utility, rounds |
| **Plan** | Deterministic kill/moment selection (`killplan` / `moments`) |
| **Capture** | HLAE drives CS2 **windowed**; one capture lane per `cs2.exe` |
| **Render** | Effects (gopher-lua sandbox), variants, QA, composition |
| **Pack** | MP4 + cover + title/caption/hashtags from demo facts |

Also: **series jobs** (shared roster across maps), **FACEIT Data API** indexing (demos via user-authorized download only), and a side-lane **CheaterDetect** screen (anomaly report — never auto-report).

---

## Highlights

- **Local-first** — demos, captures, and renders stay on your PC.
- **Demo is truth** — no kill decisions from rendered video.
- **CLI + Studio** — same pipeline via `zv` or Electron UI.
- **Publish without AI copy** — titles/captions from demo facts; optional factual alternatives in Library.
- **Hard gates** — creative brief before non-dry-run capture/render; QA warnings block “upload-ready” until resolved.
- **Recovery-aware capture** — failed real capture can re-record; soft EOF clamp avoids last-tick glitches.
- **Windows-only capture** — HLAE + CS2 `-windowed` (no fullscreen/borderless).

---

## Operator quick refs

```powershell
# Contract discovery
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe flows show stream --format json
.\bin\zv.exe workflows show short --format json
.\bin\zv.exe presets --format json
.\bin\zv.exe skills list --format json

# Staged demo path (review player / plays first)
# demo players → demo parse → demo moments → demo select → record → shorts render

# CheaterDetect (side lane; never changes job status)
.\bin\zv.exe demo anticheat --help
```

Default kill/highlight deliverable: **one compiled vertical video** per player/game with all selected kills — not one file per kill.

Public preset catalog exposes `viral-60-clean` (death notices + `viral-ultra-clean` effects) and `viral-aggressive-60` (same HUD + `viral-aggressive` grade). HUD mode is a **recording-stage** choice; changing it after capture requires recapture.

---

## Docs by goal

| Goal | Doc |
|------|-----|
| Agent / contributor rules | [CLAUDE.md](CLAUDE.md) · [AGENTS.md](AGENTS.md) |
| CLI command contract | [.codex/GUIDE.md](.codex/GUIDE.md) |
| Desktop packaging & HLAE | [desktop/GUIDE.md](desktop/GUIDE.md) |
| Web / Studio UI | [web/GUIDE.md](web/GUIDE.md) · tokens in [web/app/globals.css](web/app/globals.css) |
| Operator workflow | [docs/cli-operator-workflow.md](docs/cli-operator-workflow.md) |

---

## Layout

```text
cmd/                 CLI binaries (zv, zv-orchestrator, zv-recorder, …)
internal/            parser, killplan, recording, editor, workers, httpapi, …
effects/             sandboxed gopher-lua effects (no FS / process)
web/                 Next.js 16 Studio UI
desktop/             Electron shell + installer
landing/             marketing site (Vercel, Next.js 15)
scripts/             build.ps1, local-studio.ps1, gates
data/                local artifacts (music catalog, …) — not source of truth
```

Toolchain sources of truth: **Go** (`go.mod` → `github.com/rechedev9/cliphub`), **pnpm 11.22** / **Node 24** per package.

Quality checks are local and explicit; see [CLAUDE.md](CLAUDE.md) for the affected-package commands. The one hosted job is [Desktop release](.github/workflows/desktop-release.yml) on `windows-latest`, which publishes the unsigned installer.

```powershell
.\scripts\build.ps1
go test ./... -count=1 -timeout 3m
& "C:\Program Files\Git\bin\bash.exe" scripts/go-gate.sh --no-format

pnpm --dir web run lint
pnpm --dir web run typecheck
pnpm --dir web run test:unit
pnpm --dir desktop run lint
pnpm --dir desktop run typecheck
pnpm --dir desktop run test:unit
```

---

## Configuration (developer)

| Env | Role |
|-----|------|
| `ZV_RECORDER_PATH` | `zv-recorder` |
| `ZV_HLAE_PATH` / auto-detect | HLAE under `C:\HLAE-*\HLAE.exe` |
| `ZV_CS2_PATH` | `cs2.exe` |
| `ZV_DATA_DIR` | job DB + artifacts |

Discover flags with `.\bin\zv.exe … --help` and `flows show`, not from prose alone.

Packaged Studio pins HLAE via `desktop/src/hlae-tool.json` (SHA-256 archive) — do not invent version numbers or use `C:\HLAE\HLAE.exe` for ClipHub capture.

---

## Security & privacy

- No hosted backend for Studio; orchestrator binds loopback.
- FACEIT credentials only in env / server-side secrets — never commit keys.
- **CheaterDetect** is a screening report with limitations; Valve decides bans. ClipHub never auto-submits reports or mass-report helpers.
- Capture is Windows-local; treat demos as sensitive match data.
- Effects scripts are sandboxed (no filesystem or process access).

---

## Releases

Versioned installers + `SHA256SUMS.txt` → [GitHub Releases](https://github.com/rechedev9/cliphub/releases). Actualizar reads `releases/latest`.

```powershell
pnpm --dir desktop run dist
pnpm --dir desktop run verify:dist-integrity
```

Publish flow: bump `desktop/package.json`, land on `main`, then run **Desktop release** (`workflow_dispatch`) or push `v<version>`. Landing/Vercel is not required for the in-app updater.

---

## Brand

**ClipHub** — demo **ticks** cut into publish-ready reels.

Canonical product surfaces: GitHub `rechedev9/cliphub`, site [cliphub.gravityroom.app](https://cliphub.gravityroom.app/).

---

## Community

Issues and discussion: [github.com/rechedev9/cliphub](https://github.com/rechedev9/cliphub).

PRs should stay on product scope: deterministic pipeline, Studio/CLI contract, and Windows capture safety. Prefer tests and explicit package checks over “looks good” claims.

Work lands on `main` (no PR required for product work on this repo). Verify affected packages locally before committing.
