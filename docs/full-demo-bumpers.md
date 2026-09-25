# Full Demo intro, sponsor and outro bumpers

Optional channel clips in the Full Demo program: a pre-roll before the first
captured frame, a sponsor right after the second gameplay round and an outro
after the last one. Their placement is fixed and they never consume gameplay
frames.

## Plan

`options.bumpers` is a pointer with `intro`, `sponsor` and `outro` slots
(`{enabled, video}`). The pointer and the `sponsor` slot are omitted from the
wire when absent, so documents approved before either existed keep their
hash. Each enabled slot needs a verified video asset uploaded through
`POST /api/editor/assets` with provenance. The planner blocks an enabled slot
without a video or with an asset that has no verified video track.

`RebuildTimeline` places the intro at frame 0, then the rounds with the
sponsor inserted after the second non-empty round, then the outro after the
last item. A program with a single round plays the sponsor after it and
carries the `sponsor_after_last_round` warning. Timeline items use role
`bumper` with reason `intro-bumper`, `sponsor-bumper` or `outro-bumper`.

The sponsor used to be a separate `options.sponsor` block with placement
policies, windows and replacement narration, plus a document-level
`sponsor_placement`. Decoding drops those retired fields so render history
still reads; a stored draft plan that has them reads as absent and the
producer plans again from the defaults. Timeline items with the old `sponsor`
role only appear in those historical documents.

## Render

A bumper item plays its own clip scaled and padded onto 1920x1080 at 60 fps,
with no HUD and no game or voice audio. Its audio is the clip's embedded track
when the plan evidence says it has one, otherwise synthesized silence, so a
silent clip is valid. Every cut between gameplay and a bumper gets the
dynamic transition, even when round effects are off. The neon intro and outro
overlays are measured over the gameplay span between the leading and trailing
bumpers and shifted onto the program clock. The cover frame and cover sheet
skip bumper frames, and the silent-approved delivery gate requires bumpers
disabled.

## Web

The "Intro, sponsor y outro" card uploads each clip with provenance.
`bumpers` and its `sponsor` slot are optional in the plan schema and never
defaulted in, so an approved plan without them stays approvable.
