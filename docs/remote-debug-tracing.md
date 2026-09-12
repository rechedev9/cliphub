# Remote debugging and trace delivery

The debugging contract is to reconstruct a failing Studio job from the collector,
including the first subprocess failure, its surrounding technical output, the
job and attempt that produced it, and any missing diagnostic data. A generic
`exit status 1` is not sufficient evidence.

## Implementation requirements

- Preserve the existing error and performance event API for released clients.
- Add durable, filtered Studio logs with acknowledged, idempotent delivery to
  the collector. Persist before upload, resume after restart, and retain pending
  data on network/server failure. Bounded storage must report gaps explicitly.
- Record every task attempt's start, finish, outcome and duration without
  sampling. Correlate subprocess output with the job and attempt. Do not log
  command arguments, environment variables, media contents or task payloads.
- Keep original local logs and existing diagnostic choices. Filter technical
  text before it enters the upload spool and again at the collector.
- Expose authenticated, paginated queries by job, installation and session,
  and delivery health, so operators can distinguish a successful job, an
  unfinished trace, a failed job and unavailable evidence.
- Verify a real subprocess failure through the production desktop logging and
  upload modules into a real collector, then reconstruct its cause using only
  the remote query. Also cover recovery, consent, redaction, retention,
  concurrency, rejection and backward compatibility.

## Delivery boundaries

Previously released clients upload selected error events only; logs they never
sent cannot be recovered from the collector. New collection starts with the
updated client and follows its diagnostic setting. Collector support must be
installed before clients use the new log endpoint.

The log channel does not sample. It collects new Studio output, queue attempt
lifecycles and heartbeats, live worker stderr, FFmpeg stderr during encoding,
HTTP mutation/failure summaries, Electron errors and renderer exceptions.
React error boundaries, uncaught event-handler errors and unhandled promise
rejections use the same filtered renderer channel. Successful HTTP polling and
media bodies are excluded. Subprocess stdout remains a caller transport and is
not copied independently when it can carry binary or JSON results.

## Operator workflow

Use the existing private agent configuration
`~/.config/cliphub/telemetry-agent.env`, or set
`CLIPHUB_TELEMETRY_ADMIN_URL` and `CLIPHUB_TELEMETRY_ADMIN_TOKEN` in the process
environment. Never put the admin token in a command argument, desktop build or
support report. The URL must be HTTPS or numeric loopback through an SSH tunnel.

```sh
node scripts/telemetry-debug.mjs --job JOB_UUID --out diagnostic-report.json
node scripts/telemetry-debug.mjs --support CH-XXXX-XXXX-XXXX-XXXX-XXXX --out installation-report.json
node scripts/telemetry-debug.mjs --session SESSION_UUID --since 2026-09-12T00:00:00Z --out session-report.json
```

The report includes all paginated job records, unassigned context from the same
Studio sessions, distinct attempts, the first error and preceding stderr, and
legacy incident events. Installation delivery gaps and health may originate
from another session or job; they are explicitly labelled separately. A missing
terminal event yields `no_terminal_record`, never success. `retrieval_complete`
becomes false at the operator's retrieval cap; `legacy_limit_reached` identifies
the older endpoint's 200-event limit. `since` filters receipt time for logs and
event time for legacy incidents. Raw records retain both clocks and a per-session
sequence, so clock skew does not invalidate stored evidence.

`GET /v1/logs` is admin-only and accepts `job_id`, `support_code`, `session_id`,
`event`, `level`, `since`, `after` and `limit` (maximum 500). Follow `next_cursor`
while `has_more` is true. `GET /v1/stats` additionally returns log storage bytes,
record count, last receipt and reported lost-record count. The public endpoint
is `POST /v1/logs`, authenticated with the existing ingest key. A 202 response
includes the exact `accepted_ids` only after a SQLite FULL-synchronous commit.
Identical retries retain the same IDs; conflicting identities are rejected.

## Capacity and incomplete evidence

- The desktop fsyncs filtered JSONL segments before upload. It retains pending
  records through offline periods, lost receipts and restarts. Only an exact
  receipt advances the durable cursor. Revocation aborts active requests and
  purges pending files; a later opt-in cannot replay revoked data.
- The spool is capped at 64 MiB with 256 KiB segments. It evicts oldest pending
  segments under pressure and persists a `delivery.gap`. Damaged segments,
  missing consent metadata, overlong subprocess lines and unrecoverable
  rejection are also explicit. Disk failures appear in local delivery status;
  they cannot be remotely confirmed while the machine cannot persist or send.
- Each filtered message is chunked to at most 16 KiB. A single input is bounded
  before filtering and records truncation; process lines beyond their bounded
  reader capacity are omitted with a gap. Private-key blocks are omitted.
- The collector uses a separate `<database>.logs` SQLite database capped at
  2 GiB plus bounded WAL overhead. Existing schema-2 event rows remain intact.
  Storage exhaustion returns 503 so the desktop retains evidence. Receipt-time
  retention is 30 days. Byte admission limits are 32 MiB per source per hour and
  128 MiB globally per hour, with request limits of 120/source/minute and
  1200/global/minute. Sources behind one proxy share its budget.
- Settings show pending bytes, last confirmed receipt, loss/rejection counts
  and a warning when delivery is not confirmed. Those are evidence boundaries:
  an offline/crashed/uninstalled client cannot guarantee immediate remote logs.

## Rollout and recovery

Deploy the collector before distributing the updated Studio. Back up the old
binary and both SQLite databases using SQLite's backup API (include `.logs` if
present), then verify backup integrity. Keep credentials and service bindings
unchanged. Replace the binary atomically, restart, check service health, test
authenticated log ingestion/query and verify legacy stats/incidents still work.

The original event database stays at schema 2; the additional job index is
compatible with the previous schema-2 binary. A collector rollback can restore
the previous binary without restoring or deleting event rows. Preserve the
separate log database: new clients will retain pending records while the old
binary returns 404. Do not discard diagnostic evidence to make a rollback look
healthy. Normal Studio release and installation procedures remain in AGENTS.md.

## Reproducible validation

```sh
go test ./internal/telemetry ./services/telemetry ./internal/httpapi ./internal/obs ./internal/workers ./cmd/zv-orchestrator -count=1
go test ./internal/editor -count=1 -timeout 15m
go vet ./...
pnpm --dir desktop run typecheck
pnpm --dir desktop run lint
pnpm --dir desktop run test:unit
pnpm --dir desktop run test:e2e:diagnostics
pnpm --dir desktop run test:e2e:diagnostics-ui
pnpm --dir web run lint
pnpm --dir web run test:unit
pnpm --dir web run build
```

The collector integration test builds and starts the real Go service, executes a
failing worker subprocess, uses production ProcessSession and DiagnosticLogClient,
loses a receipt after the real commit, restarts delivery, deletes `studio.log`,
and reconstructs the cause solely from collector queries. It writes
`.local/remote-debug-validation/collector-only-canary.json`. Separate actual
FFmpeg tests prove causal stderr and exit codes in both buffered and progress
paths without changing the original returned output.

The Electron UI suite needs built desktop resources (`build` and `assemble`)
and optionally `CLIPHUB_E2E_TOOL_FIXTURE` pointing at a verified tools fixture.
It runs the real app in a disposable profile with a test-only receipt transport;
it does not send UI fixtures to production. It verifies the diagnostic choice,
renderer cause, credential filtering, receipt status, revocation and layouts at
390x844, 1366x768 and 1920x1080. Screenshots are in
`desktop/e2e/artifacts/diagnostics/`. UI receipt fixtures and the real collector
integration are separate evidence; neither proves an installed production
version has been updated.

Validated on 2026-09-12 against Studio base 3.0.2 in an isolated checkout:
the complete editor suite passed in 375.899 seconds after live FFmpeg tracing;
all affected backend packages, desktop unit tests/typecheck/lint, real collector
integration and Electron diagnostic UI flow passed. The collector Linux amd64
artifact was additionally run on the actual VPS against a SQLite backup in an
isolated temporary directory: all 100 legacy rows remained intact, three log
records were committed, their exact retry inserted zero rows, and authenticated
pagination and redaction passed. That validation did not replace or restart the
production service. Runtime rollout remains a separate step from these results.
