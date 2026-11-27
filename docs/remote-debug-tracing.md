# Remote debugging and trace delivery

Studio sends two kinds of diagnostics to the telemetry collector (`services/telemetry`, on the
VPS) while Diagnósticos is on in Ajustes: schema-2 error events and the filtered log channel. The
goal is to rebuild a failing job from the collector alone: the first subprocess failure, the
output around it, its job and attempt, and what evidence is missing. A generic `exit status 1`
is not enough.

## Invariants

- Released clients keep working: the error and performance event API does not change shape.
- Nothing logs command arguments, environment variables, media contents or task payloads.
  Technical text is filtered before it enters the upload spool and again at the collector.
- The local `studio.log` and the user's diagnostic choice stay as they are; upload never
  replaces them.
- Every task attempt records its start, finish, outcome and duration, without sampling.

## Error events (schema 2)

Each newly observed pipeline error carries a filtered `message` (at most 2 KiB, keeping the start
and the final cause) and the `job_id`; both are optional on the wire, so older Studio versions stay
compatible. The filter replaces known credentials, paths, URLs, emails, SteamIDs, IPv4 addresses
and media filenames, in the desktop queue and again before storage; events keep the 30-day
retention and the queue revocation of the diagnostic choice. Updater failures use `component=electron`, `name=update.failed`, with the failed step
(`check`, `checksum`, `download`, `verify`, `apply`) as the class. Query an installation by its
support code:

```sh
bash scripts/telemetry-query.sh incident CH-XXXX-XXXX-XXXX-XXXX-XXXX 50
```

Aggregate stats group by stable codes, never by message text.

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
healthy. A change on the VPS needs the owner's approval first
([ClipMind and the VPS](../.claude/rules/clipmind.md)); the Studio release steps are in
[Release](../.claude/rules/release.md).

## Tests

```sh
go test ./internal/telemetry ./services/telemetry ./internal/httpapi ./internal/obs ./internal/workers ./cmd/zv-orchestrator -count=1
go test ./internal/editor -count=1 -timeout 15m
go vet ./...
pnpm --dir desktop run typecheck
pnpm --dir desktop run lint
pnpm --dir desktop run test:unit
node --no-warnings --test desktop/src/remote-debug.test.mjs
pnpm --dir web run lint
pnpm --dir web run test:unit
pnpm --dir web run build
```

The collector integration test builds and starts the real Go service, executes a
failing worker subprocess, uses production ProcessSession and DiagnosticLogClient,
loses a receipt after the real commit, restarts delivery, deletes `studio.log`,
and reconstructs the cause solely from collector queries. Separate FFmpeg tests
prove causal stderr and exit codes in both buffered and progress paths without
changing the original returned output.
