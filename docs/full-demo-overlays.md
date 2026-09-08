# Full Demo overlays

The Full Demo producer offers two modes under **Cómo crear los overlays**:

- **Generar automáticamente** builds the enabled roster and scoreboard from the
  current demo, optionally enriched with FACEIT profiles. It offers the existing
  orange theme and a rebuilt violet neon theme.
- **Subir capturas** accepts two images for the introduction, one per team, and
  one image for the final scoreboard. Only images needed by enabled overlays
  are required. Switching modes preserves the uploaded selections.

The upload controls accept PNG and JPEG, up to 10 MiB each, 8192 pixels per side
and 32 megapixels. Studio decodes the entire image before accepting it, keeps
the original bytes, and shows a preview in the same proportions used by the
renderer. Team images occupy opposite sides of the introduction; the final
scoreboard is centered. Both use `object-fit: contain` so names and statistics
are not cropped or stretched. The screenshot mode does not require FACEIT.

## Rendering

The violet neon introduction uses HTML, CSS perspective, glow, embedded Roboto
fonts and SVG decorations. It has two panels with five player cards each and
places the selected POV player first within their team. The final scoreboard
uses the same visual theme and highlights the POV row. The legacy orange
renderer remains available.

Studio renders the neon and screenshot templates with its bundled Chromium
runtime. The `--cliphub-render-overlay` entry point creates a separate hidden
process without starting the normal Studio services or acquiring the main
application's single-instance lock. The page is sandboxed, uses embedded local
resources, and has no network access or Canvas elements. Rendering waits for
fonts, image decoding and a completed compositor paint before exporting a
transparent 1920x1080 PNG. A temporary DOM marker synchronizes the compositor;
the exported frame contains no marker. FFmpeg composites those images during
the intro and outro windows. In both modes, the introduction occupies only
0:00-0:05, including its slide animations; the scoreboard uses the final eight
seconds. Short videos clamp these windows without overlap. Overlay timing is
included in render reuse identity, so an earlier introduction is regenerated
without recapturing rounds or changing the approved gameplay timeline.

The five-second adjustment was checked separately on the recorded first round:
the roster is visible at 0.5 and 4.5 seconds and fully absent at 5.0 seconds.
The complete match export documented below predates this timing adjustment.

The Go worker invokes the renderer through `ZV_OVERLAY_RENDERER_PATH`. Studio
sets it to its own executable. Development also supplies
`ZV_OVERLAY_RENDERER_APP` with the desktop application path. Standalone workers
using neon or screenshots must supply the equivalent renderer configuration.
The implementation reuses Studio's Chromium; it does not add a Python runtime
or change the validated Go/FFmpeg video and audio pipeline.

## Data and immutable plans

FACEIT introduction cards distinguish lifetime `Matches` from the last 20
matches. Recent headshot rate comes from those recent FACEIT statistics rather
than from the current demo. Missing values remain absent or display an em dash.
Verification, premium membership, rank, country and avatar use available
profile evidence. The scoreboard uses demo match statistics and labels FACEIT
ELO as current profile ELO. It does not invent historical ELO changes or metrics
that the available data does not provide. Consequently, automatic values can
differ from a previously taken FACEIT screenshot.

Screenshot assets use the dedicated immutable `overlay-images/<uuid>` storage
namespace. The API returns an ID and SHA-256; the plan includes those references
and verified image evidence. Planning, admission and rendering recheck the
approved bytes. Missing, changed or invalid images block generation. Existing
music and sponsor media keep their provenance checks. New optional plan fields
use `omitempty`, preserving canonical hashes for previously saved plans.

Font and flag provenance and licenses are in
[`internal/demooverlay/assets/README.md`](../internal/demooverlay/assets/README.md).

## Validation

Regression coverage includes original upload bytes and hashes, invalid and
oversized images, enabled image requirements, tampering after approval, legacy
plan serialization, truthful FACEIT statistic scopes, and rendering without a
FACEIT client in screenshot mode. Chromium integration verifies exact output
dimensions, transparent pixels and the aspect ratios of contrasting landscape
and portrait images. Repeated runs and the packaged Studio runtime verify that
the renderer does not capture an earlier blank paint.

The Go overlay, asset, FACEIT, planner, HTTP, worker and recording packages pass.
Desktop TypeScript checking and its 308 tests pass. The affected frontend tests,
TypeScript checks, lint checks and production build pass.

Live Windows verification on 2026-09-08 uploaded all three supplied images through
the installed Studio UI, preserved them across application restarts, and produced
a 57-second test video using the 19 recorded Mirage rounds. The MP4 contains
exactly 3420 frames, passes complete decoding and the production AAC delivery
checks, and has no delivery warnings. Frames extracted from the delivered intro
and outro confirm that the screenshots remain complete and proportionate.

The full neon render reused the same 19 recordings and the original approved
round, capture and audio settings. Its delivered MP4 contains exactly 56,424
frames at 1920x1080/60 fps and lasts 940.4 seconds. Complete decoding passes;
stereo 48 kHz AAC measures -14.24 LUFS and -3.25 dBTP. Studio playback reached
the last frame after seeking to the intro, middle and outro, with no media
errors and zero dropped playback frames.

The generic still-image detector reported 936.300–937.467 and
938.650–940.367 seconds, both inside the approved final scoreboard window
(932.400–940.400). A repeat of the complete QA command found no other freeze
intervals or black-frame warnings. All 480 decoded outro frames have distinct
pixel hashes. Codex inspected the actual delivered scoreboard and recorded the
intentional still-overlay intervals through Studio's existing QA review dialog.
The warning and review note remain bound to that render revision; the detector
and delivery acceptance rules were not changed.

The native Studio download completed with 4,495,573,222 bytes. Its SHA-256 equals
the validated delivery:
`bca11ce71f60652eb8c945b53938f68abad26d68b18258a6b8ab59cc7cf30993`.
The full video is `Mirage-donk666-Full-Demo-Neon-20260908.mp4`; the screenshot
sample is `Mirage-Overlay-Capturas-Prueba-20260908.mp4`, both in `Videos\ClipHub`.

The final installed UI was checked at 390, 1024 and 1440 pixels: all three
previews load with their aspect ratios preserved, controls remain inside the
viewport, and there is no horizontal overflow. The three custom upload buttons
open their respective file selectors. These checks use a native hidden file
input: combining the shared styled input with `sr-only` had retained its
full-width sizing and caused overflow at medium widths.

The local installation is based on release 2.4.64 plus the frame-preservation
and overlay changes. It runs from `%LOCALAPPDATA%\Programs\ClipHub Studio Local`
through the **ClipHub Studio Local** desktop shortcut. Private installation and
verification receipts are in `.local/full-demo-overlay-reference-20260908/`.
