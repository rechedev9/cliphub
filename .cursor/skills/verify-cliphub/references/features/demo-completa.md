# Vídeos largos

Full Demo → 16:9 recap. Native HUD, team comms, no Shorts extras.

## Sub-features

- `full-demo-rail` — Sidebar `04 Vídeos largos` → `/full-demo`.
- `full-demo-contract` — Locked brief rows on the index.
- `full-demo-picker` — Parsed matches → `/full-demo/{jobId}`.
- `full-demo-compose` — After recorded, auto 16:9 compose.

## How to get to it (user POV)

- Click **Vídeos largos**.
- Pick a parsed match, read the contract rows, start the POV recap.

## What done looks like

Header **VÍDEOS LARGOS**. Contract: landscape 16:9, native HUD, comms, no music. A landscape recap exists in Biblioteca with PARTIDA COMPLETA. Hosted CI green is not this.

This path **cannot be recertified on Cloud Linux**. HLAE/CS2 are required.

## Driving it with zv verify

Preconditions:

- Capture recertification: Windows Studio + HLAE + running `cs2.exe`, plus a grant.
- `--job-id` of the recap job.

- **Fail closed here.** `./bin/zv verify prove --feature demo-completa --format json` names `hlae_cs2_windows_studio` on a host that cannot recertify.
- **Dry-run.** `./bin/zv verify prove --feature demo-completa --dry-run --format json` — no HTTP, no capture enqueue.
- **Live inspect.** `./bin/zv verify prove --feature demo-completa --job-id <uuid> --format json` GETs `/api/jobs/{id}?view=status`. That is not Full Demo Pass.
- **UI.** Click **Vídeos largos**. Contract rows visible. Picker only lists parsed matches.

## Gotchas

- HLAE/CS2 required. Windowed capture only. Never `C:\HLAE\HLAE.exe`.
- This section does not share the Shorts brief (no music, no punch-in).
- A 9:16 pack must not be marked ready as Full Demo.
- Unsigned installer. Actualizar reads `releases/latest`. Vercel is not the updater.
