# TickCut — CS2 demo & stream reels, forged on your PC

<p align="center">
  <img src="web/public/brand/tickcut-mark.svg" alt="TickCut" width="96" height="96">
</p>

<p align="center">
  <a href="https://github.com/rechedev9/tickcut/releases"><img src="https://img.shields.io/github/v/release/rechedev9/tickcut?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="https://fragforge.gravityroom.app/"><img src="https://img.shields.io/badge/Website-fragforge.gravityroom.app-22d9ee?style=for-the-badge" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-see%20repo-blue.svg?style=for-the-badge" alt="License"></a>
</p>

**TickCut** is a Windows-local, deterministic pipeline that turns CS2 demos (and stream VODs) into vertical Shorts ready to post — parse → kill plan → HLAE/CS2 capture → FFmpeg/Lua render → publish pack.

The **demo is the source of truth**. TickCut does not invent kills from pixels or “AI vibes”; every recording decision comes from the `.dem` (or a persisted stream edit plan).

If you want a local creator rig for CS2 highlights that feels like a production tool — not a hosted SaaS — this is it.

[Website](https://fragforge.gravityroom.app/) · [Releases](https://github.com/rechedev9/tickcut/releases) · [Product notes](PRODUCT.md) · [Desktop guide](desktop/GUIDE.md) · [Web guide](web/GUIDE.md)

---

## Install (Studio)

Runtime for packaging: **Windows 10/11**, **Go 1.26+**, **Node 24**, **pnpm 11.9**.

### End users

Download the latest installer from [GitHub Releases](https://github.com/rechedev9/tickcut/releases):

```text
TickCut.Studio.Setup.<version>.exe
```

Verify checksums with `SHA256SUMS.txt` in the same release. The installer is not code-signed yet — Windows SmartScreen may require “More info → Run anyway”.

### From source (developers)

```powershell
git clone https://github.com/rechedev9/tickcut.git
cd tickcut

# Go runtimes (zv, orchestrator, recorder, editor, …)
.\scripts\build.ps1

# Local Studio (Next + orchestrator)
.\scripts\local-studio.ps1
```

Desktop package:

```powershell
pnpm --dir web install
pnpm --dir desktop install
pnpm --dir desktop run dist   # rebuilds Go, assembles, signs nothing, writes SHA256SUMS
```

CLI-first production (no Studio required):

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe workflows list --format json
```

Treat `flows show` / `workflows show` as the executable contract. Prefer `--dry-run --format json` until real capture/render is approved.

---

## Quick start (TL;DR)

1. Install **TickCut Studio** (or run `local-studio.ps1`).
2. Point Studio at CS2 + HLAE (auto-detect; never use `C:\HLAE\HLAE.exe` for capture).
3. Upload a `.dem` → pick a player → forge a reel.
4. Approve the creative brief (HUD, effects, music, covers).
5. Publish pack lands under the run’s `shortslistosparasubir/` folder.

Stream path:

```text
stream video → edit plan → human review → stream render → pack
```

Demo path:

```text
.dem → parse/score → kill plan → HLAE/CS2 capture → FFmpeg/Lua → pack
```

---

## What it does

| Stage | What happens |
|--------|----------------|
| **Parse** | Demo ticks → players, kills, utility, rounds |
| **Plan** | Deterministic kill/moment selection (`killplan` / `moments`) |
| **Capture** | HLAE drives CS2 windowed; one capture lane per `cs2.exe` |
| **Render** | Effects (gopher-lua sandbox), variants, QA, composition |
| **Pack** | MP4 + cover + title/caption/hashtags from demo facts |

Also: series jobs, FACEIT indexing (Data API only — demos via user-authorized download), and a side-lane **CheaterDetect** screen (anomaly report, never auto-report).

---

## Highlights

- **Local-first** — demos, captures, and renders stay on your machine.
- **Demo is truth** — no kill decisions from rendered video.
- **CLI + Studio** — same pipeline via `zv` or Electron UI.
- **Publish without AI copy** — titles/captions from demo facts; optional factual alternatives in Library.
- **Hard gates** — creative brief before non-dry-run capture/render; QA warnings block “upload-ready” until resolved.
- **Windows-only capture** — HLAE + CS2 `-windowed` (no fullscreen/borderless).

---

## Docs by goal

| Goal | Doc |
|------|-----|
| Product intent | [PRODUCT.md](PRODUCT.md) |
| Agent / contributor rules | [CLAUDE.md](CLAUDE.md) / [AGENTS.md](AGENTS.md) |
| Desktop packaging & HLAE | [desktop/GUIDE.md](desktop/GUIDE.md) |
| Web / Studio UI | [web/GUIDE.md](web/GUIDE.md), [web/design.md](web/design.md) |
| FACEIT indexing | [FACEIT_GUIDE.md](FACEIT_GUIDE.md) |

---

## Layout

```text
cmd/                 thin CLI entrypoints (zv, zv-orchestrator, zv-recorder, …)
internal/            parser, killplan, recording, editor, workers, httpapi, …
effects/             sandboxed gopher-lua effects (no FS / process)
web/                 Next.js 15 Studio UI
desktop/             Electron shell + installer
landing/             marketing site (Vercel)
scripts/             build.ps1, local-studio.ps1, gates
```

Toolchain: **Go** (`go.mod`), **pnpm 11.9** / **Node 24** per package. No hosted CI — [`.githooks/pre-commit`](.githooks/pre-commit) is the gate.

---

## Security & privacy

- No hosted backend for Studio; orchestrator binds loopback.
- FACEIT credentials only in env / server-side secrets — never commit keys.
- CheaterDetect is a screening report with limitations; Valve decides bans. TickCut never auto-submits reports or mass-report helpers.
- Capture is Windows-local; treat demos as sensitive match data.

---

## Releases

Versioned installers + `SHA256SUMS.txt` → [GitHub Releases](https://github.com/rechedev9/tickcut/releases).

Landing download URL is updated per release at [fragforge.gravityroom.app](https://fragforge.gravityroom.app/) (DNS name may lag the product rename).

```powershell
pnpm --dir desktop run dist
pnpm --dir desktop run verify:dist-integrity
```

---

## Configuration (developer)

Orchestrator tools (examples):

| Env | Role |
|-----|------|
| `ZV_RECORDER_PATH` | `zv-recorder` |
| `ZV_HLAE_PATH` / auto-detect | HLAE under `C:\HLAE-*\HLAE.exe` |
| `ZV_CS2_PATH` | `cs2.exe` |
| `ZV_DATA_DIR` | job DB + artifacts |

Discover flags with `.\bin\zv.exe … --help` and `flows show`, not from prose alone.

---

## Brand

**TickCut** — demo **ticks** cut into publish-ready reels.

Formerly developed as FragForge; the product brand is now TickCut.

---

## Community

Issues and discussion: [github.com/rechedev9/tickcut](https://github.com/rechedev9/tickcut).

PRs should stay on product scope: deterministic pipeline, Studio/CLI contract, and Windows capture safety. Prefer tests and the pre-commit gate over “looks good” claims.
