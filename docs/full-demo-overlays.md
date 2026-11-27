# Full Demo overlays

A Full Demo opens with a player introduction (who plays with whom) and closes
with the final scoreboard. Both are generated from the demo. The **Diseño**
gallery under **Presentación y marcador** in the producer picks their look:

- **Neón**, the default: the violet neon panels.
- **A juego con el HUD**: the same pages in the colours of the plan's HUD design
  or own theme (CT, T, background and corner radius). Over the original CS2 HUD
  it uses a fixed neutral palette. Text that an own background would make
  unreadable switches to a neutral light or dark pair.

A design changes the look, never the content: both designs share one page body
(`intro-body.html`, `scoreboard-body.html`), so they show two panels of five
cards with the selected POV first and a scoreboard with the same columns and the
POV row highlighted. The demo origin (**Origen de la demo**) decides the data:
FACEIT demos show FACEIT profile statistics, the others the statistics of that
match.

The plan stores the design in `overlays.theme` (`neon-violet` or `hud`). It
is render only and never enters the capture hash, so a finished video can change
design without recording again. Saved plans keep `neon-violet` and their hash.
The legacy orange theme and the uploaded screenshots (`overlays.mode`
`screenshots`) are retired: new plans cannot choose them, and documents
approved with them stay readable.

Each card of the gallery shows `GET /api/full-demo/overlay-designs/preview`:
the example match drawn with the renderer's own page, wrapped as an SVG image
with the fallback font. Assistants choose the design with `plan_full_demo`
`intro_design`.

## Halftime card and subscribe button

Two optional switches under **Presentación y marcador** add overlays over the
rounds. Both are off by default, render only, and follow the chosen design
(`overlays.theme`): neon, or the HUD palette from `demooverlay.DesignPalette`.
The plan stores them as `overlays.halftime` and `overlays.subscribe`, omitted
when off, so saved plans keep their hash and neither enters the capture hash.
Assistants set them with `plan_full_demo` `halftime_scoreboard` and
`subscribe_button`.

- **Marcador del descanso**: a compact card in the upper third, under the HUD
  strip, with the word DESCANSO, both team names (the POV's team first) and the
  score at the side switch. It shows for `demooverlay.HalftimeSeconds` at the
  start of the first round after the regulation side switch and slides and
  fades in and out. The render worker derives it from the job's kill plan round
  results (`demooverlay.BuildHalftime`): the first round where the POV's side
  changes is the switch, and it counts only when the POV played every earlier
  round on the other side, so overtime switches never count and the round is
  not assumed to be 13. A POV who joins late gets no card. Names come from `overlays.team_names`, then the demo's clan
  names, then the side each team plays in the second half. The card is left
  out, without failing the render, when the kill plan has no round results (a
  job parsed before they were kept), a round before the switch has no winner or
  no number, the POV's side is unknown on the round before the switch, the
  switch round is not in the video (a round selection or a short match), or the
  round leaves less than two seconds clear of the intro roster and the final
  scoreboard. It moves under the sponsor banner when both show at once.
- **Botón de suscripción**: a SUBSCRIBE pill with a bell drawn as inline SVG
  in the bottom right corner, for `demooverlay.SubscribeSeconds`, placed near
  25 % and 75 % of the gameplay span between the bumpers. Each window lies
  inside one round item, at least a second away from the intro roster, the
  halftime card and the final scoreboard, and the two stay 20 s apart; a video
  too short for that shows it once or not at all. With the brand watermark in
  the bottom right corner the button sits above it.
  When the brand kit has a channel logo or channel name, turning the switch on
  copies them into `overlays.brand.subscribe` and the button becomes a card
  with the round logo, the name and a red SUBSCRIBE button
  (`demooverlay.SubscribeCard`); the logo is an overlay image the plan must
  verify like the watermark. Shorts use the same card through
  `subscribe_banner`, once a Short (`internal/editor/short_subscribe.go`).
  The still is a sprite: one cell at rest and `demooverlay.SubscribeSprite`
  cells of the bell swinging, which a time-based `crop` picks at
  `demooverlay.SubscribeBellFPS` from 0.5 s and 2.6 s into each window
  (`editor.ImageSprite`). The overlay renderer draws only a 1920x1080 or
  1080x1920 page, so `RenderSubscribeSprite` renders the cells on several such
  pages and stacks them into one PNG. The first swing adds a soft generated marimba note (`aevalsrc`, C5, no
  sound file) to the game audio of a Short and to the item audio of a Full
  Demo, except in a plan that mutes every audio source.

`internal/editor/full_demo_story.go` renders both stills beside the intro and
outro PNGs (`full-demo-halftime.png`, `full-demo-subscribe.png`) and adds them
as image effects that the per-item composition supports
(`supportedFullDemoOverlaySources`), so they do not bring back the
whole-program pass.

## Opening hook

**Gancho inicial**, under **Entre rondas**, opens the video with a teaser of
the player's best play and then plays the match from round 1, where the play is
seen again in its round. It is off by default and render only: the plan stores
`options.hook` (`{enabled: true}`, omitted when off, so saved plans keep their
hash), it never enters the capture hash and it reuses the footage of its round.
Assistants set it with `plan_full_demo` `hook`.

- **Selection.** When the plan is made, `recapplan` groups each round's POV
  kills into plays (kills at most `HookPlayGapSeconds` apart, the kill play
  window of the Short flow) and scores them with `moments.Rank`, the score of
  `list_moments` and Auto-Shorts. The best play wins and a tie keeps the
  earliest. Only rounds in the video count (a round selection narrows it), only
  kills up to the round's `live_end_tick`, which every certified capture
  covers, and only rounds whose certified part lasts `HookMinSeconds`. The
  document carries the choice in `hook` (round and first and last kill tick),
  so the approved hash binds it and admission plans it again from the facts.
- **Window.** From `HookLeadSeconds` (3 s) before the first kill to
  `HookTailSeconds` (1.5 s) after the last, inside the round's requested start
  and effective end. Under `HookMinSeconds` (8 s) it grows backwards, then
  forwards at the round start; over `HookMaxSeconds` (20 s) it keeps its end
  and drops its start. `RebuildTimeline` derives it from the round each time,
  so a certified tail trim shortens the tail after the last kill and the
  window grows backwards again: the hook never reads past the effective end.
- **Timeline.** One more `round` item with reason `hook`, after the intro clip
  and before the first round, with `source_ref` of its round and
  `source_offset_frames` of the teaser. It is not a round: the sponsor still
  follows the second round, captions and titles keep their round count, and
  its kills are not counted again in the kill count, the cover frame or the
  publish metadata. A video with one round, or without a POV kill, has no hook
  and carries the `hook_skipped` warning; it never blocks.
- **Picture and sound.** The item goes through the same per-item render as its
  round: custom HUD, radar, subtitles of that stretch, game audio and team
  voices at the same offsets, and the round transition into the first round
  (also when the hook comes from round 1). Its voices do not run on over the
  first round. A `MEJOR JUGADA` label (`demooverlay.RenderHookPNG`,
  `full-demo-hook.png`) slides into the top left corner, right of the radar,
  for the whole hook, in the chosen design.
- **Other overlays.** The player introduction opens the first real round
  (`fullDemoGameplaySpanSeconds` skips the hook like the intro clip), and the
  halftime card and the subscribe button never land on the hook. The brand
  watermark shows on it. The sponsor banner keeps its window on the video
  clock, as with an intro clip: a banner whose window starts in the first
  seconds also shows over the hook, and nothing is shifted.

## Second camera

**Segunda cámara**, a switch in the producer (off by default), draws a small
window over the gameplay with the view of an enemy who was holding an angle
just before the player killed them. The plan stores `options.second_camera`
(`{enabled, max_windows, removed}`, omitted when off, so saved plans keep their
hash). The windows are not part of the capture hash: they are recorded as extra
capture segments of their round. Assistants set it with `plan_full_demo`
`second_camera`, `second_camera_max` and `second_camera_remove`.

- **Selection.** A kill qualifies when the POV killed a real player (never a
  bot, never the POV, never a teammate), the kill is at least
  `SecondCameraLeadSeconds` (5 s) after the live start of its round, and the
  victim moved at most `SecondCameraStillUnits` from the kill position during
  those 5 s, sampled `HoldSampleHz` times a second by the parser (`facts.holds`,
  left out of the facts hash). The threshold is provisional and is calibrated
  from `move_units` on real demos. A match parsed before this data existed
  gets it the first time a plan asks for the second camera: the plan handler
  reads the demo once (`ensureKillHolds`) and keeps the result with the facts.
  A demo that cannot be read carries the `second_camera_unavailable` warning
  and gets no window. Each qualifying kill takes the score of its play (as `list_moments`
  scores, the same plays as the hook), the best one of each round is kept, and
  the best `max_windows` (default 5, at most 10) are the windows. Ids in
  `removed` are dropped after the cap, so a removed moment never brings in
  another. A kill is left out when its window ends less than a second before
  the end of the match's last round, or less than `SecondCameraRewindSeconds`
  before the end of the demo: recording it would show the match end screen
  before the rounds are recorded. No candidate gives the `second_camera_skipped` warning. The choice is
  deterministic and the document's `second_camera.windows` carries it (id,
  round, enemy, kill tick), bound by the plan hash. The id has the form
  `round-NNN-camera-<tick>`.
- **Capture.** Each window is a capture of the enemy's view from 5 s before the
  kill to `SecondCameraTailSeconds` (1 s) after, recorded in the same CS2
  session before the rounds: the windows first, then a rewind, then the rounds.
  CS2 stays on the match end screen once playback reaches it, and a rewind does
  not clear it, so nothing that needs a rewind may come after the last round.
  The window is
  recorded without CS2's radar and kill feed, which are unreadable at that
  size: the HLAE runtime switches the three HUD cvars when it prepares the
  first window (`fullDemoWindowHUD`), under the same readback contract as the
  rest of the capture, and switches them back when it prepares the first
  round. A video that is
  already recorded records only the windows, and the rounds are reused. The
  capture check reports them as `missing_cameras`. A window that CS2 cannot
  show, or whose enemy it loses, never fails the capture: it certifies a shorter
  clip. A window with a stored clip counts as attempted and is not recorded
  again by itself. The cost is about 11 s of capture per window and about 40
  Mb/s of disk, freed with the rest of the capture.
- **Picture.** The window sits on the left (`demooverlay.SecondCameraX`,
  `SecondCameraY`), 504 by 284 with rounded top corners, and the enemy's name in
  a bar under it in the chosen design. With the ClipHub radar it moves below
  the radar. It fades in and out and plays in sync with the POV. It has no
  audio: the camera clip's sound is never mapped, so the mix is unchanged.
- **Where it never goes.** Not on the hook, the bumpers, the player
  introduction, the halftime card or the final scoreboard. The window keeps the
  longest stretch of its time that none of them covers, and a window with less
  than 3 s left is dropped.
- **When a window cannot be used.** If its clip is missing, its certified end
  falls short of the window or its frames fail the check, the render leaves it
  out, adds a `second_camera_omitted` warning naming the round and succeeds. The
  render evidence lists each window with `drawn` and a reason
  (`capture_omitted` or `covered_by_overlay`), and `plan_full_demo` reports
  `omitted_second_camera`.
- **Metadata.** A window adds no kills and no rounds to the timeline, the
  cover frame or the publish metadata. Studio only: ClipMind never shows it.

## Rendering

The neon introduction uses HTML, CSS perspective, glow, embedded Roboto fonts
and SVG decorations. It has two panels with five player cards each and places
the selected POV player first within their team. The final scoreboard uses the
same visual theme and highlights the POV row. The HUD design draws the same
pages with flat panels in the HUD colours.

Studio renders both designs, and the screenshot template of old plans, with its
bundled Chromium runtime. The `--cliphub-render-overlay` entry point creates a separate hidden
process without starting the normal Studio services or acquiring the main
application's single-instance lock. The page is sandboxed, uses embedded local
resources, and has no network access or Canvas elements. Rendering waits for
fonts, image decoding and a completed compositor paint before exporting a
transparent 1920x1080 PNG; the player cards of a 9:16 Short (`internal/playercard`) use
the same entry point at 1080x1920, the only other size it accepts. A temporary DOM marker synchronizes the compositor;
the exported frame contains no marker. FFmpeg composites those images during
the intro and outro windows. In every design, the introduction occupies only
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
using an HTML design or screenshots must supply the equivalent renderer
configuration.
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

Plans approved with the retired screenshot mode keep their assets in the
dedicated immutable `overlay-images/<uuid>` storage namespace. The API returns an ID and SHA-256; the plan includes those references
and verified image evidence. Planning, admission and rendering recheck the
approved bytes. Missing, changed or invalid images block generation. Existing
music and sponsor media keep their provenance checks. New optional plan fields
use `omitempty`, preserving canonical hashes for previously saved plans.

Font and flag provenance and licenses are in
[`internal/demooverlay/assets/README.md`](../internal/demooverlay/assets/README.md).

## Validation

The checks below date from the screenshot mode and the first neon render.

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
