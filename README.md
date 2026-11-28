# ClipHub 🎬: Counter-Strike 2 demo in, edited video out

[![Release](https://img.shields.io/github/v/release/rechedev9/cliphub?style=flat-square)](https://github.com/rechedev9/cliphub/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/rechedev9/cliphub?style=flat-square)](go.mod)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6?style=flat-square)

ClipHub Studio is a Windows desktop app for Counter-Strike 2 players and creators. Give it a demo and it finds your plays, records them inside the real game and edits them into a vertical Short or a long 16:9 video. It is Luis Reche's own product, shared with the agency [SocialPro](https://socialpro.es). It has 100+ users and 130+ [releases](https://github.com/rechedev9/cliphub/releases). The app interface is in Spanish.

![ClipHub site](docs/assets/cliphub.webp)

Live site: [cliphub.rechedev.cloud](https://cliphub.rechedev.cloud/)

## Install

1. Download `ClipHub.Studio.Setup.<version>.exe` from the [latest release](https://github.com/rechedev9/cliphub/releases/latest). Each release also lists `SHA256SUMS.txt`.
2. Run it. The installer is unsigned, so Windows SmartScreen may show "Windows protected your PC". Choose **More info**, then **Run anyway**.
3. Open ClipHub Studio. The app fetches the tools it needs on its own: HLAE, FFmpeg and yt-dlp.

Requirements:

- Windows 10 or 11, 64-bit.
- Steam with Counter-Strike 2 installed. Capture runs CS2 on your own machine, so a dedicated GPU gives the cleanest result.
- About 1 GB of free disk space.

The app updates itself from the latest GitHub Release.

## Quick start

1. Open the section for the format you want: **Shorts** (9:16) or **Demo completa** (16:9).
2. Add the match. Drop a `.dem`, `.dem.zst`, `.zip` or `.rar` file (up to 10 demos at once), or paste a `CSGO-` share code and download the demo from Steam.
3. Pick the player. ClipHub reads the demo and lists the ten players with rating, ADR, KAST and HS %.
4. For a Short, work through four steps: plays, style, music, text and graphics. **Auto: mejores 60 s** picks the best minute for you.
5. Create the video. Studio opens CS2, records the plays through HLAE and edits them together.
6. Open the finished video in **Resultados**. The publish assistant prepares title, description and tags, and lets you download the MP4 and open YouTube Studio to upload it yourself.

## What you can do

| Section in the app | What it does |
| --- | --- |
| Shorts | Make vertical Shorts from a demo. |
| Demo completa | Make a full-match video from a demo. Several demos of one best-of are grouped as a series. |
| Resultados | Every video you made, with download and the publish assistant. |
| Clips de stream | Cut moments from a Twitch, YouTube or Kick link or an MP4 into vertical Shorts, with facecam framing. |
| Jugadores | Follow FACEIT players and turn their matches into clips. |
| Replay | Round classification, the replay of every round on the map and tendencies, all derived from the demo. |
| Ajustes | Get this PC ready to record and manage optional connections such as Steam and diagnostics. |

Long videos cover every round from one player's view, with a HUD, team voices, round transitions and optional intro, sponsor and outro clips. Shorts add beat-synced cuts, kill effects and transitions.

## How it works

A demo goes through three stages. Everything runs on your PC.

| Stage | What happens | Code |
| --- | --- | --- |
| Parse | Reads the demo tick by tick and builds a kill plan for the chosen player. | `cmd/zv-parser`, `internal/parser` |
| Record | Launches CS2 with HLAE and captures each play from the player's view. Windows only. | `cmd/zv-recorder`, `internal/recording` |
| Edit | Analyses the music for beats, then renders the video and the publish pack with FFmpeg. | `cmd/zv-rhythm`, `cmd/zv-editor`, `internal/editor` |

`zv-orchestrator` queues and runs these jobs behind a local API. The Next.js UI in `web/` talks to it, and the Electron shell in `desktop/` bundles both into the installer.

After a source build, `bin\zv.exe` runs the same pipeline from the command line:

```powershell
.\bin\zv.exe short <demo.dem> --prompt "<instruction>" --output-format short-9x16
```

Run `.\bin\zv.exe --help` for the other commands, or `zv workflows list` through the same binary for the named workflows.

## Run from source

Requirements: Go 1.26.10 (from `go.mod`), Node 26, the pnpm pinned in the root `package.json`, and Steam with CS2 for capture.

```powershell
.\scripts\build.ps1                                   # Go binaries into bin\
pnpm install --frozen-lockfile                       # one root workspace and lockfile
pnpm --dir desktop run build
pnpm --dir desktop run assemble                       # stages bin\, the web build and HLAE
$env:CLIPHUB_E2E_USER_DATA = "C:\temp\cliphub-try\$(Get-Date -f yyyyMMdd-HHmmss)"
pnpm --dir desktop exec electron scripts\isolated-userdata.cjs
```

Always start with a throwaway profile as above, never your real `%APPDATA%\cliphub-studio`. A new profile shows the first-run guide. Use demo files from outside the repo. `scripts/local-studio.ps1` is an alternative that runs the orchestrator and the web UI without Electron.

Repository layout:

| Path | Contents |
| --- | --- |
| `cmd/`, `internal/` | Go backend: orchestrator, parser, recorder, editor |
| `web/` | Studio UI (Next.js) |
| `desktop/` | Electron shell and installer build |
| `landing/` | Separate Next.js site |
| `mcp-apps/` | Views the Studio MCP server shows inside an assistant |
| `services/`, `deploy/` | The community API and the telemetry collector, and the server deploy files |
| `tools/`, `content/` | Operator tools and the owner's own YouTube pipeline; neither ships in the installer |

[AGENTS.md](AGENTS.md) has the full layout and how the pieces talk.

## Checks

```bash
go vet ./...
go test ./... -count=1 -timeout 30m
go run ./cmd/zv check
pnpm run check                                        # every package's typecheck, lint and test:unit, then zv check
```

`make check` runs `pnpm run check`; `make test` runs the Go tests and `zv check`; `make build` builds the binaries. A few Full Demo editor tests depend on the FFmpeg version, and Studio ships FFmpeg 8.1.2. There is no CI on pushes or pull requests; the release workflow runs the checks before it publishes, so run the checks for what you touched before you push. [AGENTS.md](AGENTS.md) describes how to verify a change by hand in the real app.

## Releases

Bump `desktop/package.json` and `landing/lib/site.ts`, commit `chore(release): prepare Studio X.Y.Z`, then push an annotated tag `vX.Y.Z`. The `desktop-release.yml` workflow builds the unsigned Windows installer and publishes the GitHub Release that the in-app updater reads.

## Documentation

| File | Topic |
| --- | --- |
| [AGENTS.md](AGENTS.md) | The index: products, layout, build, checks, golden rules, rules and skills |
| [docs/incidents.md](docs/incidents.md) | Domain rules and the incidents behind them |
| [docs/studio-mcp.md](docs/studio-mcp.md) | The Studio MCP server and its tools |
| [docs/remote-debug-tracing.md](docs/remote-debug-tracing.md) | Remote error events, debugging and trace delivery |
| [docs/adr/](docs/adr/) | Architecture decisions |
| [docs/product-vision.md](docs/product-vision.md) | Product vision (Spanish) |
