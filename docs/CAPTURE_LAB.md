# Capture Lab

## Purpose

Capture Lab is ClipHub's executable verification laboratory for demo-to-publish behavior without launching HLAE or CS2 during ordinary development loops.

It does not claim that a simulator proves current HLAE/CS2 compatibility. It proves the deterministic product behavior on both sides of that external boundary, executes the exact generated HLAE JavaScript against a simulated `mirv` API, renders real synthetic media through FFmpeg, and preserves a small explicit real-capture canary for changes that cross the boundary.

The laboratory exists so an agent can implement, execute the application, inspect evidence, fix failures, and repeat without treating code review or unit tests alone as proof that a user goal works.

## Safety invariants

1. Simulated and fake captures never become reusable production captures.
2. `recording.ValidateUploadResult` continues to require `capture_mode: "real"`.
3. The laboratory never fabricates a production attestation or changes a result to `capture_mode: "real"`.
4. Generated media stays in an ignored evidence directory and is never committed.
5. The default verification loop never launches HLAE, CS2, an external network service, or paid/cloud work. L4 binds ephemeral loopback-only local HTTP services.
6. A real-capture canary is explicit, Windows-only, and outside the default loop.
7. The simulator executes the generated `recording.js`; it does not maintain a second copy of the capture state machine.

## Verification model

```text
kill plan fixture or local .dem
        |
        v
real recording plan + exact recording.js
        |
        +--> MIRV simulator --> command/event transcript --> contract assertions
        |
        +--> instrumented MP4 segments --> real FFmpeg render/composition --> media oracle
        |
        +--> real orchestrator + Studio test mode --> Playwright user journey

rare Windows run: HLAE + CS2 --> sanitized performance trace + compatibility certificate
simulator transcripts --> normalized replay corpus
```

### Confidence levels

- **L1 — deterministic:** Go/TypeScript unit, contract, mutation, and chaos tests pass.
- **L2 — script exercised:** the exact generated HLAE JavaScript passes success and failure scenarios in the MIRV simulator.
- **L3 — media exercised:** real FFmpeg encodes, composes, and probes instrumented segment media; automated oracles verify identity, order, timing, dimensions, streams, and duration.
- **L4 — application boundaries exercised:** real HTTP queue/render tests exercise service behavior, while a `capturelab` build-tagged orchestrator serves the already validated synthetic render through real handlers, same-origin Next proxies, Studio Library, and a browser download hash oracle. These are composed stages, not one continuous production worker lifecycle.
- **L5 — external boundary certified:** an explicit real HLAE/CS2 canary passes for the current boundary fingerprint.

L4 without L5 must be reported as “simulated end-to-end verified; current HLAE/CS2 compatibility not recertified.”

## Hosted backend CI

`CI backend` uses Ubuntu 24.04, installs Node 24 and the distribution's
FFmpeg/ffprobe, and runs the simulator's Node
contracts plus the complete Go suite (five-minute timeout per package). It does
not need a GPU, Redis, Steam credentials, a real demo, or a browser.
`scripts/ci-backend-evidence.mjs` reads the Go JSON output and requires successful,
unskipped execution of the generated-script simulator and three media canaries:
Shorts portrait, Full Demo overlays, and two-round Full Demo concatenation.
Missing tests and skipped simulator scenarios fail the lane. Changes to the
simulator or evidence checker also trigger backend CI.

These canaries exercise L2 and selected L3 seams. They check real encoded H.264
video at 60 fps, stereo AAC, stream duration, complete decoding, and 9:16/16:9
geometry. Full Demo concatenation must retain both rounds' duration. The complete
suite also runs existing HTTP admission, worker failure/publication, persistence,
and interrupted-work recovery tests. This is not a continuous real capture flow
or L5 HLAE/CS2 certification; the Windows canary remains separate.

To reproduce the evidence gate locally with Go, Node and FFmpeg on PATH:

Use a build with `drawtext` and `filter_complex_script` support. On macOS,
Homebrew's `ffmpeg@7` works with `PATH="/opt/homebrew/opt/ffmpeg@7/bin:$PATH"`.
The minimal FFmpeg 8 Homebrew build may omit `drawtext`; FFmpeg 9 rejects the
pipeline's existing `filter_complex_script` option. Those are failures to
investigate, not reasons to skip the required canaries.

```sh
node --test scripts/capturelab/*.test.mjs scripts/ci-backend-evidence.test.mjs
# Use a disposable log outside the repository; pipefail preserves Go failures.
set -o pipefail
go test ./... -count=1 -timeout 5m -json | tee /tmp/cliphub-backend-tests.jsonl
node scripts/ci-backend-evidence.mjs /tmp/cliphub-backend-tests.jsonl
```

## Components

### 1. MIRV simulator

Location: `scripts/capturelab/`.

The simulator uses Node's built-in JavaScript sandbox. This is not an operating-system VM. It runs as a normal Node process, has no GPU requirements, and needs neither HLAE nor CS2.

It injects the minimum API surface used by `recording.js`:

- event registration/unregistration;
- demo playback state and tick;
- entity/controller/pawn handles;
- observed SteamID resolution;
- `mirv.exec`, `message`, and `warning` collection;
- deterministic client-frame delivery.

A scenario drives frames and declares expected outcomes. The transcript records commands, messages, warnings, ticks, segment boundaries, seeks, attestations, and quit behavior.

Initial required scenarios:

- healthy single segment;
- healthy multiple segments with seek;
- delayed seek landing;
- unknown observer within tolerance;
- unknown observer beyond tolerance;
- wrong observer SteamID;
- target controller temporarily missing;
- demo ends after all segments complete;
- demo ends before completion;
- soft quit occurs on later client frames, never in the disconnect frame;
- repeatability: the same scenario produces byte-identical normalized evidence.

### 2. Instrumented capture media

`zv-recorder --fake` already creates genuine FFmpeg MP4 clips and stamps `capture_mode: "fake"`. Capture Lab extends their instrumentation rather than inventing a production bypass.

Each synthetic segment should carry machine-readable truth:

- deterministic motion pattern and identity-color bar per segment;
- machine-readable segment ID in container metadata;
- a brief visual pulse at independently derived kill offsets;
- a deterministic audio tone per segment;
- normal video/audio streams with the plan's dimensions and frame rate.

The media oracle uses `ffprobe` plus decoded sample hashes/statistics. It verifies:

- expected segment set and editorial order;
- non-empty, decodable video and audio;
- codec, dimensions, FPS, channels, and sample rate;
- duration tolerance;
- sampled windows across every segment contain motion and non-silent audio (this detects fully frozen/silent segments, not every possible sub-frame stall);
- event pulse timing after composition/render;
- output pack paths and metadata remain internally consistent.

Synthetic media is never upload-ready.

### 3. Application journey

The test journey must use product entry points rather than calling UI components directly:

1. generate fake-provenance MP4 segments and render them through the real editor CLI;
2. validate source and rendered media before any application seed exists;
3. run CLI chaining, real HTTP render, and inline-queue/shutdown behavior tests;
4. build the orchestrator with the explicit `capturelab` Go build tag;
5. confine a coherent fake recording/result/pack seed to one evidence root and an in-memory repository;
6. start the actual orchestrator on loopback with a random session capability;
7. start the production standalone Next server and exercise its same-origin proxies;
8. inspect the Studio Library terminal state and hash the downloaded MP4 against the validated render;
9. shut every owned process tree down and retain browser/service evidence.

This intentionally does not claim that a single worker lifecycle ran from upload to render. The fake result cannot pass production recording validation, so the build-tagged seam starts only the final HTTP/UI boundary from an already validated result. Production orchestrator builds omit the loader and fail closed if its environment variables are present. The seed additionally requires an in-memory repository, a loopback listener, no network credentials, and canonical containment under `ZV_CAPTURE_LAB_EVIDENCE_ROOT`.

### 4. Trace replay

A trace is a sanitized JSON sequence of the HLAE observations that affect script behavior. It contains no credential and no media.

Required fields include:

- schema version;
- capture-boundary fingerprint;
- HLAE version and CS2 build supplied by the explicit canary runner;
- demo tick / playback state by relevant frame;
- observed controller/SteamID state;
- executed commands and emitted markers;
- final outcome.

Replay runs the current generated script against the recorded observations and diffs the normalized transcript. Trace files are committed only when their provenance is safe and useful; local traces may remain untracked.

### 5. Compatibility certificate

A successful explicit real canary writes a local certificate bound to:

- capture contract version;
- HLAE version;
- CS2 build;
- rebuilt `zv.exe`, HLAE, and CS2 executable hashes;
- capture-relevant recorder, unified CLI, build-script, and plan-contract source fingerprint;
- generated script fingerprint;
- exact argv as a JSON array, validated against the result's demo/output/kill plan;
- demo, kill plan, result, and segment artifact fingerprints;
- timestamp and exact canary command;
- terminal validation result.

The certificate is evidence, not an override. It never changes validation behavior. Any fingerprint change invalidates the certificate and reports which boundary input changed.

## Commands

The final operator surface should converge on:

```powershell
# Default, safe, repeatable laboratory loop. Never launches HLAE/CS2.
.\scripts\capture-lab.ps1 -Mode Full -Iterations 1

# Fast exact-script scenarios only.
.\scripts\capture-lab.ps1 -Mode Script

# Real FFmpeg synthetic-media path.
.\scripts\capture-lab.ps1 -Mode Media

# Local service + Studio journey.
.\scripts\capture-lab.ps1 -Mode Studio

# Explicit Windows canary; not part of Full. Supply every factual input/profile value.
.\scripts\capture-lab-real-canary.ps1 `
  -KillPlan .\plan.json -Demo .\match.dem -OutDir .\canary `
  -HLAE C:\HLAE-<version>\HLAE.exe -CS2 C:\...\cs2.exe `
  -HLAEVersion <installed> -LatestHLAEVersion <official-latest-checked> `
  -CS2Build <build> -HUD deathnotices -PortraitSafeKillfeed $true `
  -TimeoutSeconds 1800 -ApproveRealCapture

# Recheck a local certificate against current external versions and every hashed file.
node scripts/capturelab/certificate.mjs check `
  --certificate .\canary\capture-compatibility-certificate.json `
  --hlae-version <installed-now> --cs2-build <build-now>
```

Cross-platform component commands must remain available directly through Go, Node, and pnpm so Linux development does not depend on PowerShell.

Every run writes a bounded evidence bundle containing:

- `summary.json`;
- component test results;
- simulator transcripts;
- media probes and oracle results;
- service/browser logs when applicable;
- hashes of relevant inputs and outputs;
- a human-readable `SUMMARY.md`.

## Agent loop contract

For an applicable change, the agent must:

1. classify which confidence level the change requires;
2. run the cheapest relevant level first;
3. on failure, preserve evidence, identify one concrete cause, fix it, and rerun;
4. continue until the required levels pass or an external/user decision is genuinely required;
5. never report “verified end-to-end” without naming the highest completed level;
6. state explicitly whether HLAE/CS2 compatibility was recertified or inherited from an unchanged boundary fingerprint.

A default loop has a finite per-command timeout and a configurable iteration count. “Continue until green” means repeated diagnosed fixes, not an unbounded retry of the same failing command.

## Implementation checklist

### Phase A — exact-script simulator

- [x] Define versioned scenario and transcript schemas.
- [x] Implement the simulated `mirv` surface with deterministic entity handles.
- [x] Execute the exact generated `recording.js` without modifying it.
- [x] Implement healthy, seek, POV failure, EOF, and soft-quit scenarios.
- [x] Correlate markers with balanced low-level `record start/end` commands and attest only after closure.
- [x] Add Node unit tests and fixture validation.
- [x] Add a Go integration test that generates the script and invokes the simulator.
- [x] Prove deterministic normalized transcript replay.

### Phase B — instrumented media and oracle

- [x] Make fake segment patterns deterministic and segment-specific.
- [x] Add visual kill pulses and per-segment audio identities without introducing a runtime dependency.
- [x] Implement probe and decoded-sample media oracle.
- [x] Derive IDs, shape, colors, tones, durations, and event offsets independently from the recording plan and fake-backend contract.
- [x] Exercise a two-segment composition and detect fully frozen/silent segments, wrong identities/order/timing, static false pulses, and publication-copy mismatches.
- [x] Exercise capture artifact validation without weakening production upload validation.
- [x] Exercise real composition/editor behavior over fake-provenance MP4s.
- [x] Keep every generated artifact ignored.

### Phase C — service and Studio boundaries

- [x] Define a Go-build-tagged, in-memory-only ready-result seed omitted from production builds.
- [x] Confine every served seed file to the evidence root and validate cross-artifact coherence.
- [x] Start/stop actual loopback local services with isolated state and a random capability.
- [x] Add real HTTP CLI/render/inline-queue coverage.
- [x] Add Playwright coverage through Studio and same-origin proxies with a byte-hash download oracle.
- [x] Capture browser, service, and task evidence.
- [x] Kill owned process trees on timeout/shutdown and test grandchild cleanup.
- [x] Document that worker lifecycle and final Studio boundary are composed evidence, not one continuous transaction.

### Phase D — trace and certification

- [x] Define a normalized, credential-free simulator transcript schema.
- [x] Replay the current exact script and diff its canonical transcript against the baseline.
- [x] Define capture-boundary fingerprint inputs from non-test recorder, recording, unified CLI, local contract dependencies, build script, and canary source.
- [x] Implement local certificate generation and invalidation diagnostics.
- [x] Require Go attempt/upload validation against the exact kill plan, executable/demo/plan/artifact hashes, and exact argv JSON before certificate issue.
- [x] Add the explicit Windows real-canary command and keep it outside `Full`.
- [x] Preserve the certificate/result/evidence directory for dual-boot handoff. A real canary trace contains the recorder's sanitized performance events; full per-frame observer replay remains an external-boundary limitation.

### Phase E — one-command loop and agent policy

- [x] Add the PowerShell laboratory runner with modes, timeouts, iterations, and evidence retention.
- [x] Add direct cross-platform commands.
- [x] Write `summary.json` and `SUMMARY.md` for every run, deriving level only from completed phases.
- [x] Serialize runs per checkout, bound captured output, scrub inherited credentials, and terminate process groups/trees.
- [x] Require matched Go tests rather than trusting an empty `go test -run` success.
- [x] Add one concise rule and link in `CLAUDE.md` (never replace the `AGENTS.md` symlink).
- [x] Run focused checks, Go gates, package checks, and the completed laboratory.
- [x] Record remaining external limitations honestly.

## Completion definition

Capture Lab is complete when a clean checkout can, without HLAE or CS2:

1. generate the exact HLAE script from a factual plan;
2. execute it through all required deterministic simulator scenarios;
3. generate and validate authentic instrumented MP4 segments;
4. exercise queue/render application behavior and the final HTTP/UI boundary as explicitly composed stages;
5. verify the Studio-visible terminal outcome and downloaded bytes through real same-origin proxies;
6. produce a self-contained evidence bundle;
7. reject every attempt to treat simulated evidence as a production-real capture;
8. explain whether a real compatibility certificate is current, stale, or absent.

The explicit real canary completes L5 but is not required for the default L1–L4 laboratory loop.
