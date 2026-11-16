---
paths:
  - "web/**"
  - "landing/**"
  - "desktop/src/main.ts"
  - "desktop/src/boot-security.ts"
---

# Web inside Electron

`web/` is the UI of both editions. The desktop edition runs only in the Chromium that Electron bundles (`electron` in `desktop/package.json`), so it can use any web platform feature that Chromium ships. The web edition (ClipMind) runs in the user's own browser: a feature it renders follows the Landing rule below. [ClipMind and the VPS](clipmind.md).

## Local API

In the desktop edition `web/app/api/*` proxies to the orchestrator. `web/proxy.ts` and `web/lib/api/local-request-guard.ts` reject a cross-origin request and a DNS-rebound host. Mutations also need the per-launch HttpOnly capability cookie that the desktop process seeds (`installProxyCapabilityCookie` in `desktop/src/boot-security.ts`).

Do not loosen the guard or the cookie to make a test or the browser pane work. Drive state changes through `bin\zv-mcp.exe`, as in [Verify in Studio](../skills/verify-in-studio/SKILL.md). Tests: `web/lib/api/local-request-guard.test.ts`, `desktop/src/boot-security.test.ts`.

## Landing

`landing/` is a public site, separate from the installed app. Baseline Widely available features need no fallback. A newer feature needs feature detection and a fallback of at most 20 lines, with no new dependency.

## React attributes

Type a newly shipped HTML attribute by augmenting React's attribute types in a `.d.ts`. This replaces the `as any` spread that the `modern-web-guidance` skill suggests.
