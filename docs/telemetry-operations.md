# Telemetry operations

How the telemetry collector is deployed, watched and alerted on. This
repository is public: host names, addresses, tailnet names, env file
locations and tokens never go in it. This runbook says "the VPS",
"<admin env file>" (the collector's environment file) and "<alert env file>"
(the alerter's) instead; the real values live only on the VPS and in the
operator's password manager.

## Architecture

- **Collector** (`cliphub-telemetry.service`): long-running, ingests events
  and logs, exposes the tailnet-only admin API on a loopback port. It has no
  egress (`IPAddressDeny=any`) and never sends notifications.
- **Alerter** (`cliphub-telemetry alert`, run by
  `cliphub-telemetry-alert.timer` every minute): a oneshot of the same binary
  under its own system user. Each run, bounded to 45 s:
  1. reads admin `/healthz`, `/v1/errors` and `/v1/logs` since its cursors
     (a collector without `/v1/errors` answers 404; the alerter logs a warning
     and keeps working from `/v1/logs`);
  2. updates its own SQLite state (`<state dir>/alert.db`: issues, releases,
     cursors, sent alerts, outbox) in one transaction;
  3. sends pending alerts to Telegram and reads button presses. A transient
     failure (network, 5xx, 429, or a 401/403/404 bad token or blocked bot)
     keeps the alert and the ones after it for the next run, for up to 24
     hours. An alert Telegram refuses for good (any other 4xx, such as a
     text it cannot parse) is dropped, logged by rule and status only, and
     the run continues with the next one. Every text is cut to Telegram's
     4096 characters by dropping trailing lines behind "… N líneas más";
  4. renders the static report into `<state dir>/www`;
  5. pings healthchecks check A: success when the run was green, `/fail` with
     a short reason label otherwise (`healthz`, `errors`, `logs`, `telegram`,
     `telegram_updates`, `outbox`, `notifier`, `report`, `db_ok=false`,
     `dead_letter=<n>`, `config`, `run`).
- **Check B**: a scheduled GitHub Actions job curls the public `/healthz`
  and pings a second healthchecks check. Both checks notify the same Telegram
  chat, so a dead VPS, a dead alerter or a dead public ingest path all reach
  the phone.

`<state dir>` is the systemd `StateDirectory` of the alerter unit
(`/var/lib/cliphub-telemetry-alert`).

### Issues

An issue is `sha256(component|name|stage|class|code)[:16]`. `code` is the
`failure_code=` prefix of the message when present, otherwise
`unclassified:<signature>`, where the signature is the first error-looking
line with timestamps, paths, ids, versions and numbers replaced. The key does
not depend on the release, so "new", "regressed" and "resolved" work across
versions. A failed job arrives twice: the error event (`pipeline.error`) and
the log (`attempt.finished` with outcome `error`, or a `pipeline.error` log).
On current clients the two copies can have different labels and text, and the
log can arrive hours later when the client spool was deferred. The copies are
paired 1:1 per job: an occurrence is the paired copy of a failure already
counted when the job has more stored occurrences from the other stream than
from its own (within 24 hours of client time). A paired copy is stored but
creates or updates no issue, pages nothing and is left out of counts,
repeats, the digest and the report. A retry that fails differently is a new
failure, and an old client that only sends events counts every failure.

### Rules

| Rule | Priority | Fires when | Repeats at most |
|---|---|---|---|
| Nuevo | P1 | first occurrence of an issue key | once |
| Regresión | P1 | a resolved issue appears in a newer release than the one it was resolved in | once per key and release |
| Plataforma rota | P1 | an operation fails at least twice on a release with no success | once |
| Crash | P1 | a known issue from the crash family happens again (backend crash, uncaught exception, process gone with reason crashed/oom/launch-failed, previous-session crash, fatal runtime, HTTP panic, boot failure); shutdown kills are excluded | every 6 h |
| Reporte | P1 | a user sends "report a video" | every 10 min |
| Calidad | P1 | a Full Demo / Full POV delivery is below the floors (bitrate under 10 Mb/s at 1080p60, mean luma under 25, black ratio over 0.5) | every 6 h per installation and preset |
| Canal | P1 | new ingest rejections since the last run with status 400, 413, 415, 422, 429, 500, 503 or 507 (500/503 are store failures that lose events while `db_ok` stays true), storage at 80 % of a cap, or the collector DB ping fails. 401 does not page: internet scanners hit the public ingest without the key | every 24 h per cause |
| Repite | P2, silent | an open, not acknowledged issue keeps occurring | every hour, 6 h per issue |
| Entrega | P2, silent | a client reports lost, dropped or rejected log records (`delivery.gap`, `delivery.health`) | every 24 h per installation |
| Resumen diario | P2, silent | first run after 09:00 Europe/Madrid; includes the 401 ingest rejections since the previous digest | daily |
| Avalancha de alertas | P1 | 10 P1 alerts in the last hour; later P1s of that hour arrive silent | every hour |

Messages carry a hint with the matching AGENTS.md section when the failure
code is a known incident (HLAE vs CS2 build, loudnorm range, AAC recovery,
POV verification, CS2 already running, shutdown kill).

## One-time setup on the VPS

1. **System user.** The deploy script creates it; by hand:
   `useradd --system --no-create-home --shell /usr/sbin/nologin cliphub-telemetry-alert`.
2. **Telegram bot.** In Telegram, talk to `@BotFather`, `/newbot`, keep the
   token. Send any message to the new bot from the phone, then open
   `https://api.telegram.org/bot<token>/getUpdates` from a trusted machine and
   read `message.chat.id`. Only that chat is accepted for button presses.
3. **healthchecks.io.** Create two checks in one project and connect the
   project to the same Telegram chat:
   - check A (VPS and alerter): period 2 minutes, grace 10 minutes. Its ping
     URL goes into `CLIPHUB_ALERT_DEADMAN_URL`.
   - check B (public ingest path): period 15 minutes, grace 60 minutes, pinged
     by the GitHub probe (its ping URL is a repository secret of that
     workflow).
4. **Alert env file.** Create `<alert env file>` owned by `root`, mode `0640`
   (group `root`; systemd reads it before dropping privileges):

   ```sh
   CLIPHUB_ALERT_ADMIN_URL=http://127.0.0.1:<admin port>
   CLIPHUB_ALERT_ADMIN_TOKEN=<same value as CLIPHUB_TELEMETRY_ADMIN_TOKEN in <admin env file>>
   CLIPHUB_ALERT_TELEGRAM_TOKEN=<bot token>
   CLIPHUB_ALERT_TELEGRAM_CHAT=<numeric chat id>
   CLIPHUB_ALERT_DEADMAN_URL=<check A ping URL>
   CLIPHUB_ALERT_REPORT_BASE=https://<VPS tailnet name>/alerts
   # CLIPHUB_ALERT_INCLUDE_EXCERPT=false
   ```

   | Variable | Required | Meaning |
   |---|---|---|
   | `CLIPHUB_ALERT_ADMIN_URL` | yes | numeric loopback URL of the admin listener |
   | `CLIPHUB_ALERT_ADMIN_TOKEN` | yes | admin bearer token, at least 32 characters |
   | `CLIPHUB_ALERT_TELEGRAM_TOKEN`, `CLIPHUB_ALERT_TELEGRAM_CHAT` | yes, except for `--dry-run` and `--bootstrap` | bot token and numeric chat id |
   | `CLIPHUB_ALERT_DEADMAN_URL` | no (no dead-man without it) | healthchecks check A ping URL, https |
   | `CLIPHUB_ALERT_REPORT_BASE` | no (no links without it) | https base under which `<state dir>/www` is served |
   | `CLIPHUB_ALERT_INCLUDE_EXCERPT` | no, default `false` | see Privacy |
   | `CLIPHUB_ALERT_STATE_DIR` | no | defaults to the unit's `StateDirectory` |

5. **Attach the env file.** The unit in the repository does not name the
   file. Add a drop-in:

   ```sh
   systemctl edit cliphub-telemetry-alert.service
   # [Service]
   # EnvironmentFile=<alert env file>
   ```

   The deploy script enables the timer only once this drop-in exists.
6. **Serve the report tailnet-only**, next to the existing admin mapping:

   ```sh
   tailscale serve --bg --https=<admin serve port> --set-path=/alerts /var/lib/cliphub-telemetry-alert/www
   tailscale serve status
   ```

   Never use `tailscale funnel` for this path: the report contains support
   codes and filtered messages. If serving a directory is not available on the
   installed Tailscale version, leave `CLIPHUB_ALERT_REPORT_BASE` empty (alerts
   then carry no link) and read the report over ssh, or add a read-only static
   handler to the admin listener (it needs a `ReadOnlyPaths=` entry for the
   www directory in the collector unit).
7. **Deploy** (next section), then check the first run:
   `journalctl -u cliphub-telemetry-alert.service -n 20` must show
   `class=done bootstrap=true`.

## Bootstrap and dry run

The first run after the state is created is a bootstrap: it reads all
retained history, records every existing issue as known and sends nothing.
Normal alerting starts at the next run, so the first deploy does not page for
30 days of old errors. A bootstrap that runs out of time continues on the
next run and still sends nothing until it has caught up.

Run one by hand with the unit's user, state and env file:

```sh
systemd-run --wait --pty --collect \
  -p User=cliphub-telemetry-alert -p StateDirectory=cliphub-telemetry-alert \
  -p EnvironmentFile=<alert env file> \
  /opt/cliphub-telemetry/cliphub-telemetry alert --bootstrap
```

- `--bootstrap` resets the cursors and re-reads the retained history as known.
  Use it after restoring or deleting `alert.db`, and after restoring a
  collector database: the `/v1/errors` cursor is the events table's row id,
  which a restored copy does not have to preserve. Acknowledged, resolved and
  silenced issues keep their status.
- `--dry-run` (same command) prints the alerts the run would send and rolls
  the state back. It does not ping healthchecks or answer buttons. On an
  empty state it skips the implied bootstrap, so it previews everything
  retained as if it were new.

A normal run is `systemctl start cliphub-telemetry-alert.service`.

## Deploy and rollback

From a clean checkout on the dev PC:

```sh
printf 'DEPLOY_SSH_TARGET=<ssh alias of the VPS>\n' > .telemetry-deploy.local   # once, git-ignored
bash scripts/deploy-telemetry.sh
```

The script builds `linux/amd64` with `-X main.version=$(git describe --always --dirty)`,
uploads it with a checksum, stops the collector, copies its database files to
`/var/backups/cliphub-telemetry/predeploy-<utc stamp>` (the three newest are
kept), keeps `cliphub-telemetry.previous` and the previous collector unit,
swaps the binary atomically, restarts, and polls the admin `/healthz` until
it reports the new `version` with `db_ok: true`. On timeout it restores the
previous binary and unit, restarts and exits 1. Finally it installs the
alerter units and enables the timer when the env drop-in exists. The ssh user
must be root or have passwordless sudo; nothing about the host is printed.

- Prove the rollback path on the first deploy with
  `bash scripts/deploy-telemetry.sh --force-rollback`: it deploys, fails the
  health check on purpose and must end on the previous binary.
- The version check needs a collector that reports `version` and `db_ok` in
  the admin `/healthz`. Deploying over an older collector is fine; deploying a
  build without those fields always rolls back.
- Manual rollback on the VPS: stop the collector, copy
  `/opt/cliphub-telemetry/cliphub-telemetry.previous` over the binary (and
  `cliphub-telemetry.service.previous` over the unit), `systemctl daemon-reload`,
  start it. The alerter uses the same binary; its timer picks the swap up on
  the next run. `alert.db` is independent of the collector databases.
- Restoring a pre-deploy copy: stop the collector, copy the files back with
  the collector's owner and mode `0600`, start it.

## Rotating secrets

The alerter reads `<alert env file>` on every run, so no restart is needed on
its side.

- **Admin token:** generate one (`openssl rand -hex 32`), write it to
  `CLIPHUB_TELEMETRY_ADMIN_TOKEN` in `<admin env file>` and to
  `CLIPHUB_ALERT_ADMIN_TOKEN` in `<alert env file>`, then
  `systemctl restart cliphub-telemetry.service`. Update the local copy used by
  `scripts/telemetry-query.sh`. Runs in between fail with `errors`/`healthz`
  and ping check A `/fail`; that is expected for a minute.
- **Telegram bot token:** `@BotFather` › `/revoke`, put the new token in
  `<alert env file>`. Unsent alerts wait in the outbox for up to 24 hours.
- **healthchecks ping URLs:** regenerate in the check settings and update
  `<alert env file>` (check A) or the workflow secret (check B).

## Reading alerts and the report

An alert looks like
`[P1] Nuevo · record:demo/hook_start · hlae_hook_incompatible · 4.0.1 · #1 · ×1`
(operation and substage, failure code, release, installation alias, count),
followed by the first 8 characters of the job id, the machine line (CS2 build,
HLAE, encoder, GPU vendor), a hint and a link to the issue page. Silent (P2)
messages arrive without sound.

Buttons under issue alerts:

- **Ack**: seen; stops the hourly "Repite" reminders for that issue.
- **Resolver**: marks the issue resolved in the newest release seen. A later
  occurrence on a newer release fires "Regresión".
- **Silenciar 24h**: no alerts for that issue for 24 hours.

The report (`<report base>/index.html`, refreshed every minute) shows the
collector health and ingest rejections, the issues with status, attempts per
release and operation, user reports with the command to rebuild them, render
profile changes and CS2 builds, and the latest errors. An issue page is
rewritten only when its issue changes (occurrence, status, button press,
retention, a newer successful attempt of its operation), so its "generado"
time is that of its last change. Each issue page lists
its releases, the jobs with the `node scripts/telemetry-debug.mjs --job <id>`
command to rebuild each one, the last successful attempt of the same
operation, and its timeline. To go further, use the queries in
[remote-error-diagnostics.md](remote-error-diagnostics.md) and
[remote-debug-tracing.md](remote-debug-tracing.md).

## Privacy

- Telegram only receives labels, failure codes, releases, counts,
  installation aliases, a job id prefix and a report link. Support codes,
  session ids and message text stay on the VPS. Every free-form value is
  restricted to a short safe character set before it is sent.
- `CLIPHUB_ALERT_INCLUDE_EXCERPT=true` adds up to 300 characters of the
  filtered message to Telegram. That moves user-derived text to a third
  party: treat turning it on as a privacy policy change, not a tuning knob.
- The report is tailnet-only and keeps the same 30-day retention as the
  collector; sample messages in `alert.db` are cleared after 30 days.
  Issue keys are hashes of labels and normalized text and are only used on
  the VPS and in the report.
