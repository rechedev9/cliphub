---
name: chperf
description: Measure ClipHub performance with the chperf C tool - pipeline stage times (parse, record, render), game capture speed and black-capture bitrate check, Full Demo render stage breakdown, capture+render cost per match, Go microbenchmarks, live CPU/memory of Studio, workers, ffmpeg and CS2, orchestrator HTTP latency, and before/after comparison of any command. Use when asked how fast or slow something is, to find a bottleneck, to prove a performance change, before claiming a speedup, or to sanity-check that a capture is not black.
metadata:
  zv-catalog: "false"
---

# chperf

`bin/chperf` (C, no dependencies) prints one JSON document per call. Numbers
come from evidence ClipHub already writes plus live OS counters; nothing is
estimated.

## Build

```sh
sh tools/chperf/build.sh        # -> bin/chperf.exe (Git Bash + scoop gcc) or bin/chperf
sh tools/chperf/build.sh test   # unit + CLI tests
```

## Start here

```sh
bin/chperf snapshot             # system, pipeline, renders, captures, 3 s process sample, HTTP
bin/chperf help                 # every command, flag and convention, as JSON
```

Read `findings[]` first: stable `code`, `level`, one-line `message`
(`pipeline_slower`, `capture_suspect_black`, `render_dominant_stage`,
`bench_noisy`, `http_needs_token`...). Many carry a `hint`: the next command
or check that drills into it. Then drill into `data`. Exit codes: 0 ok, 1 regression or failed child,
2 usage, 3 runtime error. Units are in key suffixes (`_ms`, `_s`, `_bytes`,
`_percent`); stats blocks are `{n, min, p50, p90, p95, max, mean, stddev}`.

## Which command answers what

| Question | Command |
| --- | --- |
| How long do parse / record / render take, are they getting slower? | `pipeline [--since RFC3339] [--last N]` |
| Where does a Full Demo render spend its time? | `renders [--job ID] [--limit N] [--full]` |
| How fast is the HLAE/CS2 capture, is a take black, capture vs render share? | `captures [--job ID] [--limit N]` |
| Is a Go hot path faster or slower after my change? | `bench --pkg ./internal/X [--bench REGEXP] [--count 5]` |
| What are Studio, zv-*, ffmpeg, cs2 using right now? | `procs [--seconds N] [--match NAME] [--pid P]` |
| How fast is a command, a test, a CLI stage? | `run --repeat 5 --warmup 1 -- CMD ARGS` |
| Is the orchestrator API slow? | `http [--path /api/jobs] [--n 50]` |
| Did my change make it faster or slower? | `compare BEFORE.json AFTER.json` |

## Proving a performance change

1. `bin/chperf --out before.json run --repeat 5 --warmup 1 -- <cmd>` (or `snapshot`).
2. Make the change, rebuild.
3. `bin/chperf --out after.json run --repeat 5 --warmup 1 -- <cmd>`.
4. `bin/chperf compare before.json after.json`: `verdict` is
   `regress | improve | same`; exit 1 on regression.

For Go code, use `bench` instead of `run` in steps 1 and 3: it runs
`go test -run ^$ -bench REGEXP -benchmem -count N` and `compare` gates the
per-benchmark `ns_per_op` / `bytes_per_op` / `allocs_per_op` medians (paired
by `pkg` + `name`). Use `--count 5` or more; `bench_noisy` means the CV of
ns/op is over 5 %. `bench --from FILE` parses output saved from a previous
`go test -bench` run.

Trust a delta only when `wall_cv_percent` is under 10 (`run_noisy` otherwise)
and the delta is larger than the run-to-run spread. For renders and captures,
compare the evidence of two real runs (`renders`, `pipeline`), not one run
against memory.

## Reading the numbers

- Data dir: `--data-dir`, else `ZV_DATA_DIR`, else the installed Studio's
  (`%APPDATA%/cliphub-studio/data`), else `./data`. `system` reports which.
- `pipeline` reads `obs/spans.jsonl` (+ rotated `.1`-`.4`) written by every
  worker, and `obs/journal.jsonl` for errors by class.
- `renders` reads `render-result.json` → `performance.full_demo_timing`.
  Stage `wall_ms` is the union of that stage's ffmpeg intervals; stages
  overlap, so shares can sum past 100 %. `parallelism_ratio` = process time
  sum / wall. `outside_tool_spans_ms` = render time not inside any ffmpeg
  process (Go-side work and gaps).
- `renders` items also report `output_bitrate_mbps`; a Full Demo delivered
  under 6 Mb/s raises `render_suspect_black`.
- `captures` reads `jobs/<job>/recording/recording-result.json`.
  `phases_ms` sums every recorder run; `incremental_mux_ms` overlaps
  `launch_and_capture_ms`, and `before_result_write_ms` is the recorder
  total. `capture_s_per_video_s` = HLAE/CS2 time per second of captured
  video (under 1 means faster than real time). `raw_video` is what HLAE
  wrote: a take under 0.05 bits per pixel per frame raises
  `capture_suspect_black` (the 5.0.0 black capture was ~2.4 Mb/s against
  ~40 Mb/s normal at 1080p60 CRF 18). This is a cheap first check; confirm
  with `blackdetect` / `signalstats` on frames. `end_to_end` adds the newest
  render of the same job, so `capture_share_percent` says whether capture or
  render dominates one match.
- `procs` CPU is percent of the whole machine (one busy core of 16 = 6.25).
  Children of a matched process keep its role, so a worker's ffmpeg counts.
  GPU is not sampled.
- `run` measures the whole process tree (Windows job object; Linux process
  group + `RUSAGE_CHILDREN`). Child output goes to a temp file; its tail
  appears on failure or with `--show-output`. Windows `peak_memory_bytes` is
  committed job memory; Linux is max child RSS. Commands must be executables
  (`cmd /c` or `sh -c` for scripts).
- `http` only talks to loopback. Protected routes need the Studio session
  token in `ZV_MUTATION_TOKEN` (or `--token-env NAME`); it is sent as
  `X-ClipHub-Token` and never printed. Without it only `/healthz` is measured.

## Do not

- Do not start `procs` or `run` workloads while a capture is recording to
  "see the load": sampling is cheap, but extra work you launch competes with
  HLAE/CS2 and can drop frames.
- Do not present stage `wall_ms` values as additive or as CPU time.
