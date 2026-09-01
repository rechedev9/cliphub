# Jugadores

FACEIT-indexed player list for triage. Stats are not the clip source.

## Sub-features

- Sidebar `06 Jugadores` → `/players`
- Followed players and match rooms
- Profile links out; download stays user-authorized

## How to get to it (user POV)

Click **Jugadores**. Browse indexed players. Open a room URL if present.

## What done looks like

Players and matches render from the local FACEIT index. No Download API. No key in the page or logs.

## Driving it with zv verify

```text
./bin/zv verify prove --feature jugadores --format json
```

Cheap proof: map + players page. Live FACEIT needs `FACEIT_API_KEY` in the environment, never in git.

## Gotchas

- FACEIT Data API is approved. FACEIT Download API is not.
- Rate stats must be normalized per round when match lengths differ.
- Use external stats to shortlist demos; use parsed demo evidence to select moments.
