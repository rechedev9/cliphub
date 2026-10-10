# ClipHub Portal deployment

The portal behind ClipHub cloud capture. It holds accounts, the job queue,
the demos waiting for the capture worker and the finished videos until Studio
has downloaded them. Capture and render (HLAE, CS2, Steam, FFmpeg) happen on
the capture worker, a Windows machine that reaches this stack over outbound
HTTPS only. No inbound connection is ever made to that machine.

## Isolation

- Compose project: `cliphub-portal`
- Public bind: `127.0.0.1:8092` only; the host's Caddy terminates TLS
- Persistent data: `cliphub-portal_portal-data` (SQLite) and
  `cliphub-portal_portal-uploads` (demos and result videos)
- One outbound-only network, shared with nothing. No dependency on the
  Gravity Room or Fragbot stacks, databases, or volumes.

## Deploy

1. Point `cliphub.gravityroom.app` A/AAAA records at the VPS.
2. Copy this directory and `portal/` to the VPS (the build context is
   `../../portal`, so keep them side by side).
3. `cp .env.example .env` and fill in every value.
4. `docker compose build && docker compose up -d`
5. Paste `Caddyfile.snippet` into the host Caddyfile, validate, reload.
6. Sign in once, run `pnpm whoami` against the database to get your
   `provider:providerAccountId`, put it in `ADMIN_ACCOUNTS`, and
   `docker compose up -d` again so `/admin` opens for you.
7. Create the capture worker (next section).

Migrations run automatically on container boot, so a fresh volume needs no
manual step.

## The capture worker

Worker credentials live in the database, not in `.env`.

1. Open `/admin`, go to Workers and create one. The token is shown once; only
   its hash is stored.
2. On the capture machine, set two user environment
   variables for the Windows account that runs Studio:
   `ZV_BRIDGE_URL=https://cliphub.gravityroom.app` and `ZV_BRIDGE_TOKEN=<token>`.
3. Restart Studio. Within 15 seconds the worker shows as online in `/admin`.

The rest of the machine setup (auto-logon, Steam and Studio at startup, what
each pause reason means) is in `docs/cloud-worker.md`.

To rotate the token, create a second worker, change `ZV_BRIDGE_TOKEN`, restart
Studio and revoke the old worker. A revoked worker gets 401 on every call and
the job it held goes back to the queue when its lease expires (90 seconds).

## Who can use the cloud

`CLOUD_ACCESS_DEFAULT=pending` (the default) means a new account can link
Studio but cannot create jobs until an admin allows it in `/admin` under
Usuarios. Set it to `allowed` to open the cloud to every account. Admins are
always allowed. Limits (3 active jobs and 90 minutes of machine time per 24
hours) come from the environment and can be overridden per user in the panel.

## Upgrading from the manual bridge

- Migration 0003 runs on boot. Existing requests are kept as kind `manual`;
  they stay readable in the panel and an admin can cancel them.
- Remove `BRIDGE_SHARED_SECRET` and `MAX_DAILY_REQUESTS_PER_USER` from `.env`:
  nothing reads them any more. The old shared secret no longer authenticates;
  create a worker as described above.
- The capture machine needs a Studio build that speaks the new worker
  protocol. An old build gets 404 from the routes that were removed.

## Upgrading to the fenced worker protocol

- Migration 0004 runs on boot. It only adds three nullable or defaulted
  columns, so it applies over a live 0003 database with its rows in place.
- Update the capture machine's Studio together with the portal. Every job
  route now needs the `X-ClipHub-Attempt` header and every heartbeat job entry
  its `attempt`; a worker build from before this gets 400 on the job routes
  and `lost` for every job in its heartbeat, so it can claim but not finish.
- The worker reports the kill plan version of its Studio. A user whose Studio
  produces a different one is refused at submit time with
  `studio_version_mismatch` instead of failing after the capture, so update
  the worker first and tell users to update when that code shows up.

## Storage

The uploads volume turns over quickly, but size it for the worst hour:

- A demo is at most 700 MiB (typically 100 to 300 MB). It is stored once per
  user and content hash, kept while a job needs it, and for 48 hours after a
  failure so the job can be retried from the panel.
- A Short is tens of megabytes; the cap is 1 GiB per file. A result is deleted
  24 hours after Studio confirms it has the file, or 7 days after upload.
- With the queue cap of 80 jobs, plan for 60 GiB of free space on the volume.
  When Full Demo is enabled (files of several gigabytes), revisit this.

Below `MIN_FREE_BYTES` (10 GiB) of free space the portal refuses new jobs and
demo uploads instead of filling the disk: Studio tells users the cloud is full
and the panel shows the storage card in red. Demo uploads in flight count as
already written, a job takes one upload at a time and a user two. The worker
keeps claiming down to `MIN_FREE_BYTES_CLAIM` (2 GiB), because finished jobs
are what free the demos. Check with `df -h` on the volume or in the panel's
storage card.

One user can hold at most `MAX_DEMO_BYTES_PER_USER` (3 GiB) of distinct demos
at a time; past that Studio is told `limit_storage`. A job its user cancels
before it ever ran gives its demo back at once.

## Backup

Back up `/data/portal.db`. It holds accounts, sessions, token hashes, jobs and
the audit trail, so keep the copy encrypted. The uploads volume does not need
a backup: everything in it is transient.

The database runs in WAL mode, so do not copy the file while the container is
running. Take a consistent snapshot with SQLite's own backup command:

```sh
docker run --rm \
  -v cliphub-portal_portal-data:/data \
  -v "$PWD":/backup \
  alpine sh -c "apk add --no-cache sqlite >/dev/null && \
    sqlite3 /data/portal.db \".backup '/backup/portal-$(date +%F).db'\""
```

To restore, stop the stack, replace `/data/portal.db` with the snapshot,
delete `portal.db-wal` and `portal.db-shm` if they exist, and start it again.

## Proxy limits

`Caddyfile.snippet` is unchanged: a 1200 MB body limit and 30 minute read and
write timeouts.

- Results never come close to either: the worker sends each file in parts of
  32 MiB and resumes after a failure, and downloads support `Range`.
- The demo upload is the one long request (up to 700 MiB in a single PUT). At
  the 30 minute timeout that needs about 3.3 Mbit/s of uplink from the user;
  Studio retries a failed upload.
- Device linking is rate limited per client address, read from the
  `X-Forwarded-For` header Caddy sets. Keep the portal reachable only through
  Caddy (the loopback bind above), or every client shares one bucket.
- The same address decides how a link is confirmed. When the browser that
  opens `/link` is at the address Studio asked from, one click approves it.
  From any other address the page warns in red, hides the code and makes the
  user type the code Studio shows. Without the header nothing counts as the
  same address, so every link needs the typed code.

## What must not go in here

FACEIT, Steam or media credentials. This stack never renders anything and has
no use for them.
