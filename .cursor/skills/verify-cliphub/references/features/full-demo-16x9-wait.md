# Full Demo 16:9 wait

In-flight landscape recap card. Same capture lane, different brief.

## Sub-features

- `recap-wait-card` — Biblioteca `/videos` card with PARTIDA COMPLETA while recording/composing.
- `recap-wait-percent` — Same live `%` / current/total contract as Shorts while `recording`.
- `recap-wait-compose` — After recorded, auto 16:9 compose.
- `recap-wait-editando` — Compose stage is "Editando" / Montando cortes — no fake percent.

## How to get to it (user POV)

- Start a recap from **Demo completa**.
- Watch Biblioteca (and the shell wait) until the 16:9 file exists.

## What done looks like

A landscape ready card with native HUD recap. The wait never invents progress after capture. Hosted CI is not this walk.

This path **cannot be recertified on Cloud Linux**. HLAE/CS2 cannot launch here.

## Driving it with zv verify

Preconditions:

- Capture recertification: Windows Studio + HLAE + running `cs2.exe`, plus a grant.
- `--job-id` of the recap job.

- **Fail closed here.** `./bin/zv verify prove --feature full-demo-16x9-wait --format json` names `hlae_cs2_windows_studio` on Cloud Linux.
- **Dry-run.** `./bin/zv verify prove --feature full-demo-16x9-wait --dry-run --format json`.
- **Live inspect.** `./bin/zv verify prove --feature full-demo-16x9-wait --job-id <uuid> --format json`. Inspect `status` and `progress.percent`. Not Full Demo Pass.
- **Doctor.** `./bin/zv verify doctor --format json`.

## Gotchas

- PR #120 overlay snapshot is still draft; do not merge until a Windows Studio walk.
- `recordAdmitted` vs FALLO applies here too.
- Full Demo does not share the Shorts brief (no music, no punch-in).
- EDITANDO must not snap to `0 / N` during concat.
- Structural flow (2): Full Demo → 16:9 recap. Unknown on this flow is a merge block.
