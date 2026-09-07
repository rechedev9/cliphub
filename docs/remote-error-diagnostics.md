# Remote error diagnostics

Studio 2.4.60 sends a filtered diagnostic message and the job UUID with newly
observed pipeline errors. Previously `pipeline.error` retained only labels such
as `record:demo`, so the collector could count failures but could not explain
them. Existing events without a message cannot recover information that was
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
