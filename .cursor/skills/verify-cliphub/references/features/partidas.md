# Partidas

Match list for parsed and scanned demos, including series grouping.

## Sub-features

- Sidebar `01 Partidas` → `/matches`
- Series cards → `/series/{seriesId}`
- Match detail → `/matches/{jobId}`
- Empty state can send a first-run user to onboarding

## How to get to it (user POV)

Click **Partidas** in the rail, or finish an upload and return to the match list.

## What done looks like

The list shows local jobs. A series shares a client-minted `series_id`. Opening a match shows roster and next actions (Shorts, Demo completa) without inventing remote match data.

## Driving it with zv verify

```text
./bin/zv verify prove --feature partidas --format json
./bin/zv verify http --format json
```

Cheap proof: map + nav + matches page. Live job list needs a running orchestrator.

## Gotchas

- `scanned` in a series is settled, not progress.
- Orchestrator 404 after restart is unrecoverable for that job id; Retry cannot re-drive a gone job.
- Service offline is `503 {code:"service_unavailable"}`, not a bad demo.
