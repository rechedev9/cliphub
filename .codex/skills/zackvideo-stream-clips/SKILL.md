---
name: zackvideo-stream-clips
description: "Create upload-ready ClipHub stream clips from recorded stream VODs: pick a layout variant, plan the edit, and render vertical or landscape packs after the user approves the creative brief."
---

# ClipHub Stream Clips

Use this skill when the user wants clips from a recorded stream VOD (Twitch/YouTube/local MP4), especially vertical facecam-over-gameplay Shorts or landscape long-form cuts.

The journey is `stream variants -> stream plan -> stream render`.

## Creative Brief Gate

Before any non-dry-run render, ask the user only for the creative choices they have not already supplied, grouped into one concise message, and wait for explicit approval. Do not treat ambiguous execution words like "go", "hazlo", "dale", "ok", or "ya deberia estar ok" as approval unless they answer a previously shown brief:

- delivery/layout variant: discover the supported list first and offer the real names (`streamer-vertical-stack-40-60` facecam stack, `streamer-fullframe-nocam`, `streamer-landscape-16x9`, plus any newly listed variant);
- clip boundaries: which stream moments to cut (start/end timestamps or clip IDs) and one title per clip;
- framing style: clean crop preference, whether facecam should be prominent, and whether gameplay should preserve HUD/killfeed even if that means less zoom;
- music: none, or a track directory the user provides;
- delivery shape: one clip per moment or one longer compilation;
- thumbnail/cover: generated frame cover or no cover.

If the user delegates creative control before a brief exists, state the resolved defaults as a concrete brief and ask for approval; only a follow-up confirmation approves the run.
Preserve every approved answer in the exact render argv; do not silently replace answers with preset defaults later.

Use this question shape for a fresh stream URL:

```text
Antes de renderizar dime/confirmame:
1. Layout: stack vertical facecam+gameplay, full gameplay sin cam, o landscape 16:9.
2. Recorte: que parte del frame quieres en cada banda.
3. Musica: ninguna o carpeta/track.
4. Salida: un clip unico o compilacion.
5. Titulo/cover: titulo final y cover generado o sin cover.
```

## Workflow

1. Discover layout variants and default geometry:

```powershell
.\bin\zv.exe workflows run stream-variants -- --format json
```

2. Plan the edit for the chosen variant and clip boundaries:

```powershell
.\bin\zv.exe workflows run stream-plan -- `
  --input <stream.mp4> `
  --variant streamer-vertical-stack-40-60 `
  --clip-id <clip-01> `
  --clip-start <hh:mm:ss> `
  --clip-end <hh:mm:ss> `
  --title "<clip title>" `
  --out <run>\edit-plan.json
```

Use `--dry-run` first when iterating on crops or clip boundaries.

3. Render the approved plan:

```powershell
.\bin\zv.exe workflows run stream-render -- `
  --input <stream.mp4> `
  --plan <run>\edit-plan.json `
  --out <run>
```

Run with `--dry-run` to show the resolved plan before the real render, and only remove it after the Creative Brief Gate is approved.

## QA

- Verify each output with `ffprobe`: H.264 video, AAC audio, `1080x1920` for vertical variants or `1920x1080` for `streamer-landscape-16x9`, and a nonzero duration.
- Confirm the facecam and gameplay crops match the plan and nothing important (killfeed, HUD, facecam) is cut off.
- Put upload-ready MP4s, manifests, and review assets under `<run>\shortslistosparasubir`.
- Point the user to the `shortslistosparasubir` folder when delivering finished media.
