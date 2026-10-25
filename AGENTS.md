# ClipHub

ClipHub Studio is a Windows desktop app that turns CS2 demos and streams into edited videos.

## Layout
- `cmd/`, `internal/`: Go backend (orchestrator, parser, recorder, editor). Go 1.26.10 from `go.mod`.
- `web/`: Studio UI (Next.js, standalone build). `desktop/`: Electron shell that bundles the Go
  binaries, the web build and HLAE into the Windows installer.
- `landing/`: separate Next.js site. `deploy/`: telemetry deploy files.
- Node 24, pnpm 11.22.0: `pnpm --dir <web|desktop> install --frozen-lockfile`.

## Build and check
- Go: `go vet ./...`, `go test ./...`, and `go run ./cmd/zv check`. A few Full Demo editor tests depend on the FFmpeg
  version; Studio ships FFmpeg 8.1.2.
- `go test ./...` includes `TestNoDeadCode` (`cmd/zv`), which runs pinned `deadcode` and `staticcheck -checks U1000`
  for both `GOOS=windows` and `GOOS=linux` (CI). A function that only tests call is dead: delete it, or move a
  real test helper into a `_test.go` file. A helper only `_windows.go` code calls belongs in a `_windows.go` file.
- Fast local Go loop: `go test -short ./...` skips the slow FFmpeg renders; CI and pre-PR runs use the full suite.
- web, desktop: `pnpm --dir <pkg> run typecheck`, `lint`, `test:unit`; `run build` for web.
- CI runs on pushes to main, tags and nightly, not on PRs. Run the checks for what you touched.

## Run Studio from source (Windows)
1. `.\scripts\build.ps1` builds the Go binaries into `bin\`.
2. `pnpm --dir desktop run build`, then `pnpm --dir desktop run assemble` (stages `bin\`, the web
   standalone build and HLAE into `desktop\build-resources`).
3. Start it with a throwaway profile, never the real `%APPDATA%\cliphub-studio`:
   `$env:CLIPHUB_E2E_USER_DATA = "C:\temp\cliphub-try\$(Get-Date -f yyyyMMdd-HHmmss)"`
   `pnpm --dir desktop exec electron scripts\isolated-userdata.cjs`
   A new profile shows the first-run guide and fetches HLAE on first boot.
4. Capture needs Steam and CS2 on the machine. Use demo files from outside the repo.

## Verify a change
Verify by hand in the real app:
1. Build and start Studio from your branch as above, with a fresh throwaway profile.
2. Do the flow you changed the way a user would, plus one path off the happy path
   (error, cancel or empty state).
3. Attach screenshots of the relevant states to the PR, with a few lines on what you did and saw.
Do not add e2e suites, verification scripts, evidence folders or reports to the repo.
The one exception is the deterministic Short pipeline test in `internal/pipeline`: plain `go test`,
golden files in its `testdata/`, no CS2, network, real clock or video encode, and no scripts beside it.
Extend that test instead of adding another. When a change moves its output, run
`go test ./internal/pipeline -update` and review the golden diff like code; never accept it blindly.
It does not replace the manual check above. Unit tests are welcome where they test real logic.

## Releases
Bump `desktop/package.json` and `landing/app/page.tsx` (`DOWNLOAD_URL`, `RELEASE_VERSION`), commit
`chore(release): prepare Studio X.Y.Z`, then push an annotated tag `vX.Y.Z` ("ClipHub Studio X.Y.Z").
`desktop-release.yml` publishes the GitHub Release as `latest`, which is what the in-app updater reads.

Domain rules and incident history (HLAE pin, Full Demo audio, telemetry): `docs/incidents.md`.
