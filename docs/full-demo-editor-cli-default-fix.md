# Full Demo render: inherited kill-clip tail trim

On 2026-09-07, job `370a14df-1675-41cb-9285-10b131d183fb` completed and committed
all 14 native captures at 16:26:44 local time. Four seconds later, the render
worker failed with `full demo execution cannot be overridden by legacy editorial
flags`. The recording revision remained reusable; capture had succeeded.

The worker supplied the approved Full Demo execution document and explicitly
disabled hook, kill counter, intro and outro. It omitted `--tail-trim`, whose
editor CLI default is 1.5 seconds. The editor correctly rejects that legacy
timing override because the approved document owns Full Demo timing.

The worker now passes `--tail-trim=0` together with `--full-demo-execution`.
Shorts retain their existing default, and the editor's strict approval guard
remains unchanged. A regression test builds and runs the real editor CLI:
worker arguments produce a Full Demo dry-run manifest; the previous arguments
and an explicitly conflicting trim both retain the original rejection.

The installed editor was also checked on the affected PC using the original
approved snapshot, 14 committed clips, five stored voice tracks and roster
overlays. The previous invocation reproduced the rejection; the corrected
invocation returned `ok: true`, `dry_run: true`, one compilation and no warnings.
This verifies preparation and source digest checks, not a completed FFmpeg
export. No capture or production job was launched during this check.

Private inputs are under `.local/zack-full-demo-render-20260907/`. Remote
diagnostic outputs are under
`%APPDATA%/cliphub-studio/support/20260907-full-demo-render-default/`.
