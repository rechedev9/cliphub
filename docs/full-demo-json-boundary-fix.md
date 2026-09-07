# Full Demo: empty event lists across recorder JSON

Studio 2.4.60 could finish every Full Demo round and then reject the result with
`recording result plan does not match the launched attempt`. Four attempts on
2026-09-07 reported this failure. The latest native console certified all 14
rounds, restored runtime settings and emitted the completed POV marker.

The worker constructs its expected recording plan from the approved Full Demo
document in memory. Rounds without kills or utility can carry non-nil empty
slices. The kill-plan and recording-result JSON use `omitempty`, so those same
lists reach the recorder and return to the worker as nil. `reflect.DeepEqual`
then rejects the otherwise identical plan. The installed executable matched
commit `42ee2da7784eabf06da8145aaf601e6607c76dc2`; its existing NVENC identity fix
was present. This failure also reproduces without NVENC.

The fix canonicalizes only empty kill and utility lists to nil when constructing
recording segments. It preserves the JSON contract, event filtering, capture
windows, encoder, POV verification and strict attempt comparison. No validation
is bypassed. A regression test serializes a valid Full Demo result with and
without NVENC and submits the decoded result to the real attempt validator.

The same session exposed an independent polling bug: the web proxy classified
HTTP 304 as an upstream redirect and replaced it with 502. A single exception
lets 304 reach the existing ETag/cache handler. Actual redirects remain blocked;
tests cover the complete helper path and 301/302/303/307/308 rejection.

Verification included the real approved plan and native console, complete Go
recording/worker/recorder test suites, web conditional-poll tests, TypeScript,
targeted lint and the production web build. A four-file hotfix was installed on
the affected PC with original-file SHA-256 checks, verified backups and readback:
the orchestrator plus three compiled web chunks. Each web chunk differs only by
the 16-byte `304===a.status||` guard. The installed backend and web server passed
an isolated native smoke check: health 200, initial jobs request 200, conditional
request 304, matching ETag and no redirect error.

Private evidence and the remote installation receipt are under
`.local/zack-full-demo-20260907/` and are excluded from Git. Remote backups are
under `%APPDATA%\cliphub-studio\support\20260907-full-demo-json-hotfix\originals`.
This was an installation hotfix to 2.4.60, not a new published release. No new
native capture/render was started; verification of a fresh complete video
remains separate from the successful capture already present in the console.
