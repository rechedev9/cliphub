# Render quality and performance

ClipHub renders SDR H.264/AAC delivery files through three FFmpeg paths:

| Product path | Package | Default encode |
| --- | --- | --- |
| Demo shorts and long-form POV | `internal/editor` | x264 slow, CRF 16, 60 fps |
| Stream/VOD clips | `internal/streamclips` | x264 slow, CRF 18, 60 fps |
| Multitrack timeline | `internal/timelinerender` | x264 slow, CRF 18, canvas fps |

## Scaling policy

Every primary source-video resize uses FFmpeg swscale with
`lanczos+accurate_rnd+full_chroma_int`. Lanczos preserves fine HUD and text
edges; accurate rounding and full chroma interpolation reduce rounding and
chroma-placement artifacts during crop/scale operations.

The delivery format remains `yuv420p` for playback and upload compatibility.
Temporal smoothing remains opt-in because frame blending can create ghosting
on fast camera movement, crosshairs, and killfeed text.

## Measurements from real jobs

Finished result artifacts carry content-free local measurements:

- `render_ms`: wall-clock encode time;
- `output_bytes`: encoded file size;
- `media_duration_seconds`: output timeline duration;
- `render_seconds_per_media_second`: encode time divided by media duration;
- post-render timings such as probe, cover, cover-sheet, and quality-check time
  where that path performs those stages.

A value below `1.0` for `render_seconds_per_media_second` means faster than
realtime. Reused short artifacts set `reused: true` and do not claim encode
work that did not run.

Measurements live in `shorts-result.json`, stream render results, and timeline
render results. They contain no machine identifier, source URL, or media title.
Compare the same source, plan, FFmpeg build, encoder, CRF, preset, and thread
count when evaluating a quality or performance change.

## Quality decisions

Do not lower CRF globally based only on file size or a synthetic test pattern.
Capture and final encode form a two-generation pipeline, so a quality study
must compare representative CS2 movement, HUD edges, particles, smoke, and dark
areas. Preserve a candidate's command and evaluate decoded frames or objective
metrics against the same high-quality reference before changing defaults.

Hardware encoders are suitable when capture throughput is the constraint. The
upload master continues to use software x264 by default because it provides a
stable quality-per-bit baseline across supported machines.
