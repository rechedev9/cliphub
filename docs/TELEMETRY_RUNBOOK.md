# Remote diagnostics runbook

ClipHub's collector is a separate Go service on `hetzner-openclaw`. It stores a
fixed, privacy-minimized event schema in SQLite. It is not part of the Windows
installer and does not expose the user's computer, files, or local API.

## Network boundary

| Surface | URL | Exposure |
| --- | --- | --- |
| Ingest | `https://hetzner-openclaw.taila10698.ts.net:8443/v1/ingest` | Public through Tailscale Funnel |
| Agent API | `https://hetzner-openclaw.taila10698.ts.net:10000/` | Tailnet-only through Tailscale Serve |
| Collector listeners | `127.0.0.1:8120`, `127.0.0.1:8121` | VPS loopback only |

Funnel exposes only the public handler (`healthz` and bounded ingest). It uses
TLS-terminated TCP plus PROXY protocol v2 so the collector can apply transient,
per-source limits. Source addresses are salted into process-memory keys and are
never written to SQLite or logs. The agent handler uses a different private
listener and requires a bearer token even from the tailnet. The public ingest
key is compiled into Studio and is therefore a routing/abuse filter, not a
secret.

Inspect the Tailscale routing without changing it:

```bash
ssh hetzner-openclaw 'tailscale serve status; tailscale funnel status'
```

## Agent queries

The operator token lives outside the repository at
`~/.config/cliphub/telemetry-agent.env` with mode `0600`. It must never be
printed, committed, copied into Studio, or included in an agent transcript.
The querying machine must be connected to the tailnet.

```bash
scripts/telemetry-query.sh health
scripts/telemetry-query.sh incident CH-ABCD-1234-5678-90AB-CDEF
scripts/telemetry-query.sh incident CH-ABCD-1234-5678-90AB-CDEF 100
scripts/telemetry-query.sh stats 24
scripts/telemetry-query.sh stats 168
```

Ask the user for the support code shown under **Configuración → Diagnósticos**.
Start with the incident query, correlate by release/session/time, then use stats
to determine whether the fingerprint or performance change affects other
installations.

## Data contract

All errors are sent after the informed choice. Performance spans are sampled at
10% before entering the durable local queue. Events contain only:

- random event and session UUIDs;
- pseudonymous `CH-XXXX-XXXX-XXXX-XXXX-XXXX` support code;
- app release, component, stage, class, operation and outcome labels;
- server-generated fingerprint derived only from the fixed labels;
- coarse OS/architecture and duration.

The schema has no arbitrary tags. The collector enforces exact, kind-specific
allowlists for every component, operation, stage, class, outcome, OS and
architecture combination. It rejects demos, media, Steam IDs, share codes,
paths, credentials, prompts, player names and job payloads. There is no
free-text field in the remote schema. The desktop journal importer ignores the
local error message, demo and target fields, and the collector never stores the
source IP.
Transport infrastructure can still process connection metadata operationally.
Because the PROXY listener is loopback TCP, `tailscaled` and root-owned local
processes are part of the trusted transport boundary; keep the VPS single-purpose
and do not grant untrusted shell or service access.

The public key is extractable from Studio and cannot authenticate an anonymous
installation. Abuse is bounded instead: 20 events/request, 30 requests/minute
and 500 new events/hour per transient source, 600 requests/minute and 5,000 new
events/hour globally, 64 KiB bodies, a 128 MiB SQLite page cap, a proactive
28,000-used-page high-water stop (free-list pages remain reusable), and a 16 MiB
WAL journal limit. Duplicate event IDs
do not consume the event budget. `stats` reports current event and storage
totals. Treat public labels as untrusted aggregate input. Rotate the ingest key
and ship a new release if targeted abuse exhausts shared availability.

Events are deleted by `received_at` after 30 days. Disabling diagnostics deletes
the unsent desktop queue immediately. No event is queued until the one-time
notice choice has been persisted.

## Service operations

```bash
ssh hetzner-openclaw 'systemctl status cliphub-telemetry --no-pager'
ssh hetzner-openclaw 'journalctl -u cliphub-telemetry --since today --no-pager'
ssh hetzner-openclaw 'systemctl restart cliphub-telemetry'
```

Runtime files:

```text
/opt/cliphub-telemetry/cliphub-telemetry
/etc/cliphub-telemetry/collector.env
/var/lib/cliphub-telemetry/telemetry.db
/etc/systemd/system/cliphub-telemetry.service
```

The service account owns only `/var/lib/cliphub-telemetry`; systemd binds both
HTTP listeners to loopback and applies filesystem/kernel hardening.

## Build and deploy

Build on Linux with the repository-pinned Go toolchain. Before the first
hardened deploy, inspect metadata only (never print rows): `PRAGMA page_size`
must be 4096. Schema version 0 is the pre-allowlist schema; startup deliberately
purges and recreates it so no legacy summary or content-derived fingerprint
survives. Do not back up that replaceable legacy diagnostic data.

Public transport and the collector must migrate to PROXY v2 together: first stop Funnel, then install
the binary **and tracked unit**, reload systemd, start the collector, and only
then restore Funnel.

```bash
ssh hetzner-openclaw "runuser -u cliphub-telemetry -- python3 -c \"import sqlite3; p='/var/lib/cliphub-telemetry/telemetry.db'; c=sqlite3.connect(p); print('page_size',c.execute('pragma page_size').fetchone()[0],'schema',c.execute('pragma user_version').fetchone()[0],'events',c.execute('select count(*) from telemetry_events').fetchone()[0]); c.close()\""
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
  -o /tmp/cliphub-telemetry ./services/telemetry
scp /tmp/cliphub-telemetry deploy/telemetry/cliphub-telemetry.service \
  hetzner-openclaw:/tmp/
ssh hetzner-openclaw '
  set -eu
  umask 077
  tailscale funnel status --json > /root/cliphub-funnel-before.json
  if tailscale funnel status | grep -q "TLS-terminated TCP"; then
    tailscale funnel --yes --tls-terminated-tcp=8443 off
  else
    tailscale funnel --yes --https=8443 off
  fi
  ! tailscale funnel status | grep -q 8443
  tailscale serve status | grep -q 10000
  cp -a /opt/cliphub-telemetry/cliphub-telemetry /opt/cliphub-telemetry/cliphub-telemetry.previous
  cp -a /etc/systemd/system/cliphub-telemetry.service /etc/systemd/system/cliphub-telemetry.service.previous
  install -o root -g root -m 0755 /tmp/cliphub-telemetry /opt/cliphub-telemetry/cliphub-telemetry
  install -o root -g root -m 0644 /tmp/cliphub-telemetry.service /etc/systemd/system/cliphub-telemetry.service
  rm /tmp/cliphub-telemetry /tmp/cliphub-telemetry.service
  systemctl daemon-reload
  systemctl restart cliphub-telemetry
  systemctl is-active --quiet cliphub-telemetry
  tailscale funnel --yes --bg --proxy-protocol=2 --tls-terminated-tcp=8443 tcp://127.0.0.1:8120
'
```

After every deploy, verify `systemctl cat/show` reflects the tracked unit,
`tailscale funnel status` shows PROXY v2 on 8443, public Funnel health works,
private authentication still fails closed, and a disposable ingest/query round
trip succeeds. Delete the disposable event afterward. If startup fails, leave
Funnel off, restore both `.previous` files, run `systemctl daemon-reload`, restart,
and restore the prior Funnel transport that matches the prior binary.

## Backup and restore

A raw copy of a live WAL database is not a valid backup procedure. Create a
consistent SQLite snapshot as the service user and then encrypt/copy that file:

```bash
ssh hetzner-openclaw \
  "runuser -u cliphub-telemetry -- python3 -c \"import sqlite3; \
  s=sqlite3.connect('/var/lib/cliphub-telemetry/telemetry.db'); \
  d=sqlite3.connect('/var/lib/cliphub-telemetry/telemetry-backup.db'); \
  s.backup(d); d.close(); s.close()\""
```

Backups must follow the same 30-day deletion policy. Restore only while the
service is stopped, preserve owner/mode `cliphub-telemetry:cliphub-telemetry
0600`, then start the service and run both health checks.

## Emergency controls

Stop public ingestion while keeping private Serve incident access:

```bash
ssh hetzner-openclaw 'tailscale funnel --yes --tls-terminated-tcp=8443 off && ! tailscale funnel status | grep -q 8443 && tailscale serve status | grep -q 10000'
```

Restore it:

```bash
ssh hetzner-openclaw \
  'tailscale funnel --yes --bg --proxy-protocol=2 --tls-terminated-tcp=8443 tcp://127.0.0.1:8120'
```

Do not expose the admin listener through Funnel or the existing public Caddy.
