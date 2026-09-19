# Full Demo post-recording performance audit

Date: 2026-09-16. Source baseline: `21723c4`. Scope: rendering after existing, verified recordings. HLAE, CS2, recording settings, capture timing and capture implementation are outside this work. Completed implementation and validation appear first; the original audit and proposals are retained below as a clearly marked historical record.

## Implemented changes (post-audit, 2026-09-16)

The pre-implementation baseline below is kept as written, including its ranked
proposals and historical recommendations, as the pre-change record. The four
ranked opportunities were subsequently implemented and validated; this section
records what changed and the root evidence. A paired full render was accepted
(30.207 % wall reduction, see "Paired full render") and the integrated media gates
passed. Targeted NVENC and software resource sweeps are complete. Their evidence
supports keeping the current 3-worker automatic-threads defaults.

### 1. Shared delivery decode with optional diagnostics

The mandatory complete video+audio delivery decode (strict `-xerror`,
`-fps_mode passthrough`, canonical frame count, zero duplicates/drops,
probe/durations, SHA256) now also runs the existing `blackdetect`/`freezedetect`
filters in the same process, removing the second full-video quality decode.
`-v info` is used only for those event lines; the strict decode command is
otherwise unchanged. If the optional filter graph fails to configure on its
own, the renderer retries the strict decode without it, keeps both the original
setup stderr and the retry log, and reports a quality warning; a genuine
media/decode error still fails delivery.

### 2. Bounded, deterministic voice preparation

Independent voice tracks now run in a small CPU-aware worker pool (at most 3,
also bounded by CPU count). Each track keeps its unchanged full-track
measurement, gain bound, 3 dB peak headroom and one atomic PCM WAV
materialization. Results are stored by original index, so `voicePaths` and
`TrackLevels` keep approved order regardless of completion order; the first
error cancels the pool. Aggregate progress averages every track's two phase
fractions and reserves 1 for full completion, and is clamped strictly below 1
with `Nextafter` while any track is unfinished.

### 3. Overlay consolidation: one encode instead of two

Supported global `full-demo-intro`/`full-demo-outro` image overlays are now
composed per timeline item, after transitions and custom HUD, and the program
concat copies the compatible H.264 stream. The item base is placed on an
explicit canonical integer global frame clock
(`settb=expr=1/60,setpts=N+StartFrame`, then `settb=expr=1/60,setpts=N` for the
local reset) and the original whole-program overlay graph is reused unchanged,
so slide/fade/dim animation, inclusive end frames, overlapping and short
windows, sponsor inserts, certified trims and inactive-frame colour conversions
all stay on the global clock. A float `StartFrame/60/TB` shift is not exact
(start 123 emits first PTS 122); the explicit integer clock avoids that
off-by-one.

This is **not byte-identical**. The legacy path encodes items once and then
decodes and re-encodes the whole program with the overlays (two encode
generations); the new path encodes each item once with the overlays and copies
the program (one generation). Visual quality therefore has to be compared
against a common reference, not asserted byte-equal.

Unsupported combinations keep the legacy post-concat re-encode pass unchanged:
any non-image effect, the program-global cover first-frame freeze, a missing
overlay still, or a non-finite window. Effect order is preserved, and the copy
path is only used once the prepared item list replaces the raw-part list.

### 4. Audio-only candidates with a final muxed AAC measurement

The accepted candidate sequence and decoded-AAC acceptance rules are retained,
but audio candidates are rendered and measured without repeatedly copying the
multi-gigabyte video; after one candidate passes it is muxed once with the
finalized video, and the existing evidence path measures the final muxed AAC.
Native attempts and the bounded Media Foundation recovery keep their existing
contract.

### Timing evidence

The existing FFmpeg diagnostic execution now also records per-render in-memory
spans (stage, original item/track index, attempt, encoder, media duration,
interval, outcome) for `transitions`, `voice_analysis`, `voice_prepare`, `items` and the audio
stages. Wall time is the union of active intervals, which excludes gaps between
spans. `ProcessElapsedSumMS` is the sum of each subprocess's wall elapsed time —
serialized process time, **not CPU time** and not overlap-aware wall time.
`RenderMS` remains the total render measurement, so the per-stage numbers must
not be added to it. `QualityCheckMS` for Full Demo is now the combined decode
elapsed (delivery plus the optional black/freeze diagnostics in one process), so
it is not separately additive and is not directly comparable to the old
quality-check timer.

### Verified gates (root, 2026-09-16)

These are root results, not worker development checks.

- Integrated four-package suite PASS: editor 183.462s, demooverlay 3.062s,
  customhud 1.721s, recapplan 0.134s.
- Targeted overlay suite 93.602s passed every actual production behavior, but
  the sensitivity fixture failed; the corrected fixture was independently
  re-run and PASSed in 13.626s. Direct integer PTS for starts 123/245/246 and
  the pre-encode pixel comparisons are exact.
- Encoded topology against the common lossless reference: software CRF16/`slow`
  legacy two-generation 30.92 dB versus new one-generation 30.85 dB; native
  p5/cq16 legacy two-generation 28.85 dB versus new one-generation 30.86 dB.
  These are recorded local results. The NVENC subtest was subsequently removed
  from the automated suite at the user's request; the software topology test
  remains. Production NVENC rendering is unchanged.
- Exact PCM join and pipeline integration checks pass.
- Race checks for the voice pool, timing collector and evidence paths pass;
  `go vet` is clean.
- Final worker-to-editor CLI checks PASS in 4.497s, including strict legacy flag
  rejection, render-hash behavior and rejection of source frame counts that do
  not match saved metadata. No tests in this focused run were skipped.
- Baseline full render final: 609.4440397s wall (render 605693ms), functionally
  identical to the previous functional baseline of 621.9238475s. Both full
  delivery objects and decoded-AAC evidence are IDENTICAL, including SHA-256
  `5bdab9d587fbdf29a53a7ba6b65972219954e3c0502b4832aae2626fc95d4851`, 55836
  frames, 930.6s, 48 kHz stereo.
- Root generator full real-voice hash determinism PASS in 69.485s; the hash
  self-check passes at 1 ULP.

Two baselines must be distinguished: the **functional baseline** (621.9238475s)
is the prior working render used for behavioral comparison, and the **timed
baseline** (609.4440397s) is the root-run measurement taken as the paired
reference for the candidate.

### Paired full render (root, 2026-09-16)

| Metric | Timed baseline | Candidate (final) | Delta |
| --- | ---: | ---: | ---: |
| Post-recording wall | 609.444 s | 425.348 s | −184.096 s (−30.207 %) |
| `RenderMS` | 605693 | 421895 | −183798 |
| Voice active union | 72.685 s | 27.674 s (analysis + prepare combined union) | much shorter |
| Items union | 105.637 s | 133.046 s | slower |
| Assembly union | 140.573 s | 4.091 s | much shorter |
| Delivery + quality union | 75.698 s | 45.676 s incl. probe (decode 45.596 s) | shorter |
| Final mux + final AAC certification | — | 3.435 s + 11.692 s | added by the audio-only path |

Items got slower because the overlay composition moved into the item encodes,
while whole video preparation is much faster (the assembly stage collapses from
140.573 s to 4.091 s). Audio-only candidates cut the repeated full-video traffic,
but a final mux and an extra final AAC measurement were added, so no audio
wall-time improvement is claimed.

Paired equivalence (root): the approved plan is equal; every historical loudness
field and every track level is equal; final certification is unchanged and
accepted the candidate after 3 native and 2 Media Foundation attempts. The
program PCM digest and the final decoded-AAC evidence digest are identical
between baseline and candidate. Full delivery stays 55836 frames, 930.6 s, 48 kHz
stereo. The delivered MP4 file digest differs because the video encode topology
changed, and the file grew from 4,440,046,488 to 4,501,498,794 bytes (+1.384 %).

Root visual check: six paired frames at 0.2, 4.8, 24.6, 343.5, 922.8 and 929 s
looked composition-equivalent. This is a limited sample, not encoded pixel
identity; the synthetic PSNR above is separate evidence. Both renders share the
same freeze span 343.366667–344.383333 s and the same single warning. The new
local replay fixture is authorized under the current neon-violet policy, and its
hashes are deliberately distinct from the historical approval; the saved
captures are unchanged.

### NVENC resource sweep (root, 2026-09-16)

Local scratch harness over the saved real item commands, NVENC only, bundled
FFmpeg 8.1.2, 16 logical CPUs, RTX 5080. Every cell succeeded with matching
frames, samples, PCM and video against its own auto/serial baseline.

Short batch: 6 items, 8 s excerpt each (48 media-seconds per cell), 3 repeats;
median wall seconds:

| Thread profile | c1 | c2 | c3 |
| --- | ---: | ---: | ---: |
| auto | 13.718 | 9.340 | 9.167 |
| t2 | 20.224 | 11.367 | 9.421 |
| t4 | 15.445 | 9.510 | 8.443 |

Long batch: one 30 s source excerpt rendered as 3 copies per cell (90
media-seconds), concurrency 3, 2 repeats; median wall seconds:

| Thread profile | median | repeats |
| --- | ---: | --- |
| auto | 13.793 | 13.601, 13.986 |
| t4 | 13.480 | 13.956, 13.003 |

Decision: **retain the current 3-worker automatic-threads defaults; no
`threads:4` cap is implemented.** A 4-thread cap shows a modest win only in the
short batch at concurrency 3 (8.443 s vs 9.167 s, ~7.9 %) and hurts the serial
cell (15.445 s vs 13.718 s, ~12.6 % slower). In the long batch the difference is
~2.3 % and the repeat direction crosses (auto 13.601/13.986, t4 13.956/13.003),
so there is no universal stable win.

Measurement limits, not attribution:

- Each thread profile bundles decoder, filter and output threads. The saved item
  manifest does not carry the editor's existing `Threads` option, so these cells
  cannot attribute a result to any single stage or counter.
- `process_cpu_seconds_sum` is the sum of child process User+System CPU time
  (real CPU time); dividing by wall seconds can exceed 1 and is not a
  utilization percentage.
- GPU counters are `nvidia-smi` point samples around each cell (for example
  `gpu_util_max_percent` 3–11 %), so they cannot infer peak utilization or the
  bottleneck.
- RAM is sampled once at the entire sweep's start and end only; per-cell RAM is
  not sampled.
- Per-process IO and temporary bytes are not measured, so no IO or
  disk-throughput claim follows.
- The harness writes scratch NUT item outputs only: no publish, no audio master,
  and concurrency is a worker pool over item processes, not a full render.

### Software resource sweep (root, 2026-09-16)

Six real 8-second item excerpts, two interleaved repeats with automatic threads:

| Concurrent item processes | Repeat 1 | Repeat 2 | Median |
| --- | ---: | ---: | ---: |
| 1 | 59.535 s | 62.033 s | 60.784 s |
| 2 | 53.349 s | 54.557 s | 53.953 s |
| 3 | 53.685 s | 56.087 s | 54.886 s |

All 36 outputs match their own auto/serial reference: 480 frames, 384000 audio
samples, full float PCM digest and decoded video digest. Two processes beat
three by only 1.7% in this bounded sample; this does not justify changing the
current production limit across content and hardware.

The initial auto/t2 matrix was stopped after the first t2 batch took 260.808s
and failed content equivalence for all six outputs. That run is partial log
evidence, not a completed matrix. A subsequent single-entry diagnosis of
`round-44708-software` completed successfully in 49.051s with t2: frame count,
sample count and float PCM matched, but the decoded video digest differed.
The profile is therefore rejected for this equivalence requirement; a different
digest alone does not establish corruption or worse perceptual quality.

Raw evidence in `.local/render-analysis/`:

- `timing-benchmark-software-auto.json`: completed automatic-thread sweep.
- `coordinator-software-sweep.log`: partial rejected t2 matrix.
- `timing-benchmark-software-rejected-t2.json`: completed single-entry diagnosis.
- `timing-benchmark-nvenc.json` and `timing-benchmark-nvenc-long.json`: completed
  NVENC trials.
- `coordinator-paired-summary.json`: complete-render comparison and equality
  checks.

The resource sweeps share manifest digest
`1f14bdba70dd8f1b559cd4f15303ad7f1a1e8873333f6038a10c9ee1c43133e8`.
The sampling limits above apply to software too. The full-render improvement
is one paired local result, not a speed guarantee for every job or computer.
No C++ rewrite is justified by these measurements. In the candidate, FFmpeg
active intervals cover 416.682s of the 421.895s render timer; this is elapsed
time evidence, not a Go CPU profile.

## Concurrent video and audio pipelines (2026-09-17)

Item NUTs muxed video and audio only by construction; the two graphs never
exchange frames. Each timeline item is now rendered by a video-only and an
audio-only FFmpeg process. Item audio is joined into its own lossless
`full-demo-program-audio.nut`, and voice preparation, audio items and the whole
mastering loop run alongside the item video encodes. The passing AAC candidate
is still muxed once with the committed program video, measured again and
published atomically. Loudness policy, candidate sequence, acceptance rules,
encoder settings and evidence are unchanged.

Replay of the same saved job and fixture as the paired render above, against the
#192 candidate (`candidate-final`):

| Metric | #192 candidate | Concurrent pipelines | Delta |
| --- | ---: | ---: | ---: |
| Post-recording wall | 425.348 s | 375.307 s | −50.042 s (−11.8 %) |
| `RenderMS` | 421895 | 371656 | −50239 |
| Items union | 133.046 s | 158.844 s | slower (shares the CPU with audio) |
| Voice analysis union | 26.834 s | 58.252 s | slower (shares the CPU with items) |
| Audio input analysis + candidates | 191.043 s | 235.381 s | slower per pass, but overlapped |

Equivalence: the delivered MP4 is **byte-identical** (SHA-256
`2b69b26322d2a9f56f7adf7ee1c68bd527a636b4aee391312404b1e8ca258fb8`,
4,501,498,794 bytes), the program PCM digest is identical
(`68baf2bace35ff3a660e5a5bbd61fbed5e913032dfd73a5652576e38942d407a`), and
`program_loudness`, `track_levels`, `delivery` and `transitions` evidence are
equal, including the 3 native + 2 Media Foundation attempt sequence.

The gain is smaller than the sum of the removed serial time because every
overlapped process is slower under contention: the audio branch is now the
critical path (voices → audio items → mastering ≈ 314 s) and the video branch
finishes well before it. Shortening the audio branch is the next lever.

### Speculative AAC recovery (2026-09-17)

The Media Foundation recovery chain depends only on the first program
measurement, never on the native masters. Once the first native master has been
rejected, the chain now runs alongside the remaining native masters. Native
masters keep precedence: the recovery result is consumed only after every native
master failed, its attempts are folded into the evidence in the approved order,
and it is cancelled and its candidate and logs removed as soon as a native
master passes. A render whose first native master passes never starts it.

Same replay, same fixture:

| Metric | #192 candidate | Concurrent pipelines | + speculative recovery |
| --- | ---: | ---: | ---: |
| Post-recording wall | 425.348 s | 375.307 s | 319.797 s (−24.8 % vs #192) |

The delivered MP4 is again byte-identical (SHA-256 `2b69b263…58fb8`), all
loudness, track-level, delivery and transition evidence is equal, the attempt
sequence is still 3 native + 2 Media Foundation, the log file set is identical
and every timing span is `ok`.

## Audio critical path (2026-09-18)

After #194 and #195 the audio branch (voices -> audio items -> program-audio
assembly -> mastering) is the Full Demo critical path: in the saved replay the
video branch finishes well before it. Three changes shorten that branch. Output
media, loudness policy, candidate sequence, acceptance rules and encoder
settings are unchanged.

### Measurement harness and its limits

`TestFullDemoAudioBranchSweep` is an opt-in local harness (`FULL_DEMO_AUDIO_BENCH_*`)
over the saved, verified replay inputs at
`.local/render-analysis/phase-a-out/baseline-validation`: 17 timeline items,
five team-voice tracks, a 930.6 s program, Studio's bundled FFmpeg
`n8.1.2-30-g45f1910444-20260723`, 16 logical CPUs, RTX 5080. It runs only
audio-branch stages, through production code (`prepareFullDemoVoiceTracks`,
`runFullDemoItemPool`, the assembly commands), reads the saved job and the saved
replay outputs read-only, and writes only into its own scratch work directory.
With `FULL_DEMO_AUDIO_BENCH_VIDEO=1` the production video item pool runs as a
concurrent load, which is the contention the audio branch has in a real render.

Limits: one machine, one job, every repeat reported. The load is a sustained
item-encode pool, not a complete render, so these numbers rank configurations of
one stage. They do not predict total render wall time, and no paired full render
was run for this change. Stage numbers are interval unions, never additive with
`RenderMS`.

### 1. The assembly measures the program audio it writes

`prepareFullDemoProgramAudio` concatenated the item PCM into
`full-demo-program-audio.nut`, and mastering then decoded that whole file again
with `measureLoudness` for the first program measurement. The assembly command
now carries a second, discarded `-f null -` output with exactly the filter
`measureLoudness` uses (`loudnorm=...:print_format=json`), so one process
decodes the items once and produces both the committed PCM and the measurement.
The measurement output is declared first, so the committed program audio stays
the command's last argument and the atomic commit is unchanged. Mastering reuses
the parsed measurement only when the assembly measured the same target, writes
the same `program-input-loudness.txt` evidence from the assembly's own FFmpeg
output, and otherwise runs its original measurement pass unchanged, including
its `audio_input_analysis` timing span.

Quiet machine, 930.6 s program, three repeats (seconds):

| Repeat | Assembly | First measurement | Separate total | Fused | Saved |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 | 0.495 | 14.395 | 14.890 | 15.460 | −0.570 |
| 2 | 0.522 | 15.600 | 16.122 | 15.176 | 0.947 |
| 3 | 1.242 | 13.694 | 14.935 | 13.163 | 1.772 |

Two repeats under the concurrent video load saved −0.807 s and 4.879 s, with the
same equivalence results.

The saving is about the size of the assembly pass, not of the measurement:
loudnorm's dynamic mode upsamples to 192 kHz and dominates the cost, and that
work still has to happen once. What the change removes from the critical path is
the second process and its full re-read of the 357 MB program file (median
0.947 s here, −0.570 to 1.772 s across repeats, so a single repeat cannot
distinguish it from noise). It is kept because it is equivalence-verified and
strictly removes work, not because it is a large win.

Equivalence and evidence:

- The committed program audio is byte-identical to the unfused assembly's, on
  both the synthetic canary and the 930.6 s replay program
  (`TestFullDemoProgramAudioAssemblyMeasuresWhatItWrites`, plus every harness
  assembly repeat).
- The fused measurement equals a standalone `measureLoudness` over the file the
  same command wrote, field for field, so the silence gate, the evidence JSON
  and the retarget chain see exactly the values they saw before.
- Deliberate, documented changes: the assembly runs at `-v info` instead of
  `-v error`, because loudnorm prints its JSON block only at info level (no
  encoder option changes); `program-audio.log`, which is written only when the
  assembly fails, keeps its shape (label, exit status, FFmpeg's trimmed output)
  but that output, like the failure message, is now info-level rather than
  error lines alone; `program-input-loudness.txt` keeps its path and its
  loudnorm JSON block, but its surrounding FFmpeg lines are now the assembly
  process's, with one extra output section; and the attempt -1
  `audio_input_analysis` timing span is no longer recorded, because that work
  happens inside `audio_assembly`: a render whose first native master passes
  records no `audio_input_analysis` span at all, and one that retargets records
  one span per retarget (at most two instead of three). The log file set is
  unchanged.

### 2. Audio-only item budget

`runFullDemoItemPool` sized the audio-only pool with `fullDemoItemJobs()`, the
item *encoder* budget. Audio items run no encoder: each one demuxes its capture
segment plus the prepared voice WAVs and writes lossless PCM through a filter
graph. The pool now takes its budget per stream kind (`fullDemoItemPoolJobs`),
and audio-only items use `fullDemoAudioItemJobs()`, bounded by
`fullDemoAudioItemJobsMax` = 4 and by the CPU count. Timeline order is
unaffected: the pool still commits `item-%03d-audio.nut` per index and the
concat list is written from that ordered slice.

`items_audio` interval union, 17 items, under the concurrent video load, four
repeats (seconds):

| Workers | Repeats | Median | Process-elapsed sum |
| --- | --- | ---: | ---: |
| 3 (previous) | 5.711, 3.947, 4.223, 4.760 | 4.492 | 10.2–15.7 |
| 4 (current) | 3.203, 3.212, 3.126, 3.045 | 3.169 | 11.1–11.4 |
| 6 | 2.876, 2.399, 2.369, 2.166 | 2.384 | 10.8–12.4 |

Every 4-worker repeat is faster than every 3-worker repeat, so the ~1.3 s is a
real reduction rather than noise; an earlier two-repeat pass, where the two cells
overlapped, was not conclusive and was rerun. Six workers save a further ~0.8 s
while running six processes against the concurrent video encodes; 4 is kept as
the point where the stage is already tight and predictable. The absolute numbers
are small on purpose: this stage is a few seconds of a multi-minute render, so
the change is worth ~1 s of critical path, nothing more.

### 3. Voice pool budget

`fullDemoVoiceJobs`'s comment said voice preparation "must not compete with the
three item encoders". That was written when voice preparation ran before item
encoding; since #194 the audio branch runs alongside the video branch, so the
voice pool overlaps the item encoders whatever its size, and it is the head of
the audio critical path. `fullDemoVoiceJobsMax` is now 5: the five tracks of one
CS2 team are the ceiling the team-voice policy can select, so a wider pool would
never be used, and the CPU bound still applies.

Five real team-voice tracks, under the concurrent video load, two repeats
(seconds):

| Workers | Stage elapsed | `voice_analysis` union | `voice_prepare` union |
| --- | ---: | ---: | ---: |
| 3 (previous) | 105.759, 132.785 | 97.002, 104.764 | 33.750, 46.560 |
| 4 | 138.327, 140.830 | 102.411, 109.368 | 53.962, 48.424 |
| 5 (current) | 73.440, 72.848 | 69.897, 67.835 | 12.314, 11.950 |

Five workers remove the straggler: with three or four workers the last track (or
last two) starts only after an earlier one finishes and then runs alone, which
also explains why four was not better than three here. The measured stage is
~46–67 s shorter than with three workers, on the head of the audio critical
path. Process-elapsed sums rise (voice analysis 243–245 s at three workers
versus 310–317 s at five), which is the expected cost of running five decoders
against the video encodes; the video branch is not the critical path in this
fixture, and no video-branch regression was measured here.

### Paired full render (2026-09-18)

Same replay harness and `--reapprove` fixture as the 2026-09-16 pair, run back
to back on the same machine (16 CPUs, RTX 5080, bundled FFmpeg n8.1.2).
Baseline is `fcc4e1f` (`main` after #195); candidate is this change. Both runs
went through the full production path: item encodes, transitions, concurrent
branches, five mastering candidates (3 native + 2 Media Foundation), strict
delivery decode and quality pass.

| Metric | Baseline | Candidate | Delta |
| --- | ---: | ---: | ---: |
| Post-recording wall | 401.713 s | 392.787 s | −8.926 s (−2.222 %) |
| `RenderMS` | 397599 | 388621 | −8978 |
| Voice union (analysis + prepare envelope) | 3.7–87.7 s | 3.7–58.7 s | −29.0 s |
| Items (video) union | 226.3 s | 234.7 s | +8.4 s |
| Items (audio) union | 9.4 s | 7.3 s | −2.0 s |
| Audio assembly (+ fused measurement) | 1.8 s + 33.7 s | 35.5 s | ≈0 |
| Mastering envelope end | 340.8 s | 332.5 s | −8.3 s |
| Delivery + quality | 56.8 s | 56.1 s | −0.7 s |

Equivalence: the captured lossless program PCM has the same SHA-256 in both
runs (`78e9d9f515adfb4e…`), every track level, the program input measurement,
all five decoded-AAC measurements, the master targets, the fallback masters and
the final muxed AAC measurement are equal, the delivered MP4 has the same byte
size, and neither run emitted a warning.

Reading: the voice pool wins its ~29 s at the head of the audio branch as the
sweep predicted, and the audio branch is still the critical path (video items
end at 238 s, mastering at 332 s). Most of that win is then absorbed inside
mastering: the first native candidate encode and the retarget measurements ran
about 20 s slower than in the baseline, and the video items ran 8 s slower.
The likely mechanism is that mastering now starts ~36 s earlier and therefore
overlaps more of the three concurrent item encodes, so both sides contend for
longer. That is a hypothesis from one pair, not a measurement; the net wall
delta is what this section claims. The tuned budgets are kept because the
stage-level evidence is consistent and the output is equivalent, and because
the remaining mastering cost (five full candidate passes, ~230 s) is now the
obvious next target rather than the pools feeding it.

## Mastering chain: contention and speculative delivery (2026-09-18)

The paired render above left the mastering chain as the whole critical path:
102.64 s -> 332.50 s, 229.86 s of a 392.79 s render, and 100 % serial (every
span starts within 10 ms of the previous one ending). Two changes attack the
part of that envelope that is serial by accident rather than by necessity.

### The measured floor of one loudnorm pass

The uncontended cost of a single loudnorm pass over this exact 930.6 s program
was measured directly on the saved output (eight runs, two alternating repeats
of four shapes, loudnorm JSON byte-identical in all eight):

| Shape | Run 1 | Run 2 |
| --- | ---: | ---: |
| Production shape on the 4.5 GB MP4 | 12.379 s | 12.379 s |
| Production shape on a 22 MB audio-only `-c copy` of it | 12.380 s | 12.368 s |
| Cheap null sink (`pcm_u8`, mono) | 12.395 s | 12.477 s |
| `-threads 1` | 12.340 s | 12.361 s |

A loudnorm pass is 12.37 s +/- 0.04 s, single-threaded, and insensitive to input
size, to the null-sink format and to thread options. Reading the 4.5 GB video
costs nothing measurable; an audio-only `-c copy` extraction of it takes 3.53 s,
so the demux is ~1.3 GB/s and is fully hidden behind loudnorm.

Against that floor the chain's serial-by-necessity floor is ~171 s (fused
assembly measurement 14.2 + three times [encode 26.06 + analysis 12.37] +
two retarget measurements of 12.37 + mux 4.0 + final certification 12.37). The
audio tail took 265.6 s, and the whole 94.9 s difference sits inside
66.9-251.3 s, the window where the three-wide video item pool and the
speculative Media Foundation recovery encodes run:

| Pass | Contended | Uncontended reference (same render) | Tax |
| --- | ---: | ---: | ---: |
| `audio_assembly` | 35.49 s | 14.2 s floor | +21.3 s |
| `audio_candidate_encode` a=0 native | 55.83 s | 26.06 s (a=2) | +29.8 s |
| `audio_candidate_analysis` a=0 | 29.26 s | 12.30 s (a=2) | +16.9 s |
| `audio_input_analysis` a=1 | 34.09 s | 12.76 s (a=2) | +21.7 s |
| `audio_candidate_encode` a=1 native | 29.43 s | 26.06 s (a=2) | +3.4 s |

FFmpeg's own speed figures for the identical single-threaded pass agree:
26.3x / 27.4x / 31.9x inside the window, 73.1x-75.9x outside it. Everything
after 251 s is already at the floor.

### 1. The video item pool and the speculative recovery yield the CPU

`runDiagnosticFFmpeg` is the single place where every FFmpeg subprocess of the
editor is started, so the wanted scheduling class is carried on the context the
way the timing stage already is and applied there, immediately before
`cmd.Run`. The video-only item pool (`fullDemoItemPoolBackground`) and the
whole speculative AAC recovery goroutine now start their processes in
`BELOW_NORMAL_PRIORITY_CLASS`; the voice pool, the audio-only item pool, the
program-audio assembly, the native mastering chain, the final mux and the final
certification stay at normal priority. It is a Windows-only lever
(`command_priority_windows.go` / `command_priority_other.go`); elsewhere the
mark is inert.

The justification is the slack, not the priority: the video branch finished at
242.60 s while mastering ended at 332.50 s, and the final mux only needs the
committed program video by ~315 s. Scheduling priority is work-conserving, so
the deprioritised work still gets the whole machine whenever the audio branch
is idle, and it cannot reach any encoder's output: h264_nvenc, native aac,
aac_mf and pcm are deterministic functions of input plus options, and no option
changes.

The ceiling is the ~95 s of contention tax, but the realistic gain is smaller:
once mastering ends before ~260 s the video branch becomes the next gate, and
the deprioritised item encodes will themselves end later. On an idle machine
whose pools already fit the core count the gain is zero, not negative.

### 2. An accepted recovery candidate is muxed and certified speculatively

On this replay the recovery chain is never the blocker; it is the winner that
waits. Its second Media Foundation candidate was accepted at 279.86 s, but the
final mux only started at 315.17 s because native attempt 2 (retarget
measurement 264.05-276.81, encode 276.81-302.87, analysis 302.87-315.17) still
had to be evaluated for native precedence. Recovery then sat idle for 35.31 s,
and the mux plus the final certification (315.17-332.50, 17.33 s) ran afterwards
in an otherwise empty machine. The baseline run has the identical shape:
accepted at 287.46 s, native chain done at 324.30 s, 36.84 s of slack, then
3.95 s + 12.54 s. In both runs `fallback_masters` has two entries and
`final_muxed_aac` equals `decoded_aac[4]`, so the recovery candidate is what was
delivered.

Native precedence is unchanged. What moved is only the work that a passing
native attempt would make moot: the speculation now muxes its accepted
candidate with the committed program video and measures the actual muxed AAC as
soon as it has one, into its own `filecommit` attempt, publishing nothing and
recording no evidence. `finishFullDemoAACRecovery` still folds
`MasterTargets` / `FallbackMasters` / `DecodedAAC` in the approved order and
still owns the acceptance check, the `ctx.Err()` recheck and the atomic
publication. A native master that passes discards the speculative attempt and
its logs exactly as it already discarded the candidate, and the native delivery
then writes the same logs itself. The delivery was split into
`prepareFullDemoAACDelivery` (mux + final measurement, no side effect on
`output`) and `commitFullDemoAACDelivery` (acceptance + publication +
evidence); `deliverFullDemoAACCandidate` is their composition and is unchanged
in behaviour.

Cost: a second ~4.5 GB attempt file can exist beside `output` while the native
chain finishes, and it is deleted whenever a late native attempt passes. The
change buys nothing on renders where a native master passes, which is the
common case on quiet material; it only pays on the slow path this fixture
exercises.

### Timing evidence

No stage is removed or fused, and no span is renamed. One span pair moves: when
the delivered candidate comes from the recovery chain, `final_mux` and
`final_audio_analysis` now start right after the accepting
`audio_candidate_analysis` of the recovery variant instead of after the last
native `audio_candidate_analysis`, so in the sorted span list they appear
before the native attempt 2 spans rather than after them. Both keep their
stage, attempt (-1) and label. That reordering is the only intended change to
the timing evidence of this replay; every other span keeps its identity and
only its offsets move.

On the other path, where a late native master passes and the speculation is
discarded, the discarded delivery has already recorded its own `final_mux` and
`final_audio_analysis` spans, with outcome `ok` if it finished or `cancelled`
if `discard` killed it mid-mux, and the native delivery then records the same
pair again. Two spans of each stage in one render therefore mean a speculative
delivery that was thrown away, and the delivered one is the later pair. A
render whose first native master passes never starts the speculation at all and
is unaffected.

The numbers above are the pr196 pair's, measured before the two changes; the
paired render that validates them is recorded in the next subsection.

### Paired full render (mastering chain, 2026-09-18)

Same saved replay as the earlier pairs (930.6 s program, 17 items, 5 voice
tracks), same harness (`phase-a-replay.py --reapprove`), same bundled FFmpeg,
run sequentially on one machine: `mastering-baseline-1` with a binary built
from this branch's HEAD before the two mastering changes, then
`mastering-candidate-1` with a binary rebuilt from the working tree carrying
both of them in their first form (BELOW_NORMAL for the video item pool AND
for the speculative AAC recovery; speculative mux and certification of an
accepted recovery candidate). The recovery mark was removed after this pair;
the second pair below measures the shipped form. Every figure below was re-derived from the two
`phase-a-replay-summary.json` files by the reviewer, not copied from the
render report.

| metric | baseline | candidate | delta |
| --- | --- | --- | --- |
| harness `elapsed_s` | 401.5 s | 356.0 s | **-45.5 s** |
| `render_ms` | 397 116 | 351 815 | **-45 301 (-11.4 %)** |
| `render_seconds_per_media_second` | 0.4267 | 0.3781 | -0.0486 |
| `output_bytes` | 4 501 498 794 | 4 501 498 794 | 0 |

Branch envelopes (union of intervals, not sums, for the per-stage figures):

| branch | baseline | candidate | delta |
| --- | --- | --- | --- |
| audio upstream (voice analysis + voice prepare + audio-only items + assembly) | 3.7 s..98.9 s = 95.2 s | 3.7 s..50.3 s = 46.6 s | **-48.6 s** |
| mastering (first candidate encode .. end of final analysis) | 99.1 s..339.1 s = 240.0 s | 50.4 s..293.8 s = 243.4 s | +3.4 s |
| — native chain | 99.1 s..322.5 s = 223.4 s | 50.4 s..261.4 s = 211.0 s | -12.4 s |
| — Media Foundation recovery chain | 192.1 s..287.7 s = 95.6 s | 103.1 s..277.3 s = 174.2 s | **+78.6 s** |
| video branch (transitions + items + assembly) | 0..244.5 s | 0..267.0 s | +22.4 s |
| delivery | 339.1 s..397.1 s = 58.0 s | 293.8 s..351.8 s = 58.0 s | 0 |

Per-stage unions that moved: `voice_analysis` 47.3 s -> 24.4 s, `voice_prepare`
18.3 s -> 5.4 s, `audio_assembly` 32.4 s -> 18.5 s, `items_audio` 4.8 s -> 1.7 s,
`audio_input_analysis` 49.6 s -> 38.2 s; against that, video `items`
237.5 s -> 260.2 s and `audio_candidate_analysis` 84.3 s -> 102.4 s.
`final_mux` (4.11 s -> 4.06 s), `final_audio_analysis` (12.49 s -> 12.49 s),
`delivery` (58.0 s -> 58.0 s), `transitions` (3.7 s -> 3.7 s) and `assembly`
(3.3 s -> 3.0 s) are unchanged within noise.

**This replay is a recovery-wins render.** All three native masters are rejected
(decoded -15.43/0.51, -16.34/-0.5, -19.35/6.96 LUFS/dBTP against targets
-14/-1.8, -13/-4.01, -12/-5.21) and the second `aac_mf` recovery candidate
(-14.44 LUFS / -3.35 dBTP) is the accepted master in both runs. So the
speculative delivery is exercised for real: in the baseline `final_mux` cannot
start until native attempt 3 has been analysed at 322.5 s, 34.8 s after the
recovery was accepted at 287.7 s; in the candidate `final_mux` starts at
277.267 s, 1 ms after the accepting recovery analysis ends at 277.266 s. That
34.8 s of dead wait is change 2 working exactly as designed.

**Where the 45.3 s of `render_ms` comes from.** The final mux starts 45.2 s
earlier (322.5 s -> 277.3 s) and everything after it (mux, certification,
delivery) is unchanged. Of those 45.2 s, 34.8 s is the dead wait above and
10.4 s is how much earlier the recovery candidate is accepted
(287.7 s -> 277.3 s). The recovery chain starts 89.0 s earlier
(192.1 s -> 103.1 s: 48.7 s from the upstream gain of change 1 plus 40.3 s
from a faster native attempt 0, whose encode + analysis drop from 92.9 s to
52.6 s once the item pool yields), but it runs 78.6 s longer under
BELOW_NORMAL (recovery encode 0 50.9 s -> 68.6 s, its decoded-AAC analysis
13.1 s -> 41.0 s, recovery encode 1 18.7 s -> 51.9 s), so it is accepted only
10.4 s earlier.

**And that is where the two changes fight each other.** The recovery is the
critical path on this material, and change 1 marks it BELOW_NORMAL: roughly
38 s of the 48.6 s that change 1 wins upstream is handed back by change 1's
own mark on the recovery. The clear next iteration, measurable with this same
harness: keep BELOW_NORMAL on the video item pool but not unconditionally on
the recovery, for example restore NORMAL once a native attempt has been
rejected, or once the recovery candidate is the only live candidate. That
iteration was run as `mastering-candidate-2b` (next subsection): the recovery
simply keeps NORMAL, which is the shipped form.

**The video branch is now close to being the gate.** Deprioritised, it ends at
267.0 s (was 244.5 s) while the speculative mux needs the committed program
video at 277.3 s: 10.3 s of slack, down from 78.0 s in the baseline. On a
render whose native attempt 0 passes, the audio branch goes idle after
~100 s and the item pool gets the whole machine back, so the transferred
contention is bounded by that first window; but on such a render the video
branch is the critical path and change 1 can only cost time there, never win
it. This pair does not measure that path; the saved jobs of this machine all
needed the recovery chain, which is why the trade is taken.

**Equivalence.** Byte-identical on every axis checked:

- program PCM `nut/*.nut`, 4 479 493 455 bytes, sha256
  `78e9d9f515adfb4e67cb591c99480302d062a91ea82775407940cfbfef8a31e8` in both,
  the same hash as the PR #196 pair;
- delivered `short-001-demo-compilation.mp4`, 4 501 498 794 bytes in both;
- `full-demo-loudness.json`, `full-demo-audio.json`, `full-demo-delivery.json`,
  `full-demo-hud.json`, `full-demo-approved.json` and
  `full-demo-effective.json` byte-identical (`cmp` clean): `input`, the five
  `decoded_aac` entries in order, the five `master_targets` in order, the two
  `fallback_masters` (`aac_mf` gain 3 and 3.8599999999999994, ceiling -5) in
  order, `status` `verified-decoded-aac` and `final_muxed_aac` equal to
  `decoded_aac[4]`;
- a recursive field-by-field diff of the whole `shorts-result.json` finds zero
  differences outside absolute paths carrying the label and the `performance`
  timing block; `shorts[0].full_demo`, the five `track_levels` included, diffs
  to nothing;
- `warnings` identical (`quality demo-compilation detected frozen frames`,
  pre-existing and also present in the PR #196 pair);
- the same 10 files under `out/logs/`, with all 9 `loudnorm` JSON blocks
  character-identical; those files differ only in the label-bearing attempt
  path, the FFmpeg filter-instance pointer addresses and the throughput footer
  (e.g. `speed=28.3x elapsed=0:00:32.84` vs `speed=50.5x elapsed=0:00:18.41`);
- 79 spans in both, with the same (stage, attempt, variant, encoder, label,
  outcome) multiset, every outcome `ok`.

`TestFullDemoMasterSequenceMatchesSavedReplay` now pins the retarget and
recovery sequence to these exact values without FFmpeg: fed the five decoded
measurements above, the pure helpers must reproduce `master_targets` and
`fallback_masters` in order and accept only the second recovery candidate.

**Limits.** One machine, one pair, no repetitions: the wall and per-stage deltas
are single samples on a contended Windows desktop, and the scheduling-sensitive
stages (`voice_analysis`, the recovery chain) are precisely the ones a second
sample would move most. Because the recovery wins here, no speculative delivery
is ever discarded, so the discard branch of the timing evidence has unit-test
evidence only; a native-accept replay would be needed to confirm the discarded
`final_mux`/`final_audio_analysis` ordering on a real render and to measure
what change 1 costs the video branch on that path. The byte-level equivalence
above is exact and does not share these limits.

### Paired full render, shipped form (mastering chain, 2026-09-19)

Same harness, replay, FFmpeg and machine as the pair above, run with nothing
else on the machine: `mastering-candidate-2b` is the working tree with the
recovery mark removed (the video item pool is still BELOW_NORMAL; the
speculative recovery, its mux and its certification keep NORMAL). It is
compared against the same `mastering-baseline-1` run and against
`mastering-candidate-1` (both marks). A first attempt at this render was
discarded because a test suite compiled beside it during its first minute.

| metric | baseline-1 | candidate-1 (both marks) | candidate-2b (shipped) |
| --- | ---: | ---: | ---: |
| harness `elapsed_s` | 401.5 s | 356.0 s | **339.6 s** |
| `render_ms` | 397 116 | 351 815 | **335 734 (−15.5 %)** |
| audio upstream envelope | 3.7..98.9 s | 3.7..50.3 s | 3.6..54.4 s |
| Media Foundation recovery chain | 192.1..287.7 s = 95.6 s | 103.1..277.3 s = 174.2 s | 106.8..209.9 s = **103.1 s** |
| video branch end | 244.5 s | 267.0 s | 262.2 s |
| `final_mux` start | 322.5 s | 277.3 s | 262.5 s |
| delivery | 339.1..397.1 s | 293.8..351.8 s | 279.0..335.7 s |
| `output_bytes` | 4 501 498 794 | 4 501 498 794 | 4 501 498 794 |

Per-stage unions, candidate-1 -> candidate-2b: `audio_candidate_analysis`
102.4 s -> 92.9 s, `audio_candidate_encode` 170.6 s -> 160.8 s, video
`items` 260.2 s -> 255.8 s, `voice_analysis` 24.4 s -> 29.5 s,
`voice_prepare` 5.4 s -> 7.8 s (the recovery at NORMAL now shares the CPU with
the upstream stages, which is the expected small give-back).

Equivalence: program PCM sha256 `78e9d9f515adfb4e…` in all three runs; the
canonical JSON of `shorts[0].full_demo` (track levels, program loudness
input, all five decoded-AAC measurements, master targets, fallback masters,
final muxed AAC, status) hashes identically in all three; delivered MP4 byte
size identical; the same single pre-existing frozen-frames warning in all
three.

Reading: with the recovery back at NORMAL its chain returns to the baseline's
length while still starting 85 s earlier, so the accepted master arrives at
209.9 s instead of 287.7 s and the speculative mux fires as soon as the
committed program video exists. **The video branch is now the gate**: it ends
at 262.2 s and the final mux starts at 262.5 s, 0.3 s later. Any further
audio-side saving on this material is invisible until the video branch
(17 NVENC item encodes, deprioritised, 255.8 s union) gets shorter, and the
BELOW_NORMAL mark on that pool is now costing exactly the slack it created.
Next target: the video item pool, either by giving it the CPU back once the
audio upstream has finished (the mark only needs to hold for the first ~55 s)
or by shortening the item encodes themselves.

## Frozen pre-implementation audit

Everything below records the source baseline before the implementation above.
References to the current renderer, proposed changes and validation still to
be performed in this historical section describe that earlier state.

### Evidence and limits

Two successful local Full Demo artifacts from September 14 were inspected, including their saved FFmpeg commands, render results, loudness logs and worker spans. These are historical measurements, not fresh end-to-end benchmarks of the current source revision. Both jobs use the same demo/player and approximately the same edit; they are not a representative hardware/content corpus.

| Saved job | Recording worker | Rendering worker | Internal render timer | Delivered duration | Quality-check timer |
| --- | ---: | ---: | ---: | ---: | ---: |
| `29b58ffc…` | 392.005 s | 581.707 s | 572.219 s | 930.583 s | 74.273 s |
| `2dfe87ce…` | 385.831 s | 621.859 s | 610.326 s | 930.600 s | 83.653 s |

The quality-check timer overlaps delivery verification and is already included in the internal render timer. Do not add these columns together. The worker overhead beyond the internal render timer is only 9.5–11.5 seconds in these examples. The manifests' generic `video_preset: slow` is misleading if read alone: the actual saved commands explicitly use `h264_nvenc`, `p5`, `vbr`, `cq=16`. Hardware video encoding is already active.

Both outputs are about 4.44 GB, contain 17 timeline items and five selected team-voice tracks, and use a custom HUD plus transitions (`29b58ffc…` used Glacier, `2dfe87ce…` used Carbon). Intro is 5 seconds and outro is 8 seconds. Both require three native AAC attempts and two Media Foundation AAC recovery attempts before meeting the approved loudness contract.

Fresh isolated experiments use the saved media read-only and Studio's bundled FFmpeg `n8.1.2-30-g45f1910444-20260723`, on a Ryzen 7 9800X3D / RTX 5080 / 32 GB Windows machine. The reproducible scratch harness and raw results are in `.local/render-analysis/`; they are intentionally not production code. No CS2/HLAE run is needed for these experiments.

### Fresh measurements

| Isolated work | Current arrangement | Candidate arrangement | Observed improvement |
| --- | ---: | ---: | ---: |
| Verify + black/freeze check, first 60 seconds of real output | Two concurrent processes: 5.091 s | One combined process: 3.500 s | 31.2% less wall time for this work |
| Full-length loudness analysis of five real voice tracks | Sequential: 67.292 s | Two workers: 39.593 s | 41.2% less analysis wall time |
| Same five voice analyses | Sequential: 67.292 s | Three workers: 27.849 s | 58.6% less analysis wall time; 39.44 s saved |

Verification values are medians of three alternating trials per arrangement. Voice values are medians of two trials per worker count. Each verification trial counted exactly 3,600 frames with zero duplicates/drops and completed audio/video decoding. The real excerpt produced no black/freeze events in either arrangement, so this alone is not positive-event coverage. All reported loudnorm JSON fields for all five voice tracks were identical across sequential, two-worker and three-worker runs.

Voice timings cover measurement only, not WAV normalization/materialization. These experiments do not measure a fully integrated candidate renderer, cold-cache disk behavior, low-end hardware or end-to-end speedup. Do not apply the percentages to the whole render.

A separate 24.55-second, 1,473-frame real-round experiment compared the current geometry/filter chain with and without the current generated Carbon ASS HUD (`2dfe87ce…`'s theme; the fresh experiment replayed that job). Three alternating trials produced median NVENC encode times of 3.880 s without HUD and 3.898 s with HUD. Go generated ASS for the first three real rounds in 24, 25 and 51 ms (one generation each). This sample provides no evidence that replacing the HUD generator with C++ would meaningfully improve the render.

For context, reading the same H.264 packets with video stream copy to the null muxer took a median 0.118 s. These video experiments discard output, omit audio/transitions/program overlays and do not include NUT/MP4 writes. They illustrate encode versus packet-copy work, not the measured cost of the complete program assembly stage. Exact frame counts were retained in all nine trials. Raw commands and results: `.local/render-analysis/video-results.json` and `hud.json`.

## What the renderer actually does

1. Materializes and verifies approved assets and captured inputs, builds the editorial timeline, prepares overlay images and HUD data.
2. Processes each selected voice track sequentially: full-demo loudness measurement, then another decode/resample/gain pass to a stereo float PCM WAV. The inspected source tracks span about 26m43s even though the edit is 15m31s.
3. Encodes each approved timeline item into H.264 + float PCM NUT. This includes exact frame/sample trimming, transitions, HUD composition, game/voice mixing, ducking and de-clicks. At most three item processes run concurrently.
4. Concatenates the prepared items into `full-demo-program.nut`. With no effects, video uses stream copy. With intro/outro effects, the complete video is decoded, filtered and encoded again—even outside the 13 seconds where those overlays are visible.
5. Measures full-program loudness. Encodes an AAC candidate while copying the complete video into MP4, then measures the decoded AAC. Retargets and repeats up to three native attempts, with two extra input measurements when retrying. If necessary, uses up to three bounded Media Foundation recovery attempts. Every candidate is written with `+faststart`.
6. Probes format/duration, decodes the complete final audio/video with strict frame counting and error handling, hashes the output, and independently decodes video again for black/freeze quality warnings. The hash and quality work already overlap verification.
7. Publishes the result and evidence. Publication already prefers hard links, with copy fallback, and reuses the probe result.

Implementation map:

- `internal/editor/short_pack.go`: stage ordering, performance counters, quality overlap and publication.
- `internal/editor/full_demo_mix.go`: sequential voice preparation, three-worker item assembly, frame/sample windows.
- `internal/editor/full_demo_concat.go`: effect-dependent second full-video encode.
- `internal/editor/full_demo_hud.go`, `internal/customhud/ass.go`: HUD generation and FFmpeg/libass rendering.
- `internal/editor/full_demo_audio.go`, `full_demo_audio_fallback.go`: native and recovery audio loops.
- `internal/editor/full_demo_delivery.go`: metadata, strict full decode, frame count and digest.
- `internal/editor/ffmpeg.go`: black/freeze checks and encoder thread flag placement.
- `internal/editor/command_diagnostics.go`: existing tool start/finish events with duration.
- `internal/workers/studio_encoder.go`: Studio's separate capture/render encoder selection.

## Ranked opportunities

### 1. Share the final decode between verification and quality diagnostics

Run the existing blackdetect/freezedetect filters in the mandatory final decode process. These filters report diagnostics without changing the picture; audio remains mapped and decoded. Preserve the strict `-xerror`, `-fps_mode passthrough`, canonical frame count, zero duplicate/drop requirements, successful completion, stream-format/duration probe, SHA256 and loudness acceptance.

This removes a redundant video decode while preserving the generated media bytes. It is a strong first implementation candidate because it does not change rendering or audio encoding.

The quality check currently reports warnings, while delivery failure blocks publication. A combined implementation must preserve that distinction, optional quality-check behavior, diagnostic logging and cancellation. If diagnostic setup fails independently, a fallback strict decode can retain existing behavior rather than making an optional check a new hard requirement. Include black/freeze fixtures and corrupt/truncated video and audio cases, not just clean media.

### 2. Prepare independent voice tracks with bounded concurrency

Keep each track's current full-track analysis, normalization policy, gain bound and peak headroom unchanged. Execute independent track pipelines in a small worker pool, store results by original index, and publish evidence in deterministic order. A limit of two or three should be measured rather than assuming more workers are faster.

This retains the current signal processing while shortening a serial section. Budget this pool separately from item encodes so voice preparation does not compete with three simultaneous video renders. Preserve cancellation, first-error handling, atomic outputs and partial-file cleanup.

Do not replace full-demo loudness analysis with analysis of selected clips: that changes the normalization policy. Eliminating the WAV intermediates or moving gain/resampling into each item is a separate change with seek, resampler-state and sample-window risks. The five normalized float-stereo tracks would occupy roughly 3.08 GB at this source duration; reducing that I/O is useful, but needs decoded PCM equivalence tests.

### 3. Remove the second full-video encode by composing overlays in the item render

Project global intro/outro windows into intersecting timeline items and compose them after the existing transitions and HUD. Concatenate the resulting compatible item streams with video copy. Handle windows crossing item boundaries, short edits where windows overlap, sponsor placement, approved trims and the inclusive final overlay frame.

This is the largest structural video opportunity: the current plan does two complete post-capture video encode generations whenever these effects exist. It should not require C++.

However, it is not a byte-identical optimization. Currently the HUD/transitions are encoded and decoded before the intro/outro pass; moving overlays before that first encode changes the codec history. A single encode generally avoids generation loss, but that is not proof of visual equivalence. Require frame-by-frame pre-encode composition checks, review of encoded high-motion gameplay/HUD/text, and quantitative quality comparisons against the source/reference. Retain the same approved CQ/CRF and encoder preset during comparison. Check GOP/extradata compatibility, packet timestamps, exact frame counts, join behavior and audio alignment.

Also check pixel-format negotiation outside active overlay windows: bypassing an overlay graph can bypass implicit color/chroma conversions even when the overlay is disabled. Preserve or explicitly validate those differences; do not assume an inactive filter graph is a mathematical identity.

Do not implement arbitrary `-ss`/`-c copy` cuts around overlays: compressed frames have dependencies and keyframe boundaries are not arbitrary approved frame boundaries. Existing complete prepared items are safer units. Keep an explicit legacy path for unsupported effect combinations until covered.

### 4. Make audio retries independent of the multi-gigabyte video

Retain the exact candidate sequence and decoded-AAC acceptance rules, but render and test audio-only candidates. After one candidate passes, mux it once with the finalized video and perform the complete delivery checks. This avoids copying the full H.264 stream and doing MP4 faststart work for every failed audio attempt.

In both inspected jobs, eight separate loudness measurements consumed about 94 seconds (93.96 s and 94.62 s in FFmpeg's saved elapsed fields), excluding the five audio encoding passes. Three input measurements plus five decoded-candidate measurements mean thirteen full-program audio processing passes including encoding. Each measurement is about 12 seconds for this 930-second program.

The five candidate files also repeatedly contain roughly 4.44 GB of video. This is substantial avoidable file traffic, although no disk-throughput profile has established its exact share of elapsed time. Do not equate the byte count with measured disk writes; buffering, faststart implementation and the original candidate sizes matter.

Separating streams needs tests for native AAC priming, Media Foundation's final padded packet, edit lists, start timestamps, exact playable sample duration and the existing `setts` correction. Compare decoded audio and copied video content, and measure/validate the final muxed artifact—not only the intermediate audio file.

Reducing the number of attempts could save more, but is a different, higher-risk audio-policy change. The inspected native candidates really failed the existing LUFS/true-peak criteria. Do not accept them early, skip decoded-AAC checks, loosen thresholds or bypass the bounded recovery path. The `offset` returned by loudnorm depends on the target, so reusing all first-pass fields across retargets is not automatically equivalent.

### 5. Profile and tune item filtering and resource limits

Item encoding already uses at most three concurrent FFmpeg processes. The existing `Threads` option appears at the output and caps encoder threads; it is not a global budget for decoder and complex-filter threads. FFmpeg's default threading and three processes can compete for the same CPU, GPU and memory bandwidth.

Compare worker counts and explicit filter/decode thread budgets using representative long/short rounds, custom HUDs, transitions and software-encoder fallback. Preserve picture and audio equivalence. Do not raise parallelism blindly, and do not treat the general `--render-jobs` setting as an effective Full Demo item control: `fullDemoItemJobs()` derives its own automatic limit.

## Where C or C++ would help

FFmpeg, its codecs and libass already do the heavy media processing in native code. Rewriting the Go scheduler in C++ leaves the same expensive passes in place. No profiling evidence currently identifies Go allocations, subprocess launch cost or Go execution time as the main bottleneck.

A native GPU compositor could eventually help if measurements identify CPU HUD composition, CPU/GPU transfers or filter throughput as the remaining limit after redundant passes are removed. It would keep decoded frames and suitable filters on the GPU and feed the hardware encoder directly. NVENC encoding alone is not that pipeline: the current filters and ASS HUD run on CPU frames. NVIDIA's documented FFmpeg path already supports hardware decoding/encoding and GPU-resident scaling, so prototype with existing FFmpeg capabilities before introducing a new backend.

If a custom C++ component becomes justified, scope it to a versioned, isolated media worker with the existing CPU path as fallback. It must reproduce font shaping, alpha composition, color conversion/range, rounding, effect ordering and frame/sample clocks. A rewrite of the whole application or an unconditional GPU path would add risk without evidence of proportionate benefit.

References: [FFmpeg filtering documentation](https://ffmpeg.org/ffmpeg-filters.html), [NVIDIA FFmpeg hardware acceleration guide](https://docs.nvidia.com/video-technologies/video-codec-sdk/13.1/ffmpeg-with-nvidia-gpu/index.html). In particular, loudnorm's dynamic mode upsamples to 192 kHz; `linear=true` is a request subject to constraints, not a guarantee of cheap linear normalization. Input LRA of 22.6 exceeds target LRA 11 in the inspected second job.

## Measurement and regression gates

Existing tool diagnostics already record FFmpeg command labels and elapsed durations. First aggregate them into stable render evidence with item/track index, stage, attempt, encoder, media duration and overlapping start/end times; avoid creating a second logging system. Persist separate totals for voices, item wall time, program assembly, input analysis, candidate encoding/analysis, delivery and quality. UI progress weights are not a timing profile.

Use the existing captures as replay inputs for paired before/after runs: no new HLAE/CS2 recording. Match plan, captures, voice/assets, HUD telemetry, tools, drivers and encoder settings. Run candidates in alternating order and keep stage timings, total post-recording wall time, CPU/GPU utilization, RAM/VRAM, temporary bytes and output size. Time overlapping stages by their critical path, not by summing child durations.

For scheduling/shared-read changes, require identical prepared PCM/decoded video and unchanged acceptance evidence. For encode-topology changes, explicitly assess quality and codec differences rather than claiming byte identity. All candidates must preserve approved frames and samples, transitions, HUD stability, FACEIT/demo layout resolution, sponsor and voice boundaries, strict loudness/AAC acceptance, full decode, corruption rejection and atomic publication. Include cancellation, low disk space, failed encoders and retries.

Relevant existing coverage includes `full_demo_mix_test.go`, `full_demo_ducking_test.go`, `full_demo_voice_mix_test.go`, `full_demo_hud_transition_test.go`, `full_demo_transitions_test.go`, `full_demo_media_canary_test.go`, `filter_overlay_test.go`, `full_demo_audio_test.go` and `ffmpeg_efficiency_test.go`. Use bundled FFmpeg 8.1.2 for the media gates, respecting the repository's FFmpeg 9 compatibility caveat.

Implement and benchmark one change at a time: shared verification, bounded voice preparation, overlay encode consolidation, then audio-only retries. Retain independent timing evidence for each. A total render-time target should be set from those paired full-render measurements; this audit does not certify a specific 10-minute-to-X-minute improvement or promise zero regressions without implementing and validating the candidate.
