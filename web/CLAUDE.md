# web/ frontend guidance

This file is loaded when working under `web/`; the repo-wide rules live in the root `CLAUDE.md`.

## Visual work

Before any CSS, component chrome, layout, or user-visible copy:

1. Read `~/.grok/design.md` (shared method + component sources).
2. Load the `frontend-design` skill (`.claude/skills/frontend-design/SKILL.md`).
3. Restyle onto tokens in `app/globals.css`. There is no product `design.md`.

Do not introduce a parallel token set.

## Web frontend (web/)

`web/` is a standalone Next.js 16 app (App Router, React 19, Tailwind 4): the no-login `/upload` entry, match/clip/video/stream/tactical views, and a typed API client under `web/lib/api`.
It is local-first and stateless: it talks only to the orchestrator (`zv serve`) through same-origin proxy route handlers under `web/app/api/demos/*`, which forward `.dem` uploads and job calls while keeping the orchestrator URL and token server-side.
`web/lib/api` always uses the real typed client; same-origin route handlers keep the orchestrator URL and token server-side.

Hosted mode (`CLIPHUB_WEB_MODE=hosted`) adds accounts and device pairing while
keeping the data plane local. A service worker forwards ordinary Studio API
requests to the authenticated loopback gateway; multipart demo/VOD uploads go
directly from Chrome to loopback because service-worker request streams require
HTTP/2, while the agent intentionally serves HTTP/1.1 on `127.0.0.1`. Account,
pairing, and installer routes remain on the hosted origin. Never proxy demos,
media, Steam credentials, job SQLite, or render artifacts through the VPS.

Finished Library reels expose a manual publication assistant through the per-artifact `/api/demos/*/publish-assistant` proxy. It generates Madrid-time guidance and factual reel-derived metadata, lets the user download the MP4, and opens only `https://studio.youtube.com/` in the system browser. Account, audience, visibility, scheduling, and the official upload flow remain entirely in YouTube Studio; ClipHub has no Google account connection or direct publishing path.
Library ready cards do not require a cover-candidate pick before MP4 download or PREPARAR PUBLICACIÓN; cover generation in the render pipeline is unchanged.

Run it locally with the standard pnpm scripts in `web/package.json` (`dev`, `typecheck`, `lint`, `test:unit`).
The dev server needs the orchestrator on `127.0.0.1:8080`; the desktop/local-studio path uses persistent SQLite plus the inline queue.

Proxy-route contract: every `/api/demos/*` route reaches the orchestrator through `callOrchestrator` (`web/app/api/demos/_lib.ts`).
When the orchestrator is unreachable the route returns `503 {code: "service_unavailable"}` and logs the cause server-side, and the UI tells "service offline" apart from a bad demo via `SERVICE_UNAVAILABLE_CODE`.
Keep that contract when adding `/api/demos/*` routes; do not let a bare `fetch` throw into a code-less 500.

Real `.dem` files are never committed, so the fixture stays local.

Share codes: `web/lib/sharecode.ts` validates SHAPE only — 25 characters over the 57-character dictionary. The base-57 decode lives in the Go package `internal/sharecode` and must never be ported here; two implementations of one bijection is how they start disagreeing.
`matchId` and `outcomeId` cross the `/api/steam/sharecode` boundary as **strings**, because they exceed 2^53 and `Number()` silently corrupts them. Keep them strings everywhere, tests included.
A `decoded` response is a success, not a failure: it means the code is valid but the Game Coordinator has not returned a demo URL. Download is `POST /api/steam/import` (same job pipeline as `/upload`). `409 steam_credentials_required` opens the login dialog; that password is not persisted. Only `service_unavailable` means the local service is down, and the UI must keep those two apart.
Ajustes stores the revocable authentication code, SteamID and Web API key through `/api/steam/account`. It never writes a Steam password.
`/full-demo` is the Full demo to video section: landscape recap, native HUD, team comms. It does not share the Shorts brief.

## TypeScript style (web/)

Applies to everything under `web/` (Next.js 16 App Router, React 19, Tailwind 4).
Adapted from the jvidalv/berrus agent guidelines.
Same priorities as the Go rules: clarity, simplicity, concision, maintainability, and repo consistency, in that order.

Full TypeScript, strict:

- The project is full TypeScript: no `.js`/`.jsx` source files, `strict: true` and `allowJs: false` stay on in `web/tsconfig.json`.
- `pnpm run typecheck` (`tsc --noEmit`) and `pnpm run lint` (oxlint) must pass before any change is considered done.
- Lint config lives in `web/.oxlintrc.json` (adapted from berrus).

Type safety:

- No `any`, ever: not explicit, not `as any`, not `<any>`, not `any[]`.
  If a type is genuinely unknown, use `unknown` and narrow it.
- No `!` non-null assertions.
  Handle the null case, or restructure so the type proves it.
- No `as <Type>` to silence the checker.
  A cast is acceptable only at a trust boundary where TypeScript cannot know the shape: `JSON.parse`, `await res.json()`, storage reads, `process.env`.
  Even there, cast to a named type (never `any`) and validate or narrow untrusted input before acting on it (see `lib/api/reel-store.ts` for the pattern).
- No `@ts-ignore`.
  `@ts-expect-error` only with a comment explaining the upstream cause.
- Exported functions declare explicit return types; local variables rely on inference.
- Keep exported APIs small; do not export a symbol only tests use.

Modules and imports:

- No re-exports: a module never re-exports a symbol it does not define.
  When moving code, update every import to the new location; do not leave "backwards compat" shims.
- Prefer direct file imports over barrels for heavy libraries.
  Exception: `lucide-react` uses barrel imports only (its direct paths lack type declarations).
- Shared API types live in `web/lib/api`; do not duplicate response shapes in components.

No magic strings:

- A string literal that crosses a boundary or repeats (an error code, a status value, a query param, a storage key) must be a named `const`, ideally an `as const` map with a derived union type, imported at every use site.
  `SERVICE_UNAVAILABLE_CODE` is the house example; inline duplicates of such strings are a review finding.

Comments:

- A comment is at most 2 lines, and a changed file carries at most 1 comment line per 5 code lines.
  The code is typed: delete comments that restate a name, a type, or the line below; keep only a non-obvious why.
- Keep comments concise and focused on non-obvious invariants; lint, typecheck, tests, and review are the enforcement mechanisms.

Async:

- Sequential `await` of independent operations is a performance bug; use `Promise.all`.
- Every `fetch` to the orchestrator goes through `callOrchestrator` (`web/app/api/demos/_lib.ts`) so failures map to `503 {code: "service_unavailable"}` instead of a code-less 500.

Server/client boundary:

- Secrets (orchestrator URL, tokens) stay server-side: route handlers and `server-only` modules.
  Never read them in a client component or ship them via `NEXT_PUBLIC_*`.
- Keep components server components unless they need state, effects, or browser APIs; add `"use client"` at the leaf, not the layout.

React:

- Derive types from data (`typeof x`, `as const` unions) instead of maintaining parallel enums.
- No `React.FC`; type props with an explicit interface or inline object type.
- Handle loading, error, and empty states explicitly in UI that fetches; keep response parsing and error mapping inside the typed `web/lib/api` boundary.

Testing:

- Unit tests are `lib/**/*.test.ts` on `node:test`, run with `pnpm run test:unit` (Node strips types natively; relative imports keep the `.ts` extension, allowed by `allowImportingTsExtensions`).
- Browser E2E is Playwright under `e2e/`, run with `pnpm run test:e2e` (the `playwright test` CLI). It verifies the presentation contract in `e2e/contract.ts` against `app/globals.css` — token ramps, the type scale, shell geometry, focus and target sizes, the `--shell-depth` gates, and zero horizontal overflow at the six validation widths — plus the `/upload` roster flow with the three `/api/demos/*` proxy calls stubbed at the network boundary.
- The suite drives a **production build** (`next build && next start`), not `next dev`: the dev server's HMR client never completes its handshake under Playwright and the app-router bootstrap stalls behind it, so React creates the root container and never attaches the tree — every interaction test would see server HTML with no handlers. Pass `E2E_SKIP_BUILD=1` to reuse an existing `.next`.
- Assert tokens through the parsers in `e2e/contract.ts`, never as literal strings: the production minifier rewrites `oklch(0.128 0.02 264)` to `oklch(12.8% .02 264)` and `380ms` to `.38s`, so a text comparison pins the minifier instead of the contract and passes in dev while failing in the build that ships.
- E2E needs a build and a server. Run it explicitly when the shell, tokens, or the upload flow change. Deeper integration coverage still lives in Go HTTP/worker tests and `scripts/smoke-real.ps1`.
- A test double for an external client (e.g. a fake `SupabaseClient`) types only the call surface it fakes and is cast once at creation with `as unknown as <ClientType>` plus a comment; that is the sole sanctioned use of a double cast.
- Bug fixes need a regression test, same as Go.
