# Full Demo round transitions

The **Entre rondas** panel adds optional visual and sound effects to the approved
Full Demo plan. The default remains a clean cut. Suave, Dinámico and Solo sonido
are starting points; each effect can then be changed independently.

| Decision | Choices |
| --- | --- |
| Transition duration | 6–18 frames at 60 fps (100–300 ms), default 8 |
| Whip pan | Left, right, up, down, alternating, or estimated screen motion |
| Whip strength / directional blur | 2–20%; 0–64 px |
| Punch-in | 105–115% scale; cut, last player kill, or round-end event |
| Microflash / RGB accent | Shared 2–4 frame window; 2–20% brightness; 1–6 px RGB shift |
| Whoosh | Generated filtered pink noise, −36 to −6 dB |
| Impact | Generated bass, −36 to −6 dB, 35–90 Hz, 80–400 ms |
| Comms continuation | 0–1.5 seconds of source team voice after the outgoing cut |
| Game audio | 0–250 ms fades; optional 200–12,000 Hz outgoing low-pass |

## Runtime and timeline

The runtime remains **Go + FFmpeg**. No Python, editor plugins, downloaded SFX,
or new runtime packages are required. `zoompan` supplies a frame-clock S curve
and crop margin for the pan; `avgblur`, `eq`, and `rgbashift` provide the accents.
Audio sources and filters generate and mix the swish and low impact before the
existing full-program AAC mastering and decoded-output validation.

Effects decorate existing frames rather than overlapping clips. Frame/sample
counts, kills, music progression, overlay positions, and fixed two-second freeze
context remain unchanged. Only adjacent different round items receive a cut
effect. Sponsor entries/exits, split continuations, and program endpoints are
excluded. Very short items clamp the effect window without changing coverage.

Event-anchored zooms use existing kill or round-end facts and receive a paired
whoosh. An event outside the retained piece falls back to the cut. A round-end
event is the demo's observed outcome boundary, not an independently inferred
defusal. Motion following estimates translation in a central HUD-free crop of
the outgoing second (at most 12 grayscale 96×54 frames). It is not optical flow
or a claim about crosshair intent. Quiet/ambiguous frames alternate left/right;
decode errors fail the render. Applied direction and provenance are recorded in
render evidence and validated against the planned boundaries.

Comms continuation reads new source samples after the outgoing editorial end;
it does not repeat the last syllable. The tail fades into the next round's
preparation and is bounded by the incoming source start and item duration. Game
audio does not carry over. Voice disable/zero gain and unavailable-voice fallback
remain authoritative. The full voice extraction retains its existing team policy.

`options.transitions` is an optional complete object. Absence preserves the
original JSON and hash of historical documents. Present values reject missing,
unknown, duplicate, fractional integer, non-finite and out-of-range decisions.
Changing effects invalidates editorial approval but preserves compatible capture
reuse. Enabled SFX cannot be accepted under the silent-program exception.

## Verification

- Go tests cover strict options, approval/capture hashes, ad/split boundaries,
  short rounds, event anchors, voice-tail bounds, and direction estimation.
- Actual FFmpeg tests compare decoded frames for four pan directions, cut/event
  zoom, flash and RGB, including unchanged frames outside the effect window.
- A synthetic full-program canary mixes voice tails, SFX and a sponsor, then
  validates 544 decoded frames and stereo 48 kHz AAC at −14 LUFS ±0.5 and
  true peak ≤ −1.5 dBTP through the production delivery gate.
- Browser tests cover saving/reload/generation and 390/1024/1440 px layouts with
  a long unbroken player name. The current local build was also exercised with
  a freshly imported real Mirage demo and a saved 19-round donk666 plan.
- Six real-capture review excerpts each passed the unchanged 480-frame/8-second
  delivery gate. They reuse existing recordings with the supplied demo's SHA-256;
  they do not constitute a new HLAE capture or an installed-app release check.
  An older R1 clip had 2,439 frames for a 2,440-frame tick window. Review excerpts
  use available source bounds; full-job admission remains strict.

Run the focused checks:

```powershell
go test ./internal/recapplan ./internal/editor -run 'TestTransition|TestFullDemoTransition' -count=1
cd web
pnpm run typecheck
pnpm run lint
pnpm exec node --test lib/full-demo-plan.test.ts lib/full-demo-transitions.test.ts
pnpm exec playwright test full-demo.spec.ts
```

To regenerate the optional real review loops (requires local NVENC and FFmpeg):

```powershell
$env:FULL_DEMO_TRANSITION_JOB_DIR = '<local data>/jobs/<recorded job UUID>'
$env:TEST_DEMO_PATH = '<matching demo.dem>'
$env:FULL_DEMO_EVIDENCE_DIR = '<review output directory>'
go test ./internal/editor -run '^TestFullDemoTransitionsRealCapturePreview$' -v -count=1
```

Filter reference: [FFmpeg documentation](https://ffmpeg.org/ffmpeg-filters.html).
