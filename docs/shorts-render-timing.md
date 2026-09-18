# Shorts pack render timing

The shorts pack renders several shorts at once through a bounded pool of render
job slots (`RenderJobs`, `normalizeRenderJobs`). Each short holds one slot from
the moment the pool grants it until its goroutine returns, which today covers
the encode **and** every post-encode step (output probe, publish, quality check,
covers). Whether it is worth releasing the slot earlier — right after the encode
— depends on how much of the occupancy is actually post-encode work, and on how
much of that work overlaps anyway.

`RenderPerformance.short_pack_timing` records that. It is evidence only: it
measures the intervals the render already measured, and it changes nothing about
scheduling, the pool size or when a slot is released.

## What is recorded

Each short's `performance.short_pack_timing` holds:

- `index` — the short's index, the same index used in `shorts[]`.
- `spans[]` — one entry per completed stage interval, with `stage`, `index`,
  optional `variant`, `start_ms`, `end_ms`, `elapsed_ms` and `outcome`.
- `stages[]` — per-stage aggregate: `spans`, `wall_ms`, `process_elapsed_sum_ms`.
- `wall_ms`, `process_elapsed_sum_ms` — the same two numbers for the whole short.
- `slot` — the render job slot occupancy (below).
- `truncated` — set if the pack exceeded the 4096-span cap.

Stages: `encode`, `probe` (variants `output`, `cover`, `cover-sheet`), `publish`,
`quality_check`, `cover`, `cover_sheet`. Outcomes are `ok`, `error`, or
`cancelled` when the failure happened under a cancelled context, exactly as in
the Full Demo collector.

`start_ms`/`end_ms` are offsets from one collector started at the beginning of
the pack render, so offsets of different shorts are directly comparable and pool
overlap is readable: two shorts whose slot intervals intersect ran together.

Spans carry no paths, command arguments or user strings — only fixed
code-controlled identifiers, the index and the interval.

## Reading wall versus elapsed sum

Same convention as `full_demo_timing` (see
`docs/full-demo-render-performance-audit.md`, "Timing evidence"):

- `wall_ms` is the **union** of active intervals: the time during which at least
  one span was running. Gaps between spans are excluded.
- `process_elapsed_sum_ms` is the **sum** of span elapsed times: what the work
  would cost if it ran serially. It is process/stage elapsed time, **not CPU
  time** and not overlap-aware wall time. `quality_check`, `cover` and
  `cover_sheet` are one subprocess each; `probe` is a stat plus one ffprobe
  when an ffprobe path is configured; `publish` is a hardlink (or a byte copy
  on the cross-device fallback); a Full Demo `encode` covers the whole program
  render, which is many subprocesses.
- `wall_ms < process_elapsed_sum_ms` means the post-encode fan-out
  (publish ∥ quality check ∥ covers) really overlapped.
- Spans measure the same intervals as `render_ms`, `probe_ms`,
  `quality_check_ms`, `cover_ms` and `cover_sheet_ms`, so they must never be
  **added** to `render_ms`. The encode span's `elapsed_ms` is `render_ms`.

## Slot occupancy

`slot` splits one short's slot occupancy at the encode boundary:

| Field | Meaning |
| --- | --- |
| `start_ms` / `end_ms` | when the pool granted and released the slot |
| `held_ms` | total occupancy; always `>= encode_ms` |
| `pre_encode_ms` | granted → encode start (directory creation, setup) |
| `encode_ms` | the encode itself (union of encode spans) |
| `post_encode_ms` | encode end → slot released |

A reused output has no encode span: `pre_encode_ms` and `encode_ms` are 0 and the
whole occupancy is `post_encode_ms`.

Full Demo shorts record the delivery decode (including its optional quality
filters) inside their encode span, because for them that work happens during the
program render; the internal breakdown stays in `full_demo_timing`. So their
`post_encode_ms` is genuinely small by construction, and the question below is
about plain shorts.

## Is releasing the slot before the post-encode work worth it?

Read it per short, then across the pack:

1. **Is there a tail at all?** `post_encode_ms / held_ms`. A pack where this is a
   couple of percent has nothing to recover; the optimization would only add a
   second synchronization point.
2. **How big is the tail in absolute terms?** The upper bound on what an early
   release could hand to the next queued short is the sum of `post_encode_ms`
   over all shorts, and it can only be realized while shorts are still waiting
   for a slot. If the last shorts started with an empty queue, their tails
   recover nothing.
3. **Was the pool actually saturated?** Compare slot intervals across shorts. If
   at any time fewer slots were held than `RenderJobs`, nothing was queued then
   and a tail released at that moment is dead time either way.
4. **What would move into the unbounded region?** The post-encode stages of a
   short whose slot was released keep running concurrently with the next short's
   encode. `stages[]` for `quality_check` and `cover*` says how heavy that is:
   the quality check is a full decode, so releasing the slot trades a bounded
   encode pool for encode + N decodes running together. If
   `quality_check.process_elapsed_sum_ms` is on the order of the encode, the
   change moves contention rather than removing it.

A worked decision therefore needs: per-short `held_ms`, `encode_ms`,
`post_encode_ms`, the overlap picture from `start_ms`/`end_ms`, and the
`quality_check` stage cost. All of it is in this record; none of it requires a
new measurement run beyond one normal render.

## Caveats

- Millisecond offsets are truncated independently, so `pre + encode + post` may
  differ from `held_ms` by 1 ms. Do not treat the split as exact arithmetic.
- For the same reason `wall_ms` (computed from `start_ms`/`end_ms`) can exceed
  `process_elapsed_sum_ms` (the sum of `elapsed_ms`) by up to 1 ms per span,
  even for a single-span stage (for example `wall_ms: 946`,
  `process_elapsed_sum_ms: 945`). Only a difference larger than the span count
  means real overlap or a real gap; `elapsed_ms` is the exact counter value.
- `cancelled` is derived from the context at the moment the span is recorded,
  as in `full_demo_timing`: once a sibling short has failed and cancelled the
  pack, a stage of another short that fails for its own reason is still
  recorded as `cancelled`. The first failure of a pack is always `error`.
- The record is attached after the slot is released and all stages returned. A
  short that failed before its performance record existed has no timing record.
  Shorts that did reach a performance record keep their timing evidence in the
  failed `shorts-result.json` too, with `error`/`cancelled` outcomes.
- The cap is 4096 spans per pack render; spans past it are dropped and
  `truncated` is set on every short snapshotted after the cap was hit. A
  dropped `encode` span makes that short's slot split read like a reused output.
