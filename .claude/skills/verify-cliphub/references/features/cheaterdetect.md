# CheaterDetect

Side-lane anomaly screen. Never a guilt verdict.

## Sub-features

- `cheaters-rail` — Sidebar `05 CheaterDetect` → `/cheaters`.
- `cheaters-pick` — Pick a local parsed demo (`aria-label="Demos analizables"`).
- `cheaters-run` — **ANALIZAR DEMO** starts `analyze:anticheat`.
- `cheaters-dossier` — Per-player dossier render.
- `cheaters-limits` — Section **Qué es y qué no es esto** stays on the report.

## How to get to it (user POV)

- Click **CheaterDetect**.
- Select a parsed demo. Start screening. Open one player's dossier.

## What done looks like

`jobs/<id>/anticheat.json` exists. The job's production status is unchanged. The UI still says this is an anomaly report, not guilt. Unanalyzed copy: **Esta demo aún no se ha analizado**. ClipHub does not submit a report.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live screen: a parsed `.dem` and orchestrator. Screening does not need HLAE/CS2.

- **Cheap contract.** `./bin/zv verify prove --feature cheaterdetect --format json`.
- **Live API.** When Studio is up, prove GETs `/api/demos/jobs` on Studio `web_url`. Screening is a later POST. `user_path` becomes inspected, never pass.
- **Open.** Click **CheaterDetect**. Pick a demo in Demos analizables.
- **Start.** Click **ANALIZAR DEMO**. Running copy: Analizando la demo. You can leave the page; analysis continues.
- **Limits.** After a report, **Qué es y qué no es esto** lists `limitations` from the document.
- **Side lane.** Job production `status` is unchanged in `GET /api/jobs/{id}?view=status`.

## Gotchas

- Screening never writes `job.Status`. A failed screen must not make a healthy job look broken.
- Do not file reports on the user's behalf. Do not weaken `insufficient_data`.
- Baseline sample counts are measurements. Do not zero them to reconcile prose.
