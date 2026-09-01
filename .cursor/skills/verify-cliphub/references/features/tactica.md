# Táctica

Radar / round analysis side lane.

## Sub-features

- `tactica-rail` — Sidebar `04 Táctica` → `/tactical`.
- `tactica-picker` — Parsed demos only; this screen does not start work.
- `tactica-workspace` — `/tactical/{jobId}` rounds, replay, tendencies.
- `tactica-positions` — Positions blob is Range-capable.

## How to get to it (user POV)

- Click **Táctica**.
- Pick a parsed demo, wait for analysis, scrub a round.

## What done looks like

Header **ANÁLISIS TÁCTICO**. Workspace shows rounds and a usable radar. Tactical state is its own lifecycle; it does not rewrite production job status.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live analysis: parsed demo + orchestrator.

- **Cheap contract.** `./bin/zv verify prove --feature tactica --format json`.
- **Live API.** When Studio is up, prove GETs `/api/demos/jobs` on Studio `web_url` (picker lists parsed demos). Empty list is success. `user_path` becomes inspected, never pass.
- **Open picker.** Click **Táctica**. Index lists parsed matches and does not enqueue analysis by merely opening.
- **Workspace.** Open `/tactical/{jobId}`. Rounds are scrubbable.
- **Offline.** Same-origin `/api/demos/{jobId}/tactical/*` returns service-unavailable, not a parser crash.

## Gotchas

- `tacticalplan` must not import a parser.
- Anticheat/tactical must not write production job status.
- Same-origin only: `/api/demos/{jobId}/tactical/*`.
