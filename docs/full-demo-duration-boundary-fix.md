# Full Demo: approved tails beyond legacy duration

The offline Downloads corpus found 104 recording-plan failures across nine
demos and 26 player/demo cases. Each case failed under both crosshair policies
and both encoder configurations with `segment ... tick_end ... exceeds demo
duration ...` before starting capture.

The parser's legacy kill plan uses the last tracked event as its duration.
Full Demo facts independently follow the final source frame. A valid approved
round/death tail can therefore extend past the legacy duration while remaining
inside the demo. For example, the Inferno BESTIA–Gremio demo has a last event
at tick 221876, source frames through 222261, and an approved round tail through
222004. The adapter previously copied the 221876 duration unchanged.

`Document.KillPlan` now raises the adapted duration only when an approved
capture end exceeds it. This records a known minimum extent of the source;
it does not infer extra footage or move the approved end. Existing sufficient
durations, the original Shorts plan and the approved document remain unchanged.
Both capture and render workers already use this adapter, including for saved
plans from earlier versions. No schema, planner hash or approval migration is
needed, and the recording validator still rejects out-of-range legacy plans.

Regression tests cover round tails, death tails, tails capped by source EOF,
already sufficient durations, JSON persistence, software/NVENC profiles and
HLAE script generation. The tests reproduced the duration mismatch before the
fix. The affected PC's previously committed 14-round recording also retains
the identical capture fingerprint after adaptation, so this correction does
not invalidate that recording.

Replaying the saved 250 player/demo cases repairs all 104 failing Full Demo
configurations. The 842 already valid configurations produce identical
recording plans and capture fingerprints, and all 27 blocked plans remain
blocked. Approved documents are byte-for-byte unchanged by adaptation; every
repaired recording bound remains within the source end established by facts.
