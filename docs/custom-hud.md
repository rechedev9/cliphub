# Custom broadcast HUDs

The reference is a broadcast layout: team score, round clock, living players,
player cards and observed-player health, armor and weapon information. The
ten original ClipHub designs are Arena, Apex, Prism, Carbon, Pulse, Royale,
Glacier, Ember, Circuit and Mono. Colors, panel shapes and layout differ;
they share one factual state model and one vector scene renderer.

## Rendering decision

Evaluated on 2026-09-09:

| Approach | Evidence | Fit for ClipHub |
| --- | --- | --- |
| HLAE Panorama styling | [Source2:mirv_panorama](https://github.com/advancedfx/advancedfx/wiki/Source2%3Amirv_panorama) exposes panel opacity, with commands reapplied after demo reload. | Useful to hide the native panels that will be replaced. It does not provide a full CS2 theme authoring API. |
| HLAE before-UI / separate HUD stream | [Source2:mirv_streams](https://github.com/advancedfx/advancedfx/wiki/Source2%3Amirv_streams) documents `capture beforeUi` and alpha / white-black HUD separation. | A useful future capture option, but removing the entire UI also removes the native radar and crosshair. |
| Browser overlay and live GSI | [Lexogrine React HUD](https://github.com/lexogrine/cs2-react-hud) and [OpenHUD](https://github.com/JohnTimmermann/OpenHud) use a local web HUD driven by game state. | A good live broadcast architecture and visual reference. A rendered MP4 cannot reproduce the GSI stream; wall-clock GSI is also a poor edit clock for seeking and offline recording. |
| Demo facts and offline vector composition | ClipHub already keeps the demo, tick-to-frame edit plan and captured media. | Selected: parse HUD facts from the source demo, keep source ticks, compose at the approved frame boundaries, and reuse a compatible clean capture when changing theme. |

The scene is drawn by FFmpeg/libass as ASS vector graphics. The HTML picker
uses lossless transparent WebP previews from that same renderer; an SVG export
is also available for diagnostics. This keeps typography, panel geometry
and data mapping in one implementation without requiring a second browser
installation on the capture worker. No third-party HUD source, logos or player
portraits are copied. The catalog thumbnails use clearly identified example
data; the export path requires real demo telemetry.

The broadcast capture keeps native radar, killfeed, scope and crosshair. It
sets `cl_draw_only_deathnotices=1`, `cl_drawhud_force_radar=1` and
`cl_drawhud_force_deathnotices=1` through HLAE's cvar API, verifies their
readback throughout the capture and restores the saved values afterward.
The [CS2 demo config by Purp1e](https://github.com/Purple-CSGO/CS2-Config-Presets/blob/master/demo.cfg)
also uses the native crosshair/deathnotice and radar controls. Existing
clean-spectator Panorama suppression still hides chat, votes and death panels.
A native HUD already burned into a video cannot be removed losslessly. The
first custom HUD therefore needs a compatible capture; subsequent theme
changes are render changes and reuse it. Native Full Demo remains available.

## Integration and verification

The Full POV Chill constructor exposes the ten previews and persists the selected
`overlays.hud_theme` with the approved plan. The planner requires the
`broadcast-clean` capture profile for a custom theme. Theme changes affect the
render hash but leave the capture hash unchanged. Old native plans omit the new
optional field and keep their existing wire format and capture behavior.

The worker verifies the source demo hash, extracts or reuses validated telemetry,
and binds that telemetry to the approved demo, player and tick rate. Source2's
network clock supplies round and bomb deadlines; its origin differs from demo
seek ticks. The renderer uses each edit item's source offset, including trims
and sponsor splits. The HUD is only composed onto round items. The published
`full-demo-hud.json` identifies the renderer, theme and telemetry digest, and is
required for accepting a cached custom export.

Local acceptance on 2026-09-09 used the first full round of the user's Mirage
Donk demo (SHA-256 `eb8853783b98a7899ab772eb75efb14729b9c628bb050aec3a37339f7638b8b1`).
The installed Studio 2.4.64 and its data were preserved. Tests ran the candidate
source on `feat/custom-broadcast-huds` from base `12d1311`, using separately built
binaries and an isolated SQLite/data directory.

- A real capture with HLAE 2.191.1 and CS2 passed the existing capture attestation.
  Radar, killfeed and the observed crosshair remained visible while native player
  panels were absent. Original cvar values were restored and verified.
- All ten designs completed the production editor pipeline at 1920x1080, 60 fps,
  2,440 frames (40.6667 seconds), stereo 48 kHz. Every output passed full decode,
  exact frame count and decoded AAC loudness/true-peak acceptance.
- The real local UI saved and generated Arena with team voices, then saved and
  generated Apex. Both became ready with distinct render revisions and the same
  verified capture revision `78dbe053-4e6e-40e9-a8eb-286b44b7346a` and telemetry
  digest. Playback, seeking and pause were exercised through the UI player.
- Rendered frames were checked for all ten layouts. Against the native recording,
  player health, ammunition, alive counts and the round clock agreed in sampled
  gameplay. Native clock samples at 22.1, 22.5 and 23.1 seconds matched the custom
  clock; a display update can differ at a frame exactly on a second boundary.
- The selector was exercised at 390, 1024 and 1440 px; the browser regression also
  uses a long unbroken player name and checks neighboring controls and saved
  approval/generation requests. All 17 Full Demo browser tests passed.

The recording probe exposed an existing capture mux issue: `-shortest` could
drop the last video frames when audio ended first. During PR integration, the
upstream frame-preservation fix from #170 was retained, including its existing
FFmpeg regression. This change adds no competing mux policy. The single-round
worker path now explicitly requests the compilation required by Full Demo.

The PR candidate is based on `e11f9b0` (Studio 2.4.66), preserving the newer
overlay-image and round-transition controls. HUD composition follows the camera
transition effects, so the scoreboard remains fixed. A real FFmpeg regression
checks that gameplay changes during a transition while HUD pixels remain stable;
the wire regression also combines custom HUD and transition options.

Relevant Go suites, web unit tests, typecheck, lint and the production web build
passed. The real media acceptance above covers one complete round with two
Donk kills; it is not a claim of replaying and exporting the entire match in
every style. Local QA artifacts and logs are under `.local/custom-hud/`.

The requested P0 autoreview was attempted on the complete local candidate with
Codex GPT-6 Astra / high. Its preparation rejected the ten binary WebP previews
before a reviewer started, so no independent review result was produced. This
change does not include a release or an installed-app update.

## PR verification follow-up

The HUD generator is built by both project build entrypoints and exposed as
`zv hud-designs`; the existing command-coverage checks remain strict.

Two Bugbot cases were reproduced and fixed. Freeze snapshots now follow the
CS2 freeze property even when a `RoundStart` event is absent. Disconnected or
missing controllers retain an inactive observed-player identity without taking
a current roster slot or poisoning the alive count. A connected player whose
pawn statistics are unavailable still displays unknown data. Telemetry/cache
version `broadcast-hud-v2` invalidates snapshots from the previous extraction.

The regression can also run against a real local demo with
`FULL_DEMO_HUD_DEMO` and `FULL_DEMO_HUD_TARGET`. On the Donk Mirage fixture it
suppressed every `RoundStart` event and verified the freeze prefix, ASS window
coverage and source round label for all 19 rounds. The separate round-number
finding was a false positive: the test comment's "rounds" meant magazine
bullets. `TotalRoundsPlayed` is bound to CS2's `m_totalRoundsPlayed`; all 19 live
round labels agreed with the source scores. The comment now says "bullets".
