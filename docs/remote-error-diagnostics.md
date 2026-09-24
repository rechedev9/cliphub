# Remote error diagnostics

This document describes the existing schema-2 error-event channel. The new
[remote debugging trace channel](remote-debug-tracing.md) additionally uploads
filtered Studio output with durable receipts, per-attempt correlation and
explicit delivery gaps. Its 16 KiB chunked records are separate from the 2 KiB
legacy messages below; neither old clients nor old events acquire missing logs
retroactively.

Studio 2.4.60 sends a filtered diagnostic message and the job UUID with newly
observed pipeline errors. Previously `pipeline.error` retained only labels such
as `record:demo`, so the collector could count failures but could not explain
them. Recorder journal entries also retain the underlying subprocess output
alongside the short reason displayed in Studio. Existing events without a message cannot recover information that was
never uploaded; reproduce the failure after updating to capture its cause.

Updater failures now use `component=electron`, `name=update.failed`,
`stage=update`. Their class identifies the failed step: `check`, `checksum`,
`download`, `verify`, or `apply`. Quiet background checks also report failures.
The message retains the exception chain and network error code when available.
The original error remains in the local `studio.log` and is unaffected if
diagnostic delivery fails. `apply` covers launching the installer; failures
inside an installer after Studio exits still require installer diagnostics.

Messages are filtered before entering the desktop queue and again before
storage. Known credentials, paths, URLs, emails, SteamIDs, IPv4 addresses and media
filenames are replaced. Messages have a 2 KiB UTF-8 limit and preserve both the
beginning and final cause when truncated. This is filtered technical text,
not an upload of complete log files or media. The existing diagnostic choice,
pre-notice cursor boundary, queue revocation and 30-day retention still apply.

The wire contract remains compatible with older Studio versions: `message` and
`job_id` are optional error fields. Collector database schema 2 adds
`diagnostic_message` and `job_id` without deleting existing rows. Deploy the new
collector **before** publishing the desktop release; the old collector rejects
unknown fields. Back up the database with SQLite's backup API and the old
binary before migration. Rolling back to the old binary also requires restoring
the version-1 database backup while the service is stopped.

Query an installation using its support code from Studio settings:

```sh
bash scripts/telemetry-query.sh incident CH-XXXX-XXXX-XXXX-XXXX-XXXX 50
```

Inspect `message` and correlate `job_id`, `session_id`, release and event time.
Aggregate stats deliberately keep grouping by stable codes, not message text.

Validation covers shared client/server redaction fixtures, long UTF-8 and JSON
batches, updater failures in every phase, local journal import, the authenticated
ingest-to-query path, and preservation of old rows across migration and restart.

## Failure codes, grouping and alerts (Studio 5.3.0)

Go errors that end a job now start with
`failure_code=<code> substage=<substage>; ` (see `internal/obs/failure_code.go`).
The prefix survives both redaction filters and travels in the existing
`message` fields, so the wire does not change. Exec failures read
`<tool label>: exit status N: <last stderr line>` instead of a redacted
executable path.

The alerter on the VPS (see [telemetry-operations.md](telemetry-operations.md))
groups errors into issues: by failure code when there is one, otherwise by a
normalized signature of the already-filtered message. The key is computed and
kept only on the VPS. Alerts that leave the VPS go to Telegram and carry
labels, failure codes, releases, counts, an installation alias (`#1`), a job id
prefix and a tailnet-only link; never the support code or message text.

New log records, all in the existing log channel (`key=value` messages,
versions written with a `v` prefix so the address filter leaves them intact):

| Event | Content |
|---|---|
| `device.context` | once per session: Windows build and edition, GPU vendor/device/driver, CPU cores and model, RAM and free-disk buckets, FFmpeg and HLAE versions, `aac_mf` availability |
| `attempt.toolchain` | per capture: CS2 build and patch, HLAE pin, capture encoder |
| `cs2.console_tail` | last CS2 console lines when a capture fails |
| `delivery.quality` | per delivery: preset, size, fps, bitrate, mean luma, black ratio |
| `render.profile` | per successful render: source kind, overlay source, HUD, encoder, AAC path, tail pads |
| `stage.entered` | weighted pipeline stage breadcrumbs (audio master attempt and TP target) |
| `user.report` | "Reportar un problema con este vídeo": category only, no free text |
| `desktop.backend_crashed`, `process.gone`, `renderer.unresponsive`, `session.crashed_previous`, `runtime.fatal_previous` | crash metadata; Windows logoff kills are marked `exit_class=shutdown_kill` |

The collector counts every ingest rejection by channel, status and code and
reports them in the admin `/healthz` together with its build version, database
health and storage ratios; the alerter pages when a new rejection appears.
It also keeps 7 daily `VACUUM INTO` backups of both databases next to them, so
a record can outlive the 30-day retention by up to 7 days in a backup.

### Sending a diagnostic from the boot error screen

A Studio that fails to start never reaches the diagnostic notice, so its
failure used to be invisible. The boot error screen now offers
"Enviar este diagnóstico". The screen states what is sent (the error, the
filtered log tail and the device summary) and that sending turns diagnostics
on. The click is the same choice as accepting the notice; nothing is sent
before it. For a user who already accepted, the button only flushes the
pending queue.
