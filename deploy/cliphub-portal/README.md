# ClipHub Portal deployment

The public submission portal. It holds accounts, the request queue, and the
uploaded demos waiting to be claimed — nothing else. Rendering, HLAE, CS2,
Steam credentials and the finished media all stay on the owner's Windows PC,
which reaches this stack over outbound HTTPS only. No inbound connection is
ever made to that machine.

## Isolation

- Compose project: `cliphub-portal`
- Public bind: `127.0.0.1:8092` only; the host's Caddy terminates TLS
- Persistent data: `cliphub-portal_portal-data` (SQLite) and
  `cliphub-portal_portal-uploads` (demos and delivered videos)
- One outbound-only network, shared with nothing. No dependency on the
  Gravity Room or Fragbot stacks, databases, or volumes.

The database holds account rows, session hashes and request metadata, so
include its volume in encrypted backups.

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
7. On the Windows PC, set `ZV_BRIDGE_URL=https://cliphub.gravityroom.app` and
   `ZV_BRIDGE_TOKEN` to the same value as `BRIDGE_SHARED_SECRET`.

Migrations run automatically on container boot, so a fresh volume needs no
manual step.

## What must not go in here

FACEIT, Steam or media credentials. This stack never renders anything and has
no use for them.
