---
paths:
  - "web/**"
  - "landing/**"
  - "desktop/src/main.ts"
  - "desktop/src/boot-security.ts"
---

# Web inside Electron

`web/` is the Studio UI. It runs only in the Chromium that Electron bundles (`electron` in `desktop/package.json`). Use any web platform feature that Chromium ships, with no fallback and no polyfill.

## Local API

`web/app/api/*` proxies to the orchestrator. `web/proxy.ts` and `web/lib/api/local-request-guard.ts` reject a cross-origin request and a DNS-rebound host. Mutations also need the per-launch HttpOnly capability cookie that the desktop process seeds (`installProxyCapabilityCookie` in `desktop/src/boot-security.ts`).

Do not loosen the guard or the cookie to make a test or the browser pane work. Drive state changes through `bin\zv-mcp.exe`, as in [Verify in Studio](../skills/verify-in-studio/SKILL.md). Tests: `web/lib/api/local-request-guard.test.ts`, `desktop/src/boot-security.test.ts`.

## Landing

`landing/` is a public site, separate from the installed app. Baseline Widely available features need no fallback. A newer feature needs feature detection and a fallback of at most 20 lines, with no new dependency.

## React attributes

Type a newly shipped HTML attribute by augmenting React's attribute types in a `.d.ts`. This replaces the `as any` spread that the `modern-web-guidance` skill suggests.
