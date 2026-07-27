# Full HUD Native Killfeed Design

## Status

Approved in conversation on 2026-07-21 and corrected on 2026-07-27 after
vertical Full HUD review showed that CS2 safe-zone commands compress the entire
HUD, not only death notices.

## Problem

Vertical reels recorded with the `full-hud-60` preset preserve the gameplay HUD but do not configure HLAE death notices.
The editor therefore uses its legacy FFmpeg crop-and-overlay fallback to copy the source killfeed into the vertical frame.
That fallback can capture unrelated kills, multiple accumulated rows, partial rows, and frozen notices.
The resulting killfeed is visually broken and does not represent only the selected player's kills.

## Root Cause Evidence

The affected Nuke job `56af7651-8013-4f99-894a-f88cdfd33a5d` persisted `hud_mode: gameplay` without `portrait_safe_killfeed`.
Its generated `recording.js` enables `cl_draw_only_deathnotices 0` but contains no `mirv_deathmsg`, `safezonex`, or `safezoney` commands.
Its `full-hud-60` edit manifest persisted `killfeed_overlay: true` and generated cropped killfeed effects as tall as 222 source pixels.
The manifest also reported six missed notice detections, confirming that render-time probing is not reliable for this capture mode.
The working `viral-60-clean` path persists `portrait_safe_killfeed: true`, filters death notices through HLAE, positions them inside the vertical crop, and does not add render-time killfeed effects.

## Decision

Vertical `Full HUD` capture will preserve CS2's native 16:9 HUD layout.
The 9:16 center crop will retain central context such as the round score,
timer, and overtime state while naturally cropping lateral elements such as the
radar, health, and ammunition instead of moving them into the gameplay.
HLAE will still filter death notices to kills made by the selected target
player, but it will not apply `safezonex` or `safezoney` in gameplay HUD mode.
The editor will crop those filtered notices from their native source position
and overlay them inside the portrait frame.

## Data Flow

For a 9:16 request, the HTTP admission layer will set `portrait_safe_killfeed` when the selected preset declares `KillfeedSource`.
This includes `viral-60-clean` and `full-hud-60`, but excludes `clean-pov-60`.
Landscape requests will retain their current behavior.

The task payload will continue carrying the existing `portrait_safe_killfeed` boolean.
No new persisted field, enum, endpoint, or migration is required.

The record worker will include `--portrait-safe-killfeed` for both supported killfeed HUD modes, `deathnotices` and `gameplay`.
It will include the effective flag in the normalized recording profile used by durable-artifact reuse checks.

## Recording Behavior

The recording contract will normalize safe-zone defaults only when
`portrait_safe_killfeed` is enabled for deathnotice-only HUD.
It will normalize the death-notice lifetime for `deathnotices` captures and portrait-safe `gameplay` captures.
Validation will accept portrait-safe killfeed only for `deathnotices` or `gameplay` HUD modes and will require a valid target SteamID64 whenever HLAE filtering is needed.

The HLAE script will keep HUD visibility independent from killfeed filtering.
`deathnotices` mode will continue to use `cl_draw_only_deathnotices 1`.
Portrait-safe `gameplay` mode will continue to use `cl_draw_only_deathnotices 0`, preserving radar, health, ammunition, weapon, team panels, and the rest of the gameplay HUD.

Both paths that configure a filtered killfeed will run `mirv_deathmsg clear`, clear prior filters, block notices whose attacker is not the selected target, and set the local-player and lifetime behavior.
Only deathnotice-only portrait capture will apply and later restore the configured safe zone.
Gameplay HUD capture will never write `safezonex` or `safezoney`.
Plain gameplay capture, including existing landscape behavior, will not gain HLAE filtering unless portrait-safe killfeed is requested.

## Render Behavior

The editor will treat a validated portrait-safe deathnotice-only recording as
containing a native killfeed that is already visible in the vertical frame.
For gameplay HUD, it will enable `KillfeedOverlay` and emit one
`EffectKillfeed` per selected kill so the filtered native notices remain
readable without changing the rest of the HUD layout.

Legacy recordings without the flag will retain the existing crop-and-overlay fallback.
This preserves render-only compatibility for persisted captures whose native notices remain outside the center crop.

## Artifact Reuse

The normalized stream profile includes `PortraitSafeKillfeed`, safe-zone geometry, and death-notice lifetime.
An old `full-hud-60` gameplay recording carrying non-zero deathnotice safe-zone
geometry therefore will not satisfy the new vertical capture profile.
A new generation will recapture rather than silently reuse the broken source material.
The already-rendered Nuke reel remains unchanged until an explicitly approved recapture is run with updated binaries.

## Error Handling

Invalid combinations such as portrait-safe killfeed with `clean` HUD mode will fail during recording-plan validation before HLAE launches.
The existing HTTP preset validation and task HUD-mode validation remain authoritative.
No new runtime retry behavior or pipeline failure class is introduced.

## Test Strategy

HTTP handler tests will assert that vertical `full-hud-60` and `viral-60-clean` requests set the portrait-safe flag while `clean-pov-60` and landscape requests do not.
Recording type tests will cover gameplay safe-zone and lifetime defaults plus rejection of invalid clean-HUD combinations.
Script-generation tests will assert that portrait-safe gameplay keeps
`cl_draw_only_deathnotices 0`, emits the target-only HLAE filter, and never
emits safe-zone commands.
Script-generation tests will also assert that plain gameplay capture remains unchanged.
Worker tests will assert that portrait-safe gameplay reaches both the recorder CLI and the expected durable recording profile.
Editor manifest tests will assert that portrait-safe gameplay recordings retain
the crop-and-overlay killfeed path while deathnotice-only recordings keep their
native portrait notices.

Focused Go tests will run first for `internal/recording`, `internal/httpapi`, `internal/workers`, and `internal/editor`.
The final verification will run the repository Go gate without formatting unrelated worktree changes.
No real HLAE/CS2 capture or long FFmpeg render will run without separate explicit approval.

## Out Of Scope

This change intentionally keeps the other Full HUD elements in their native
16:9 positions and accepts lateral cropping in portrait output.
It does not synthesize a custom killfeed from kill-plan data.
It does not include assists or kills by other players.
It does not modify existing generated media in place.
