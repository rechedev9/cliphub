# Clips de stream

VOD → persisted edit plan → render. The plan is canonical.

## Sub-features

- Sidebar `07 Clips de stream` → `/streams`
- Variant pick, edit plan, render
- Crop, audio, fades, text, `music.volume` live on the plan

## How to get to it (user POV)

Click **Clips de stream**. Open a local MP4 or a fetched VOD. Plan the edit, then render after the brief.

## What done looks like

A persisted edit plan exists. Render output matches that plan. No ad hoc FFmpeg flags bolted around it.

## Driving it with zv verify

```text
./bin/zv verify prove --feature clips-de-stream --format json
```

Cheap proof: map + stream unit tests. A stream dry-run does not create its `--out` artifact.

## Gotchas

- Changing a stream plan invalidates its creative brief.
- Persist each approved plan before a dependent stage.
- Skip fetch when the source is already a local MP4.
