# ClipHub hosted control plane

This stack hosts account authentication, the device registry, and the Next.js
interface. Demo files, Steam credentials, SQLite job state, captures, and
rendered media never enter these containers; Chrome sends Studio API traffic
to the installed loopback gateway through the ClipHub service worker.

## Isolation

- Compose project: `cliphub`
- Public bind: `127.0.0.1:8091` only
- Private control service: `control:8090`
- Persistent data: the dedicated `cliphub_control-data` volume
- No dependency on the Gravity Room or Fragbot networks, databases, or volumes

## Deploy

1. Point `cliphub.gravityroom.app` A/AAAA records at the VPS.
2. Copy this directory and the repository source to an isolated checkout.
3. Copy `.env.example` to `.env` and verify the public origin.
4. Run `docker compose build` and `docker compose up -d` from this directory.
5. Add `Caddyfile.snippet` to the host Caddy configuration, validate it, and
   reload Caddy.
6. Verify `https://cliphub.gravityroom.app/`, `/healthz` through the control
   container, registration, login, device pairing, and local-agent checks.

Do not place FACEIT, Steam, or media credentials in this stack. The account
database contains password hashes, session hashes, device-secret hashes, and
minimal device metadata, so include its dedicated volume in encrypted backups.
