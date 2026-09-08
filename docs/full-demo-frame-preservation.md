# Full Demo frame preservation

Studio 2.4.64 could record every round and fail near the end of rendering with
`full_demo_output_invalid: delivered video differs from 1080p60 H.264 or canonical
frame count`. The reported 19-round capture required 56,424 frames after approved
POV tail reductions, but its saved clips contained 56,403. Twelve rounds were
short by one to three frames. Raw capture metadata contained enough frames for
every round.

The capture mux combined stream-copied H.264 and AAC with `-shortest`. With
reordered video frames and a WAV slightly shorter than the video, that option
discarded video packets. The mux now preserves both source streams; the editor
still applies its approved video and audio windows. This also protects Shorts
captures without changing their existing duration-validation tolerance.

Full Demo capture completion and worker admission now require enough 60 fps
frames for every certified interval. Render admission checks each requested
interval relative to the original capture start. The editor independently
probes the localized files before preparation, so stale frame metadata cannot
hide a short input. Failures identify the round, actual and required counts and
carry the existing `recording_not_reusable:` classification.

Previously saved short clips are replaced through the existing per-round
capture reuse and immutable revision merge. Complete rounds remain reusable,
including a narrower request that fits its source. Explicit retries of the old
generic Full Demo frame error enter that reuse check. Other render errors keep
their existing retry behavior. No frames are duplicated and no approved window
or capture attestation is rewritten to conceal missing media.

The final delivery validator retains its exact frame-count requirement and now
reports the observed video properties. The worker stops after an editor failure
instead of probing its unpublished output and adding secondary missing-file
warnings.

Validation includes a real B-frame/WAV mux regression with decoded frame and
timestamp comparison; Full Demo coverage at 64, 100 and 128 ticks per second;
selective recapture, merge and reuse; an actual-file preflight with overstated
stored metadata; browser retry tests; and the Full Demo voice/music/sponsor,
mastering and complete-decode media canary using muxed B-frame sources. A
read-only replay against the reported capture selects exactly twelve rounds for
replacement and seven for reuse. That replay does not launch a new CS2 capture
or produce a completed video from the user's job.

The complete editor, recording, worker, render-plan, HTTP API and recorder/editor
CLI test packages pass, as do the 61 targeted frontend tests, TypeScript checking,
and lint checks. The production web build and all three Go media binaries build
successfully.

The live Windows verification on 2026-09-08 imported the supplied Mirage demo,
selected donk666 and captured all 19 rounds again through Studio and CS2. All
56,446 raw frames survived muxing. The certified delivery contains exactly
56,424 frames at 1920x1080/60 fps, lasts 940.4 seconds and passes complete video
and audio decoding. Its stereo 48 kHz AAC measures -14.24 LUFS and -3.25 dBTP.
Capture settings and files were restored, and the capture and render completed
without warnings. Studio playback, seeking, both enabled overlays and reaching
the final frame were verified without media errors or observed dropped frames.
The Studio MP4 download completed with 4,503,792,062 bytes. The exported file's
SHA-256 matches the validated delivery:
`808f3ed57dbba0e1e8b35fc52c57bf497a27de924929a3e9d6ed6a6d37e233ae`.
It is saved as `Mirage-donk666-Full-Demo-verified-20260908.mp4` in the user's
`Videos\ClipHub` folder.

The tested local runtime is built from release 2.4.64 plus this patch. Windows
denied writes to the Program Files installation, which retains its original
files. The patched copy runs from `%LOCALAPPDATA%\Programs\ClipHub Studio Local`
using the existing Studio profile and the `ClipHub Studio Local` desktop shortcut.
No desktop release was published.

Private diagnostic evidence, installation hashes and the live verification
receipts are under `.local/full-demo-log-review-20260908/`.
