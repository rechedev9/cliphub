# Demo completa

Full Demo → 16:9 recap. Native HUD, team comms, no Shorts extras.

## Sub-features

- Sidebar `03 Demo completa` → `/full-demo`
- Picker of parsed matches → `/full-demo/{jobId}`
- Locked brief: landscape 16:9, native HUD, comms 85%, no music, no punch-in
- After recorded, auto 16:9 compose (PR #119)

## How to get to it (user POV)

Click **Demo completa**, pick a parsed match, read the contract rows, start the POV recap.

## What done looks like

A landscape recap exists in Biblioteca with PARTIDA COMPLETA, native HUD, team comms, no Shorts music bed. Hosted CI green is not this.

## Driving it with zv verify

```text
./bin/zv verify prove --feature demo-completa --job-id <uuid> --format json
./bin/zv verify prove --feature demo-completa --dry-run --format json
./bin/zv verify doctor --format json
```

King's Windows Studio is the host of record. This **fails closed** on Cloud Linux (`hlae_cs2_windows_studio`). HLAE/CS2 cannot be recertified here. Do not treat `go test` as a Pass. Do not call Full Demo Pass.

## Gotchas

- HLAE/CS2 required. Windowed capture only. Never `C:\HLAE\HLAE.exe`.
- This section does not share the Shorts brief.
- PR #119 merged the POV re-spec + auto 16:9 compose after recorded. Touching this flow still needs a real Studio/HLAE walk or a named gap.
- Unsigned installer. Actualizar reads `releases/latest`. Vercel is not the updater.
