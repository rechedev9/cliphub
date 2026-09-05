# Full Demo editorial CLI

`zv full-demo` drives the same `gameplay-pov-60` variant and workers as Studio.
It does not create another capture or rendering pipeline. Its profile is
`full-demo-pov-chill-v1`; legacy recap requests retain their historical behavior.

## Local session and inputs

Run the existing local orchestrator with `zv serve`, or use its already-running
session. Set `ORCHESTRATOR_URL` to its HTTP loopback origin and
`ZV_MUTATION_TOKEN` to its session token. `--url` overrides the origin. The new
commands reject remote origins and redirects; no token is accepted in argv.
Packaged Studio's session address is recorded in its `ports.json`; use the
actual session configuration, not a guessed fixed port or a second server.

`zv full-demo import --demo <match.dem> --steamid <SteamID64> --dry-run --format json`
validates the local input. Removing `--dry-run` uploads it through `POST
/api/jobs`, enqueues the existing parser and returns a job ID. Wait for
`zv full-demo inspect --job <uuid> --document status --format json` to report a
parsed job. An existing parsed Studio job can be used directly. Old parses
without independent Full Demo facts need to be parsed again.

## Complete options and declared assets

`zv full-demo defaults --out <options.json> --format json` writes every explicit
choice. Edit this JSON, including disabled switches and zero gains; partial,
null or unknown options are rejected. Music and sponsor are initially enabled
and block approval until declared assets are supplied or those choices are
explicitly disabled. The document includes capture/HUD/crosshair, freeze and
tail bounds, team voice availability policy, mix/playlist/ducking, sponsor
placement/narration, overlays and cover choices.

Import each asset with `zv full-demo asset --input <media> --provenance
<provenance.json> --dry-run --format json`, then remove `--dry-run`. The
provenance JSON has all these fields:

```json
{
  "schema_version": "1.0",
  "asset_sha256": "<actual lowercase SHA-256 of the file>",
  "title": "<asset title>",
  "creator": "<creator>",
  "source_url": "<authoritative HTTP(S) source or explicit local: declaration>",
  "permission": "<your permission/license declaration>",
  "attribution": "<required attribution, or an explicit empty string>"
}
```

The CLI verifies the supplied digest; the server independently hashes and
decodes the upload and stores immutable provenance. Copy its returned `id` and
`sha256` into the relevant music, sponsor or narration reference in options.
An import does not infer a license or certify permission on the user's behalf.

## Plan, inspect, approve and execute

`zv full-demo plan --job <uuid> --options <options.json> --out <plan.json>
--dry-run --format json` checks local JSON without contacting the server.
Removing `--dry-run` resolves current demo facts, voice and asset bytes, persists
an immutable server plan and saves that document locally. Planning never starts
CS2 or renders a program. Blockers and warnings remain visible in the result.

Inspect the complete document with `zv full-demo inspect --plan <plan.json>
--format json`. Review the player, format, every option and asset, round ranges,
timeline, estimated duration, death/safety rule, warnings and blockers. A local
file is not sufficient for admission: its plan must also exist on the server.

After approving that concrete brief and granting the current Windows/HLAE/CS2
run, validate the exact command:
`zv full-demo execute --job <uuid> --plan <plan.json> --approve <plan-hash>
--allow-safe-tail-trim=true --dry-run --format json`. Use `=false` instead when
that is the explicit rule in the document. Removing only `--dry-run` requests
real execution through `POST /api/jobs/{id}/generate`. Admission verifies the
server plan, facts, source bytes, asset provenance and approval again; stale
inputs fail instead of substituting defaults. The response means queued,
not rendered or verified. A retry carries that same complete snapshot.

All Full Demo dry-runs are local validation: no HTTP requests, artifact writes,
capture or rendering. They cannot prove server freshness or media quality.
Use `zv workflows show full-demo-execute --format json` to inspect the executable
contract. `zv flows show demo --format json` describes the common demo pipeline.

## Durable evidence and failures

Read current evidence with `zv full-demo inspect --job <uuid> --document
<approved|effective|audio|loudness|delivery> --format json`; `--out` optionally
exports it. The current status still owns readiness: a previous committed file
can remain accessible after a failed replacement and is not proof that new
settings succeeded. Library retains the matching approval and evidence links.

| API | Contract |
|---|---|
| `GET /api/jobs/{id}/full-demo/plan` | Current document, defaults and compatibility |
| `POST /api/jobs/{id}/full-demo/plan` | Complete `options`; persisted draft, no capture |
| `GET /api/jobs/{id}/full-demo/plans/{planID}` | One immutable stored plan |
| `POST /api/jobs/{id}/generate` | Existing admission with `edit.full_demo` approval snapshot |
| `GET /api/jobs/{id}/renders/{variant}/full-demo/{document}` | Current approved/effective/audio/loudness/delivery JSON |
| `GET /api/jobs/{id}/renders/{variant}/revisions/{revision}/full-demo/{document}` | Evidence from one committed revision |
| `POST /api/editor/assets` | Existing bounded multipart asset import with provenance |

Recorder and editor workers transport documents using `--full-demo-plan` and
`--full-demo-execution` in the existing runtime binaries. These internal files
are not alternate approval paths. No new runtime executable is required.

See [implementation evidence](FULL_DEMO_IMPLEMENTATION.md) for executed checks
and remaining gaps. Synthetic media, CLI tests and simulated capture contracts
do not certify Windows Studio, HLAE or CS2.
