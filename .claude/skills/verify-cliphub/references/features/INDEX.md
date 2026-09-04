# ClipHub Studio feature map

User-POV inventory the verification lever can name. Dump with:

```text
./bin/zv verify features --format json
```

Sidebar order lives in `web/lib/nav.ts`. `/` redirects to `/onboarding`.
The verification host of record for HLAE/CS2 rows is King's Windows Studio, not Cloud Linux.

## Baseline preconditions

- Cheap contract: repo root, `zv verify doctor --format json` parsed, named gaps accepted.
- Studio UI walk: live `ports.json` so `prove` emits `drive.open_url`. Click rail labels. Do not launch a second Studio against the same userData.
- Capture recertification: Windows + live Studio + HLAE (`C:\HLAE-*\HLAE.exe`, never `C:\HLAE\HLAE.exe`) + running `cs2.exe`, plus an explicit grant on that run.
- Keep `--dry-run` until capture/render is approved. Dry-run must not write `jobs.db` or enqueue capture.

## Driving conventions

- Start from the rail label in this INDEX unless the feature file names another door.
- Rail accessible names are uppercase (`INICIO`, `EDITOR`, `AJUSTES`). The `00`–`11` prefixes in `web/lib/nav.ts` are source order, not the clickable name.
- Prefer visible headings and ARIA names. Class selectors are fallbacks.
- `drive.open_url` is Studio loopback web + route. A Chrome tab is the Next.js UI, not Electron chrome.
- Wait on observable copy, not fixed sleeps.
- Offline orchestrator is `503 {code:"service_unavailable"}`, never a bad demo.
- Cheap `prove` GETs the catalog `probe_path` when Studio is up. Empty lists and gated `503 {code}` are success. That is not a UI walk and not a capture Pass.

## Proof and skip reporting

- Exercise every reachable entry point the change can affect, plus success / cancel / error / empty / persistence.
- Record the action and the final observable state.
- When a path is unreachable, name it, say whether grant, OS, HLAE/CS2, Steam, or FACEIT blocks it, and cover the closest real path that remains.
- Do not report a skipped entry point as verified through a different path.

## Full sweep

Walk this map top to bottom for a broad regression. Finish with [journeys.md](journeys.md).

### Rail

- [inicio](inicio.md): First-run door. `/onboarding`. Header EMPIEZA AQUÍ.
- [partidas](partidas.md): Match list and series. `/matches`.
- [subir-demo](subir-demo.md): Local `.dem` intake. `/upload`.
- [demo-completa](demo-completa.md): Full Demo → 16:9 recap. `/full-demo`. **gap** — HLAE/CS2.
- [tactica](tactica.md): Radar / round analysis. `/tactical`.
- [cheaterdetect](cheaterdetect.md): Side-lane anomaly screen. `/cheaters`.
- [jugadores](jugadores.md): FACEIT player index. `/players`.
- [clips-de-stream](clips-de-stream.md): VOD edit plan → render. `/streams`.
- [editor](editor.md): Multitrack montage of already-rendered MP4s. `/editor`.
- [biblioteca](biblioteca.md): Ready / in-flight / failed reel cards. `/videos`.
- [feed](feed.md): Community reel grid. `/feed`.
- [ajustes](ajustes.md): Version, telemetry, Steam account. `/settings`.

### In-flight waits (not rail items)

- [shorts-9x16-wait](shorts-9x16-wait.md): 9:16 capture overlay on Biblioteca. **gap** — live overlay.
- [full-demo-16x9-wait](full-demo-16x9-wait.md): 16:9 recap overlay on Biblioteca. **gap** — live overlay.

## Entry contract

Every catalog file starts with an H1 and one paragraph, then exactly these H2s:

1. `Sub-features`
2. `How to get to it (user POV)`
3. `What done looks like`
4. `Driving it with zv verify`
5. `Gotchas`
