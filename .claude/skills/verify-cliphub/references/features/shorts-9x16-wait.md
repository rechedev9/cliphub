# 9:16 Shorts wait

In-flight vertical reel while HLAE/CS2 holds the single capture lane.

## Sub-features

- `shorts-wait-card` — Biblioteca `/videos` card while `status === 'recording'` or queued.
- `shorts-wait-percent` — Live `%` from orchestrator `progress.percent` (0–100 of planned ticks).
- `shorts-wait-counts` — `current/total` copy: `Capturando {done+1}/{total}`.
- `shorts-wait-queue` — Queued copy: Esperando turno de captura (one `cs2.exe`).

## How to get to it (user POV)

- From a match, start a 9:16 Short.
- Stay on Biblioteca (or the shell overlay) while capture runs.

## What done looks like

The wait shows a real percent and a current/total that match the job poll. Editing after capture is indeterminate (no fake percent). A successful capture becomes a compose/ready card, not a silent stall.

This path **cannot be recertified on Cloud Linux**. HLAE/CS2 are required.

## Driving it with zv verify

Preconditions:

- Capture recertification: Windows Studio + HLAE + running `cs2.exe`, plus a grant.
- `--job-id` of the recording job.

- **Fail closed here.** `./bin/zv verify prove --feature shorts-9x16-wait --format json` names `hlae_cs2_windows_studio` when this host cannot recertify.
- **Dry-run.** `./bin/zv verify prove --feature shorts-9x16-wait --dry-run --format json` — planned GET only, no HTTP.
- **Live inspect.** `./bin/zv verify prove --feature shorts-9x16-wait --job-id <uuid> --format json`. Overlay percent is `GET /api/jobs/{id}?view=status` → `progress.percent`. `user_path` is `inspected`, never `pass`.
- **Doctor.** `./bin/zv verify doctor --format json` before the walk.

Cheap proof: `web/lib/capture-progress.ts` and `shell-activity` unit tests. Do not fake a Pass.

## Gotchas

- PR #120 (live `%` + current/total on Studio waits) is still draft. Hosted CI green is not a Windows Studio overlay walk. Do not merge #120 until that walk.
- `recordAdmitted` vs FALLO: an admitted record must stay in capture, not latch failed.
- Progress is ignored when the job is not `recording`.
- Zero-segment capture reports no progress (no divide-by-zero).
- Capture is `MaxRetry(0)`. `demo_incompatible:` is not retried against the same CS2 build.
