---
paths:
  - "web/**"
  - "landing/**"
  - "desktop/src/**"
---

# Web inside Electron

`web/` is the Studio UI. It runs only in the Chromium that Electron bundles (`electron` in `desktop/package.json`). Use any web platform feature that Chromium ships, with no fallback and no polyfill.

## Local API

`desktop/src/main.ts` sets `ORCHESTRATOR_URL` and `PORT` and loads the UI from the web server it spawned. Child output is prefixed with the process label (`desktop/src/process-session.ts`), so the Next server's `- Local:` line appears as `[web] - Local:` in the Electron output. Drive the UI against that URL.

`web/app/api/*` proxies to the orchestrator. `web/proxy.ts` and `web/lib/api/local-request-guard.ts` reject a cross-origin request and a DNS-rebound host. Mutations also require the HttpOnly per-launch capability cookie (`cliphub_proxy_capability`), seeded by the desktop process. The browser pane cannot import a demo: its POSTs lack that cookie. Import through `bin\zv-mcp.exe` with `CLIPHUB_STUDIO_USER_DATA` set to the throwaway profile (`import_demo`, then `wait_for_demo`). The guard's tests are `web/lib/api/local-request-guard.test.ts`.

## Landing

`landing/` is a public site, separate from the installed app. Baseline Widely available features need no fallback. A newer feature needs feature detection and a fallback of at most 20 lines, with no new dependency.

## React attributes

Type a newly shipped HTML attribute by augmenting React's attribute types in a `.d.ts`. This replaces the `as any` spread that the `modern-web-guidance` skill suggests.

The manual UI check is [Verify in Studio](../skills/verify-in-studio/SKILL.md).
