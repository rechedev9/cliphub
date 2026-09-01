# Táctica

Radar / round analysis side lane.

## Sub-features

- Sidebar `04 Táctica` → `/tactical`
- Workspace → `/tactical/{jobId}`
- Round list, replay, tendencies
- Positions blob is Range-capable

## How to get to it (user POV)

Click **Táctica**, pick a parsed demo, wait for analysis, scrub a round.

## What done looks like

The workspace shows rounds and a usable radar. Tactical state is its own lifecycle; it does not rewrite production job status.

## Driving it with zv verify

```text
./bin/zv verify prove --feature tactica --format json
```

Cheap proof: map + tactical unit tests. Live analysis needs a demo and the orchestrator.

## Gotchas

- `tacticalplan` must not import a parser.
- Anticheat/tactical must not write production job status.
- Same-origin only: `/api/demos/{jobId}/tactical/*`.
