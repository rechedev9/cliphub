# Full Demo POV Chill: implementation and verification

This change implements the `full-demo-pov-chill-v1` profile inside the existing `gameplay-pov-60` flow. It is not certified for production capture: real Windows Studio/HLAE/CS2 verification is still outstanding. Do not merge on the strength of synthetic evidence alone.

## Baseline and scope

- Specification: `CLIPHUB_FULL_DEMO_ASTRA_PLAN.md`, read in full (604 lines); SHA-256 `ca21fdecfd72aec98f1a5c2b535b13bbc48780df22712730376bee25d2a562dd`.
- Initial clean HEAD: `0d0f10bd2689a9152da6cd96b87e2a35ec1a1d64`.
- Branch: `codex/full-demo-pov-chill`.
- During verification, upstream added `0586509e322df967c0e4825f72ab29d42fc2589f`, retiring root instruction files. Upstream then fixed its checks in cf412c587ac47faa707f020600e2aa3ad3c7b5f8. The branch includes both commits, preserves that solution and does not restore the retired files.
- Host: Windows, Go 1.26.6, Node 24.18.0, pnpm 11.22.0, FFmpeg 8.1.1.
- Local ignored evidence: `data/full-demo-astra-evidence/`. Generated media, installer payloads and third-party assets are not committed.

## Requirements and code

| Phase | Implemented behavior | Behavior-level evidence |
|---|---|---|
| P0 — baseline | HEAD, status, complete specification and repository contracts inspected; existing parser/recording/editor/task paths retained | Baseline log and final diff; no additional pipeline or runtime executable |
| P1 — contracts | `internal/recapplan`: versioned options/facts/document/snapshot, bounded strict JSON, explicit false/zero, semantic approval/capture hashes, canonical frame/sample timeline, immutable stored plans | Domain tables, Go/Studio serialized fixture agreement, missing/null/unknown/duplicate/limit rejection |
| P2 — editorial facts | Parser persists independent all-round facts, including zero kills, warmup resets, freeze deaths, halftime/OT, target crosshair changes; planner bounds freeze/voice/death/survival and sponsor placement | Parser event-sequence fixtures, planner boundary tables and the supplied real Ancient demo: 23 complete rounds, including 10 zero-kill rounds |
| P3 — capture | Existing recorder consumes approved document; exact windows, SteamID/first-person checks, zero unknown/drift budget, native HUD/crosshair cvar readback, game voice disabled, settings/crash journal, immutable clip revisions and original-run receipts | 13 generated-script simulator scenarios, journal/restore/window-file tests, partial-recapture coverage tests; real game behavior unverified |
| P4 — audio | Temporal team voice extraction and typed availability; bounded spill/cancellation; long silent intervals; reference-normalized ordered music with gameplay-only cursor; explicit gains, ducking/attack/release, sample-exact buses | Real Opus encode/decode across 31 minutes and side changes; 11 decoded frequency-band ducking/zero scenarios; mastering loudness/transient/mute tables |
| P5 — sponsor/output | Actual 1080p60 fit/pad video, embedded/replacement audio, automatic/manual/split insertion, playlist pause/resume, no game/music during sponsor; effective tail rebasing; full-program two-pass master with decoded AAC remediation and delivery checks | Four independent five-second/300-frame FFmpeg canaries: embedded, narration replacement, manual split, playlist exhaustion; visual/audio boundaries, cover selection, decode and loudness checks |
| P6 — integration | Existing Full POV UI and same-origin proxy, asset provenance/preview, visible blockers, content-bound approval; CLI defaults/import/asset/plan/inspect/execute through existing API/tasks; Library evidence/downloads pinned to immutable revisions | API planning/upload/admission/retry tests, CLI/catalog tests, web lint/types/unit/build, 18 Chromium Full Demo + Library tests, seven actual Electron shell/proxy tests |
| P7 — durability | Persisted approvals in task/render intent, strict rehydration on retry, generation ownership fences, queue compensation, immutable capture and render publication, byte/digest verification on cache reuse, narrow/expanded coverage behavior | Worker/cache/stale-owner/restart/compensation tables, complete Go and race suites, synthetic application seams |
| P8 — packaging/verification | Existing three-runtime desktop package, unsigned NSIS installer, reproducible evidence documents and read-only doctor/prove | Installer build/integrity, unsigned signatures and packaged/runtime byte equality; hardware preflight failed closed. See verification boundary below |

Every Full Demo option travels in the approved document through UI/API, durable plan, task, capture/execution identity, effective render, cache/retry and Library evidence. Capture-only choices affect capture identity; audio/sponsor/output changes invalidate rendering without silently recapturing compatible footage. Legacy requests without the new snapshot retain their existing path. New snapshots cannot fall through to legacy defaults.

The CLI contract and concrete commands are documented in [FULL_DEMO_CLI.md](FULL_DEMO_CLI.md). `full-demo execute` requires the exact approved hash and an explicit safe-tail-trim choice. Synthetic fixtures under `testdata/full-demo-*` do not attest to a real capture.

## Executed verification

Logs below record actual local runs, not a promise of coverage.

| Check | Result / evidence |
|---|---|
| `go test ./... -count=1 -timeout 5m -json` | PASS; `go-tests.jsonl`. `scripts/ci-backend-evidence.mjs` accepted all mandatory canaries without skips |
| `go test -race ./... -count=1 -timeout 10m` | PASS; `go-race.txt`. Added timestamp-bound rejection checked separately with race |
| `go vet ./...` | PASS; `go-vet.txt` |
| `go run ./cmd/zv check` | PASS; catalog includes 35 workflows; `zv-check-final.txt` (seven workflow documents). The complete CLI suite was repeated after rebasing |
| Node simulator and backend evidence contracts | 17 PASS; `node-capturelab.txt` |
| Web lint, typecheck, unit, production build | PASS; corresponding `web-*.txt` logs |
| Chromium Full Demo + Library | 18 PASS; `web-e2e.txt`, `web-report/`. Actual production frontend with controlled HTTP responses, not a live capture worker |
| Desktop lint, typecheck, unit, build | PASS; `desktop-*.txt` |
| Actual Electron dev-layout shell using assembled resources | Seven PASS; `desktop-e2e-final.txt`. Boots real orchestrator/Next, checks proxy, native minimization/focus and clipboard. Computer Use visually confirmed the clips hub. No game launched |
| `pnpm --dir desktop run dist` and `verify:dist-integrity` | PASS; `desktop-dist-final.txt`, `desktop-integrity-final.txt`, `desktop-unsigned-runtime-final.json`. Installer created locally; NSIS installation itself not exercised |
| `govulncheck` | No reachable vulnerabilities; `govulncheck.txt`. Four advisory module matches reported as unreachable |
| `staticcheck` | New diagnostics corrected. Six unused functions remain in unchanged legacy overlay/editor files; `staticcheck-final.txt`. Aggregate gate is not green |
| `gosec` | Findings retained for review in `gosec-final.json` and `security-review.json` (27 remaining findings); includes existing findings and explicit local-path/bounded conversion warnings. A valid missing upper tick bound was fixed and tested. Do not report the scanner as passing |

Capture Lab Full passed through L4, including both live Studio download and 18 presentation cases. Evidence is stored separately under `capture-lab-final-pass/`; its summary is authoritative for the highest completed level. Its application proof composes actual HTTP/queue tests with a build-tagged in-memory ready-result seed served through the real orchestrator, proxy and Studio. It is not one uninterrupted Full Demo production worker lifecycle and cannot certify HLAE/CS2.

### Media and failures discovered

`audio/` contains the four synthetic MP4s and matching JSON reports, decoded frequency-band measurements, and mastering evidence. They use generated colors/tones, not game recordings or downloaded music. Checks found and corrected a 33 ms concat timestamp loss, sidechain EOF truncation (including a 17-sample boundary remainder), and a cover selection window that could choose the following sponsor.

Capture Lab's pre-existing fixture stretched two/three-second tick windows into fixed five-second fake clips. Its corrected fixture keeps five-second protected windows and EOF headroom; the independent duration/event checks remain unchanged. Electron's old test expected `/onboarding`; production code redirects to `/clips`, so its route and document-title assertions now match that code. The live Studio fixture also now carries all source segment IDs, preventing its old one-segment selection from correctly triggering a mismatch re-drive against a two-segment render. Windows pnpm shims are invoked through a fixed repository-owned command. Initial failed logs remain alongside the passing reruns.

The assembler required 15 existing bundled catalog MP3s absent from this worktree. They were copied from the main local checkout into ignored `data/music`, with SHA-256 equality recorded in `packaged-catalog-bytes.json`. No new music was fetched or licensed as part of this implementation.

## Real verification boundary

Read-only `doctor.json`, `prove-full-demo.json` and `prove-shorts.json` failed closed with `hlae_cs2_windows_studio`: the host-of-record Studio health endpoint was unavailable and CS2 was not running. A separately isolated Electron test instance subsequently booted successfully. The final packaged Studio was also launched at the canonical profile; doctor-final.json reports windows_studio=true and cs2=false, so the host is still not capture-certified. Final prove documents are retained alongside the earlier failures.

CS2 being stopped while Studio is idle is expected: the recording worker launches it through HLAE. Doctor is a live-state certification check, not a prerequisite requiring the user to start CS2 manually. The real recording was paused by the Computer Use interruption, not by the idle doctor result.

Not executed:

- Actual HLAE/CS2 capture, native HUD/POV/crosshair visual verification or real runtime settings restoration.
- A complete production Full Demo capture/render worker run or a full-match AV drift check. Real `.dem` roster, parser-worker and persisted round-facts checks did run successfully.
- Real Shorts capture-to-Library regression or final user-approved media delivery.
- NSIS installation, release publication, or a merge.

The user subsequently approved the real canary brief and supplied spirit-vs-furia-m1-ancient.dem, selecting donk (76561198386265483). The approved brief is all rounds, 1080p60 landscape, strict POV/native clean HUD/observed crosshair without fallback, team voice and game gain 1, no music/sponsor, hard cuts, counter/intro/outro/covers off. Computer Use was then stopped with physical Escape, so no HLAE/CS2 capture was initiated. The authorization and selection are recorded; UI control remains paused. Full Demo and Shorts remain unknown at the production-real gate, and the PR remains a draft.

## Runtime source references

The checked-in pipeline is the implementation contract. Runtime format/API details were checked against primary sources; those documents do not substitute for running the installed game:

- [HLAE mirv declarations](https://github.com/advancedfx/advancedfx/blob/main/misc/mirv-script/src/types/mirv.d.ts), [cvar declarations](https://github.com/advancedfx/advancedfx/blob/main/misc/mirv-script/src/types/cvar.d.ts), and [panorama commands](https://github.com/advancedfx/advancedfx/wiki/Source2:mirv_panorama).
- GameTracking-CS2 extracted schema for `OBS_MODE_IN_EYE = 2`; local demoinfocs 5.2.0 for `Player.CrosshairCode()`.
- [Crosshair share-code layout](https://github.com/akiver/csgo-sharecode/blob/main/src/index.ts), including checksum and signed gap values.
- [FFmpeg filters](https://ffmpeg.org/ffmpeg-filters.html) and [loudnorm implementation](https://github.com/FFmpeg/FFmpeg/blob/master/libavfilter/af_loudnorm.c).
