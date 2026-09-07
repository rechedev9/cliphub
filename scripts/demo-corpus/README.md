# Offline demo corpus

Run the production parser for every roster player in a directory of `.dem` and
`.dem.zst` files, deduplicating decoded content by SHA-256:

```powershell
go run ./scripts/demo-corpus --input C:/Users/reche/Downloads --out .local/demo-corpus-new --workers 2 --timeout 10m
```

The output directory must be new and outside the input directory; its parent
must exist. Source files are read only. Each unique demo runs in an isolated
child process with a timeout, so a parser crash cannot stop the remaining
corpus. Compressed inputs are decoded into the private report directory.

Reports retain the roster, parser output, Full Demo plans and each failed
contract check. `summary.json` contains the full result. A successful command
means the report was written: inspect every `error`, `blocked` and `empty`
status before interpreting coverage as successful.

Checks cover three HUD profiles with software and NVENC configuration, Full
Demo observed crosshair with and without an explicitly allowed default,
persisted-plan JSON round trips, strict recording-plan equality, capture
fingerprints and HLAE script generation. Voice, music and sponsor are disabled
for these capture checks. The tool never starts Studio, CS2, HLAE, FFmpeg, voice
extraction or a render. Use the separate real-editor CLI regression tests to
cover executable flag defaults.

## Downloads corpus, 2026-09-07

27 source files yielded 25 unique demos, seven maps and 250 player/demo cases
(109 distinct players). All roster and player parses completed with no parser
or facts-validation errors. The 2,473 checks produced 2,336 passes, 104 errors,
27 blockers and six empty selections.

The 104 errors repeat one duration-boundary discrepancy across nine demos and
26 player/demo cases, with four profile combinations each. Full Demo facts
track the actual final frame, while the legacy kill-plan duration uses the
last tracked event. The adapter retains that shorter duration, so recording
plan validation rejects a valid post-round tail before capture starts. For
example, `bestia-vs-gremio-m1-inferno.dem` has facts ending at tick 222261, a
legacy duration of 221876 and an approved last-round capture ending at 222004.
This is separate from the editor CLI default failure and remains a documented
product finding; the corpus does not silently extend or clamp either plan.

Twenty blockers are the incomplete first part of the split
`1-b5604ae7-c676-454b-901a-0b02014abd94` demo, across ten players and both
crosshair policies. Seven blockers concern unavailable observed crosshairs;
the explicitly approved fallback removes those blockers. Six empty checks
are one player's zero-kill selection across HUD/encoder combinations.

Coverage included 5,170 player-round observations: 2,780 without kills, 1,198
without utility, 210 with round numbers above 24, and one death before live
play. These counts overlap and are not distinct rounds or matches.

The complete private report is `.local/demo-corpus-20260907/summary.json`.
Demos, player output and machine-specific report files are excluded from Git.
