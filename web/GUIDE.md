# ClipHub Web UI Guide

`web/` is the Vite and React 19 static SPA shipped inside ClipHub Studio. The
local Go orchestrator serves both the compiled UI and its API on one loopback
origin; it is not a hosted web application.

```text
Electron renderer
  -> Go Studio origin (static files + /api)
  -> parse, HLAE/CS2 capture, render, and local artifacts
```

The browser never receives an orchestrator token. Internal clients authenticate
with `X-ClipHub-Token`; the UI uses the per-boot, HttpOnly, SameSite=Strict
`cliphub_ui_capability` cookie. Electron seeds it before navigation. The
standalone browser launcher authorizes through `/bootstrap#<capability>`; the
fragment is removed before a bounded same-origin form POST creates the cookie.

The orchestrator is the source of truth for jobs and artifacts. The client
stores only lightweight reel intent and shell preferences in browser storage.

## Development

The supported launcher builds the SPA, starts the SQLite-backed orchestrator,
and opens the upload flow:

```powershell
.\scripts\build.ps1
.\scripts\local-studio.ps1
```

For frontend-only work, start the orchestrator on `127.0.0.1:8080`, then run
`pnpm run dev`; Vite proxies local `/api` requests during development only.

```powershell
pnpm run lint
pnpm run typecheck
pnpm run test:unit
pnpm run build
pnpm run test:e2e
```

`pnpm run build` writes hashed static assets to `dist/`. Desktop assembly copies
that directory verbatim and rejects a bundle without `index.html` or the Vite
manifest. Fonts are bundled locally; production has no Node server dependency.

## Layout

```text
web/
  index.html                  # static document
  vite.config.ts              # build, aliases, and development proxy
  src/main.tsx                # router and application shell
  src/compat/                 # typed navigation adapters
  app/                        # route components and presentation tokens
  components/                 # UI, shell, and domain components
  lib/api/                    # typed same-origin clients and contracts
```
