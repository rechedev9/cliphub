# Clips de stream

VOD → persisted edit plan → render. The plan is canonical.

## Sub-features

- `streams-rail` — Sidebar `07 Clips de stream` → `/streams`.
- `streams-door` — Inicio plate **Corta un stream**.
- `streams-source` — Twitch/Kick URL or local MP4.
- `streams-plan` — Crop, audio, fades, text, `music.volume` live on the plan.
- `streams-render` — Render after the brief; output matches the persisted plan.

## How to get to it (user POV)

- Click **Clips de stream**.
- From Inicio, choose **Corta un stream**.
- Open a local MP4 or a fetched VOD. Plan the edit, then render after the brief.

## What done looks like

A persisted edit plan exists. Render output matches that plan. No ad hoc FFmpeg flags bolted around it.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Live render: local MP4 or fetched VOD, orchestrator up, brief settled.

- **Cheap contract.** `./bin/zv verify prove --feature clips-de-stream --format json`.
- **Live API.** When Studio is up, prove GETs `/api/streams` on Studio `web_url`. Empty list is success. `user_path` becomes inspected, never pass.
- **Open.** Click **Clips de stream**. Source card accepts URL or file.
- **Skip fetch.** When the source is already a local MP4, do not hit the network fetch path.
- **Persist then render.** Save the plan before a dependent stage. Changing the plan invalidates the creative brief.
- **Dry-run.** A stream dry-run does not create its `--out` artifact.

## Gotchas

- Changing a stream plan invalidates its creative brief.
- Persist each approved plan before a dependent stage.
- Skip fetch when the source is already a local MP4.
