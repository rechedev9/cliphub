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
installation on the capture worker. The layout and renderer are original;
weapon and status silhouettes are adapted from Lexogrine's MIT-licensed assets,
with source revision and license under `internal/customhud/assets/`. The
unmodified Barlow Semi Condensed faces are bundled under the SIL Open Font
License, with Montserrat as the Cyrillic fallback. No team logos or player
portraits are invented. The catalog thumbnails use clearly identified example
data; the export path requires real demo telemetry.

The broadcast capture keeps native radar, killfeed, scope and crosshair. It
sets `cl_draw_only_deathnotices=1`, `cl_drawhud_force_radar=1` and
`cl_drawhud_force_deathnotices=1` through HLAE's cvar API, verifies their
readback throughout the capture and restores the saved values afterward.
The current `broadcast-clean-v2` profile also sets radar background alpha to
0.35, disables additive map blending, uses radar scale 0.85 and default HUD
color, and sets horizontal/vertical safe zones to 0.97/0.95. These six settings
participate in the same snapshot, readback, periodic verification and restoration
contract. A previous `broadcast-clean` capture cannot satisfy this profile.
The [CS2 demo config by Purp1e](https://github.com/Purple-CSGO/CS2-Config-Presets/blob/master/demo.cfg)
also uses the native crosshair/deathnotice and radar controls. Existing
clean-spectator Panorama suppression still hides chat, votes and death panels.
A native HUD already burned into a video cannot be removed losslessly. The
first custom HUD therefore needs a compatible capture; subsequent theme
changes are render changes and reuse it. Native Full Demo remains available.

## Integration and verification

The Full POV Chill constructor exposes the ten previews and persists the selected
`overlays.hud_theme` with the approved plan. The planner requires the
`broadcast-clean-v2` capture profile for a new custom theme. Legacy approved
documents remain readable; reopening their draft upgrades the profile and
requires a fresh saved plan and approval. Theme changes affect the
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
The installed Studio app and its data were preserved. Tests ran the candidate
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
before a reviewer started, so no independent review result was produced at this
initial acceptance stage. Release and installed-app verification are recorded below.

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

## Studio 2.4.67 production acceptance

PR #172 was merged after backend, frontend, infrastructure and Bugbot checks
passed. The final follow-up diff also passed the requested Codex GPT-6 Astra /
high review filtered to P0. The complete PR autoreview remained unavailable
because its preparation rejected the binary WebP previews; the follow-up review
does not represent a review of that entire original candidate.

The published `v2.4.67` Windows installer was built from
`797d979051660b985c3e0a25cb31e932866ac40e`. Its SHA-256 is
`105ed772658a91b93744d8a593f1f0aeb45dc5aaf8ef960d356dd98d0d46c710`.
The downloaded installer and blockmap matched both GitHub's asset digests and
`SHA256SUMS.txt`; the public download returned HTTP 200. The installer upgraded
the existing Studio 2.4.66 installation after installation and SQLite backups.
The launched packaged app and Settings both reported 2.4.67, build production.

Acceptance used the installed executable and the existing user profile. All ten
HUD previews loaded at their native 1920 px width, their selection worked, and
the constructor had no horizontal overflow at 390, 1024 and 1440 px. The Donk
plan was restored to automatic intervals for all 19 rounds with team voices.

HLAE 2.192.1 captured all 19 rounds under `broadcast-clean`. The capture passed
attestation and restored both cvars and configuration files. Its immutable
revision is `e247b09b-a884-44ce-affd-68310e8beb7e`, with input fingerprint
`0c15ff86434a99ec534c490494962d5f0c301eedd664d28fb5e97f0a0a0c423a`.
Sampled first-round and second-half footage retained native radar, killfeed and
crosshair while the native player panels were absent.

Arena completed through the installed UI, including playback, seek and pause.
The delivered video passed full decode, exact frame count and audio timing checks:
56,424 frames at 1920x1080, 60 fps, 940.4 seconds, stereo AAC at 48 kHz. Its
final decoded audio measured -14.24 LUFS and -3.37 dBTP. The effective duration
uses the existing certified POV-tail trimming rules. Final samples from rounds
1, 7, 13 and 19 matched the source telemetry, including the side change and a
zero-armor state. The render recorded `broadcast-hud-v2` and 9,041 snapshots.

Changing to Apex through the same UI produced a separate ready render with the
same frame count, duration and accepted audio measurements. Playback, seek and
pause passed again. Both exports used the same capture revision and telemetry
digest; the stored capture document was unchanged. Published HUD, delivery,
loudness and approval documents matched the immutable render results, and both
delivered video hashes matched their delivery evidence. Final Apex samples from
rounds 1, 7, 13 and 19 also matched the source state. This production run covers
all rounds in Arena and Apex; the ten-style export comparison above covers one
complete round in each style.

The database passed its integrity check and retained all eight original job IDs.
The prior native capture document was unchanged, and its 19 clips remained
available under its immutable revision.

Local production evidence, screenshots and verification scripts are under
`.local/pr/production-ui/` in the PR worktree. These are local QA artifacts and
are not included in the installer or the landing deployment.

## Professional visual revision

Renderer `broadcast-hud-v4` uses the compact composition in the supplied
[broadcast reference](https://www.youtube.com/watch?v=dipHoYeFHr0): five stable
player slots on each side of a central score and clock, the observed player's
name, health and armor at bottom left, and their weapon and ammunition at
bottom right. The ten styles retain their own colors, shapes and accents.
All player slots fit in one 64-pixel upper row. Eliminated players have a skull
and empty health bar; there is no separate alive count or clutch counter.
The lower plates have no player portraits. Weapon icons preserve unknown
values as text rather than guessing a silhouette.

The selected player's SteamID controls both the capture contract and the HUD.
Roster order and other players' deaths never change the focus. The recorder
ends an approved death tail at the last verified frame of that same player;
it does not include a switch to another player's camera.

Three bundled font weights distinguish the primary numbers from secondary
statistics. Team accents and translucent plates keep the action area open;
the score and clock remain opaque so flashes and transitions keep their contrast.

A ten-frame damage trail highlights only the lost part of the health bar.
Numbers, live health and elimination update immediately on the source frame.
The trail retains the same source timing through trims and sponsor splits.
Telemetry remains `broadcast-hud-v2`, because its schema and extraction did not
change. The renderer version invalidates rendered media independently.

The enlarged picker shows the complete composition or a close view of the
scoreboard, observed player and weapon, with a light-background option. Transparent
previews use the same ASS rasterizer as exports, recovered from black and white
opaque mattes to preserve straight alpha and avoid libass's alpha-plane blending.

Initial renderer v3 acceptance on 2026-09-09 used source based on Studio 2.4.68 (`5033a88`)
and an isolated local Studio instance. The final real HLAE canary passed all
nine broadcast cvar checks and verified restoration of the original cvars and
configuration files. All ten full first-round exports passed the production
editor's strict video and decoded audio acceptance at 1920x1080, 60 fps and
2,440 frames (40.667 seconds). Final gameplay frames were visually inspected for
every style. The smaller inset radar, native killfeed and crosshair remained
visible.

Local Go suites for the renderer, fonts, planner, recording, worker and Full
Demo editor passed, including real FFmpeg tests for opacity, icon holes, damage
timing and the fixed scoreboard during transitions. Web unit tests, lint,
typecheck, production build and all 21 Full Demo browser tests passed. Browser
regressions use long unbroken names at 390, 1024 and 1440 px and verify adjacent
controls. The actual local app also loaded and selected all ten 1920 px previews
and exercised enlarged detail views at those three widths without horizontal
overflow.

The initial complete candidate P0 autoreview was attempted with Codex GPT-6 Astra / high.
Its preflight rejected non-UTF-8 Git output before a reviewer started;
there is no independent review result for the complete visual candidate. Local evidence is
under `.local/hud-pro/`, excluded from publication.

Full-match capture acceptance used the real Studio retry flow with all 19
Donk rounds and the same approved Arena plan. Capture revision
`9d71f9ae-5bf3-4d7f-adc8-2773c7ff5b9c` passed native attestation and exact
frame coverage for every round; all cvars and configuration files were restored.

This flow exposed two timing defects before publication. Death-tail evidence
now ends at the last confirmed POV tick, because the first unknown tick is
rejected before rendering. Complete windows retain their explicitly verified
terminal render frame and close before the next frame. That fixes the 64 Hz to
60 fps rounding case that previously captured 3,815 frames in round 9 when the
approved interval required 3,816. The real corrected capture contains 3,816.
No approved window is shortened to fit an artifact, and no frames are duplicated.
The existing exact frame and decoded-audio validators remain unchanged.

The regressions reproduce the original underfill in four of sixteen clock
phases and protect unknown POVs, unapproved tail trimming and skipped ticks.
The complete recording suite and recording/worker integration tests pass.
The recording follow-up received a separate P0-only independent review; that
limited result does not cover the complete visual candidate. Further autoreview
runs were explicitly removed from the workflow by the user.

Three grenade assets now use view boxes matching their actual paths. The real
FFmpeg regression reproduced all three being invisible inside their slots and
painting outside them; it now checks visible silhouettes and no escaping pixels.
The final asset set was rendered with real flashbang, HE and smoke states in all
ten styles. The full HUD and Full Demo editor suites pass, and regenerated
the initial picker previews stayed byte-identical because their example used other weapons.

The reference composition regenerates all ten transparent picker previews.
FFmpeg pixel regressions verify that every style paints only the upper strip
and the two lower corners, leaving the native radar, killfeed and central action
clear. Source-timing regressions continue to verify the damage trail, unknown
values and a fixed target despite roster reordering. The source and generated
catalogs are valid UTF-8, including Spanish descriptions.

Final v4 acceptance exported the complete first round in all ten styles:
2,440 frames per output, full decode, verified stereo AAC at 48 kHz, and hashes
matching the delivery documents. The real Studio app selected every new
preview and exercised all four detail views at 390, 1024 and 1440 px.
The full 19-round Arena export then passed with 56,413 frames (940.217 seconds),
decoded AAC at -14.24 LUFS / -3.63 dBTP, and working seek/play/pause in Studio.
Sampled final frames include both sides of the match and the last round.
