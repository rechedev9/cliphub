# Full Demo 16:9 wait

In-flight landscape recap card. Same capture lane, different brief.

## Sub-features

- Biblioteca `/videos` card with PARTIDA COMPLETA while recording/composing
- Same live `%` / current/total contract as Shorts while `recording`
- After recorded, auto 16:9 compose (PR #119)
- Compose stage is "Editando" / Montando cortes — no fake percent

## How to get to it (user POV)

Start a recap from **Demo completa**. Watch Biblioteca (and the shell wait) until the 16:9 file exists.

## What done looks like

A landscape ready card with native HUD recap. The wait never invents progress after capture. Hosted CI is not this walk.

This path **cannot be recertified on Cloud Linux**. HLAE/CS2 cannot launch here.

## Driving it with zv verify

```text
./bin/zv verify prove --feature full-demo-16x9-wait --format json
./bin/zv verify doctor --format json
```

Fails closed. Name the gap. Do not call Full Demo done from unit tests.

## Gotchas

- PR #120 overlay snapshot is still draft; do not merge until a Windows Studio walk.
- `recordAdmitted` vs FALLO applies here too.
- Full Demo does not share the Shorts brief (no music, no punch-in).
- Unsigned installer. Actualizar reads GitHub `releases/latest`. Vercel is not the updater.
- Structural flow (2): Full Demo → 16:9 recap. Unknown on this flow is a merge block.
