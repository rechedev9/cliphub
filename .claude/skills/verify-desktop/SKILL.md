---
name: verify-desktop
description: Verify a ClipHub change in the real Studio desktop app (Electron) in the background, on a disposable copy of the user's Studio data. Use for every change with a runtime surface (web/, desktop/, or Go that reaches the API or UI) before calling it done or opening a PR, and for QA passes after merges.
metadata:
  zv-catalog: "false"
---

# Verify in the desktop app

Studio desktop is the product. A standalone `next start` or a browser tab is not the surface users meet, so verify there. `verify-desktop.mjs` builds this checkout into `desktop/build-resources`, then boots Electron through `desktop/e2e/isolated-userdata.cjs` with its own userData at `C:\temp\cliphub-verify\profile`. No window focus or OS input is needed, and the user's running Studio is never touched.

## Run it

From the root of the checkout you are verifying. Run one verify at a time per machine: every worktree shares the profile, and a new run stops the previous run's app.

```sh
node .claude/skills/verify-desktop/verify-desktop.mjs all --check <scratchpad>/check.mjs
```

- `all` = `prepare` + `run`. Cold, it takes about 1.5 min (web build 17 s). Once the profile exists, the app boots in about 3 s.
- Only `web/` changed: `prepare --skip-go --skip-desktop`, then `run`.
- Only Go changed: `prepare --skip-web --skip-desktop`.
- `run` alone reuses the last build.

`run` walks every screen at 1440, 1024 and 760 px. The screens are the sidebar sections plus, for up to 3 real jobs, the hub row and both produce pages. It records page errors, console errors, API responses of 400 or more, horizontal overflow, the boot error screen and a screenshot of each page. Then it runs your `--check`.

The summary prints only flagged pages and check results. It exits 1 on a flag or a failed check. Everything is written to `C:\temp\cliphub-verify\runs\<time>\` (`report.json`, `shots/`). Open only the screenshots of flagged pages and of your check.

Options:
- `--widths 1440,760` changes the widths.
- `--routes /clips,/players` replaces the default screens.
- `--no-pass` runs only the check.
- `--fresh-data` recopies the real data after the user's Studio changed.
- `--keep-open` leaves the app running.
- `--allow-writes` lets non-GET API calls through; see Safety.

## Write a check for the change

The route pass catches breakage. It does not prove your change. Always write a small check in the scratchpad that drives to the changed code, asserts what should now be true and probes one path off the happy path.

```js
// <scratchpad>/check.mjs
export default async function check({ page, origin, jobs, setWindow, settle, shot, expect, log, app }) {
  await setWindow(760);
  await page.goto(`${origin}/clips?partida=${jobs[0].id}`);
  await settle();
  const row = page.locator(`#partida-${jobs[0].id}`);
  const [rowRight, metaRight] = await row.evaluate((el) => [
    el.getBoundingClientRect().right,
    el.querySelector('button[aria-expanded] .text-body-sm').getBoundingClientRect().right,
  ]);
  expect(metaRight <= rowRight + 1, `metadata inside row (${metaRight} <= ${rowRight})`);
  await shot('hub-row-760', row);
}
```

`ctx` provides:
- `page`: the Playwright page of the Studio window.
- `app`: the ElectronApplication, for main-process state via `app.evaluate`.
- `origin`.
- `jobs`: `[{ id, status, map, player }]` from the copied data.
- `setWindow(width, height?)`, `settle()`, `shot(name, locator?)`, `expect(ok, message)` and `log(...)`.

A thrown error is reported as CRASHED, with a screenshot.

Prefer measuring over looking: bounding boxes, text, API responses and `report.json`. Look at a screenshot only to confirm what the numbers say.

Real data rarely contains the edge case. Edit the copy: `C:\temp\cliphub-verify\profile\data\jobs\<id>\*.json` holds names, plans and rosters. Delivered MP4s under `renders/**/videos/` and demos over 100 MB under `demos/` are hardlinks: never edit them in place. Restore the copy with `--fresh-data` afterwards.

## Before and after, for UI PRs

The screenshots must come from the app.

1. Run your check on the branch. Its `shot()` files land in that run's `shots/`.
2. For the baseline, restore only the changed files from the base, rebuild web, run the same check, then put the branch back:

```sh
git show origin/main:web/path/file.tsx > web/path/file.tsx
node .claude/skills/verify-desktop/verify-desktop.mjs all --skip-go --skip-desktop --no-pass --check <check>
git checkout -- web/path/file.tsx
node .claude/skills/verify-desktop/verify-desktop.mjs prepare --skip-go --skip-desktop
```

Attach the PNGs with `gh pr create --attach`, as `pr-create` describes.

## Safety

- Non-GET API calls made by the app's windows are blocked: they get a 204 and are listed as `blocked`, unless you pass `--allow-writes`. Calls from the Electron main process, from Next server code and from the orchestrator's own workers are not intercepted. The script therefore also strips every `ZV_*` variable from the app's environment, so the copy never polls the real Portal bridge. With writes allowed, every write lands in the copy. A capture would still launch CS2 through the profile's real HLAE, so allow writes only for a flow that needs them, and never start a recording without the user's go-ahead.
- The dev layout is unpackaged, so it sends no telemetry.
- `zv-orchestrator` embeds `FACEIT_API_KEY` from the environment with the same ldflag as `scripts/build.ps1`. It uses the `go` on PATH, not the pinned toolchain, and does not assert that the key was embedded. Errors redact the key; never print it yourself.
- Only the three binaries `assemble.mjs` ships are built, and the web build runs without `XAI_API_KEY`, as in the installer. HLAE is not staged: the profile reuses the real Studio's installed, hash-verified tools. If the branch changes the HLAE pin, the app provisions the new pin into the profile on boot; that path is not the installer's.
- Deleting a job in the copy removes only the hardlink, never the user's video.

## What this does not cover

- **Capture, HLAE and CS2:** use `zv verify doctor` and a real capture, as the AGENTS.md HLAE section describes.
- **Render output:** use the render lab (`zv-editor lab ...`, AGENTS.md). A delivered file that decodes is not proof the game is visible.
- **Installer, updater, native dialogs and the tray:** they have no DOM. Use computer use if the session has it; otherwise name the gap in the PR.

## Gotchas

- Git Bash rewrites `/route` args into `C:/Program Files/Git/route`. The script undoes it, and `routes` also accepts `clips,players`. For other path args, use forward slashes: the Bash tool collapses backslashes.
- If a web build fails with EBUSY on `.next/standalone`, a standalone server started outside this script is still running. Stop it first; the script already stops its own previous app.
- `tsc` fails on `.next/types` for a deleted route: remove `web/.next`. This does not affect this script.
- Web Playwright e2e run locally, never in CI (the user's call): `pnpm --dir web run build`, then `E2E_SKIP_BUILD=1 npx playwright test` from `web/`. The full suite (312 tests) took 2.7 min with 2 workers on a loaded machine.
- Go: `internal/editor` needs `go test -timeout 30m` (it takes about 11 min). `desktop` `smoke-launch-contract` fails in a fresh worktree until `bin/zv` exists.
- The first boot of a new profile copies about 260 MB of tools and the data under 100 MB, which takes seconds. The profile persists between runs; `.tools-ready` and `.data-ready` mark a finished copy, so an interrupted one is redone. Delete `C:\temp\cliphub-verify\profile` to start clean. `CLIPHUB_VERIFY_HOME` must stay on the same volume as `%APPDATA%` for the hardlinks.
- Known benign API states are listed in `KNOWN` in the script, each with a reason (for example, 409 on `/anticheat` means the analysis has not started). Add to it only with a reason, never to silence a real failure.
