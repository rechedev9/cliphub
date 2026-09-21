# Full Demo intro and outro bumpers

Optional channel clips around the Full Demo program: a pre-roll before the
first captured frame and an outro after the last one. They are the usual
YouTube bookends and never touch gameplay placement.

## Plan

`options.bumpers` is a pointer with `intro` and `outro` slots
(`{enabled, video}`); it is omitted from the wire when absent so documents
approved before bumpers existed keep their hash. Each enabled slot needs a
verified video asset uploaded through `POST /api/editor/assets` with
provenance, exactly like the sponsor. The planner blocks an enabled slot
without a video or with an asset that has no verified video track.

`RebuildTimeline` places the intro at frame 0, then the rounds, then the
sponsor, then the outro after the last item. Sponsor candidates, windows and
manual frames are program frames, so an intro shifts them by its duration;
a manual sponsor frame can split a round but never a bumper. Timeline items
use role `bumper` with reason `intro-bumper` or `outro-bumper`.

## Render

A bumper item renders like the sponsor: its own clip scaled and padded onto
1920x1080 at 60 fps, no HUD, no transitions, no game or voice audio. Its audio
is the clip's embedded track when the plan evidence says it has one, otherwise
synthesized silence, so a silent outro is valid. The neon intro and outro
overlays are measured over the gameplay span between the bumpers and shifted
onto the program clock. The cover frame and cover sheet skip bumper frames,
and the silent-approved delivery gate requires bumpers disabled.

## Web

The "Intro y outro" card next to Sponsor toggles each slot and uploads its
clip with provenance. `bumpers` is optional in the plan schema and is never
defaulted in, so an approved plan without it stays approvable.
