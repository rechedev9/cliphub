# CheaterDetect

Side-lane anomaly screen. Never a guilt verdict.

## Sub-features

- Sidebar `05 CheaterDetect` → `/cheaters`
- Pick a local demo, run `analyze:anticheat`
- Per-player dossier render
- Official-channel links only

## How to get to it (user POV)

Click **CheaterDetect**. Select a parsed demo. Start screening. Open one player's dossier.

## What done looks like

`jobs/<id>/anticheat.json` exists. The job's production status is unchanged. The UI still says this is an anomaly report, not guilt. ClipHub does not submit a report.

## Driving it with zv verify

```text
./bin/zv verify prove --feature cheaterdetect --format json
```

Cheap proof: map + `/cheaters` + anticheat unit tests. A real demo screen needs a `.dem`.

## Gotchas

- Screening never writes `job.Status`. A failed screen must not make a healthy job look broken.
- Do not file reports on the user's behalf. Do not weaken `insufficient_data`.
- Baseline sample counts are measurements. Do not zero them to reconcile prose.
