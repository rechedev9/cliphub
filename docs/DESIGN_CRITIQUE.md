# ClipHub Studio — UX design critique (Shorts 9:16 + Full Demo 16:9)

Analysis only. This document is the deliverable: a ranked, file-referenced review of what in the
current Studio design hurts the two user flows, and the smallest UX changes that would make a
creator go from `.dem` to a YouTube-ready 16:9 (and still forge Shorts) without getting lost.
No component was restyled in this PR.

## Scope and method

Read before judging: `web/CLAUDE.md`, `web/app/globals.css`, `web/lib/full-demo.ts`,
`web/lib/full-demo.test.ts`, the `web/app/(app)/full-demo` route and its components
(`web/components/full-demo/demo-picker.tsx`, `web/components/full-demo/capture-bar.tsx`), the
upload surface (`web/app/upload/page.tsx`, `web/components/upload/*`), Partidas
(`web/app/(app)/matches/**`, `web/components/matches/*`), the Shorts constructor
(`web/app/(app)/matches/[id]/page.tsx`, `web/components/clips/*`), the Library
(`web/app/(app)/videos/page.tsx`, `web/components/videos/*`), Editor, Feed, onboarding
(`web/app/(app)/onboarding/**`, `web/components/onboarding/*`), the shell
(`web/components/shell/app-sidebar.tsx`, `web/lib/nav.ts`), and the API layer that feeds these
surfaces (`web/lib/api/real.ts`, `web/lib/api/jobs-index.ts`, `web/lib/reel-brief.ts`,
`web/lib/demo-parse-flow.ts`), plus the orchestrator's recap-plan handler
(`internal/httpapi/handlers.go`, `GetRecapPlan`).

The visual target for Full Demo is the imfcnd-style 16:9 reference: FACEIT roster overlay at the
open, live native HUD gameplay in the body, FACEIT scoreboard at the close. That target is the
destination for `/full-demo` output; it is **not** a prompt to restyle the Shorts flow, and
nothing below proposes new tokens, a design system, or a visual rewrite.

## Locked Full Demo contract (constraints on every fix below)

Nothing in this critique may be implemented in a way that breaks the contract in
`web/lib/full-demo.ts`, pinned by `web/lib/full-demo.test.ts`:

- `format: 'landscape-16x9'`, `matchRecap: true`, `nativeHud: true` (gameplay HUD),
  `voiceComms: true` at `voiceVolume: 0.85`, no music, no Shorts garnish
  (`killEffect: 'clean'`, `transition: 'cut'`, no intro/outro/hook/counter).
- Recap-plan failure copy stays distinct from the 404 empty state: `FULL_DEMO_RECAP_ERROR`
  must never read as `FULL_DEMO_EMPTY.missing` ("Demo no encontrada"), and a plan 500 must
  never be painted as "gone from disk".
- `reelIdentity` keeps `__full-demo` distinct from a Shorts reel identity.

## What already works — do not regress while fixing

- The three-way failure taxonomy on `/full-demo/[id]` (offline 503 / load error / missing 404)
  is correct, tested, and better than most of the app: `classifyFullDemoLoadFailure` +
  `fullDemoEmptyState` with `FULL_DEMO_EMPTY` keep "Demo no encontrada" reserved for a real 404.
- The brief-approval gate (explicit checkbox before INICIAR CAPTURA / FORJAR) exists on both
  flows and is revoked on any change in the Shorts constructor. Keep it.
- `PlayerPicker` already understands `purpose='full-demo'` (different recommendation heuristic,
  "Mejor rendimiento" badge, "REVISAR CAPTURA" CTA). The seam for flow identity exists; it is
  just entered too late (FLOW-2).
- The Library's stale-data strip (`StaleDataNotice`) and first-load failure state are honest:
  data on screen is never silently frozen.
- `CaptureReadiness` in the sidebar footer is the right always-on answer to "why did capture
  never start".

---

## Ranked findings

Severity: **BLOCKER** = the UI lies to the user or can break the locked contract;
**FLOW** = the user gets lost, blocked, or routed into the wrong product;
**NIT** = polish, naming, consistency.

### BLOCKER-1 — Pending parse is painted as "no highlights in this match"

- **Where:** `web/app/(app)/matches/[id]/page.tsx` (the `n === 0` empty state) fed by
  `findClips` in `web/lib/api/real.ts` and `ROSTER_READY` in `web/lib/api/jobs-index.ts`.
- **User moment:** Partidas lists any job whose status is in `ROSTER_READY`, which includes
  `'scanned'` and `'parsing'`. The row for a still-parsing demo is visually identical to a
  parsed one (no status tag; `match-row.tsx` renders the same VER PARTIDA CTA). The user opens
  it; `findClips` returns `[]` because the status is not in `PLAN_READY_STATUSES`; the page
  shows *"Sin jugadas destacables — El análisis no encontró ninguna jugada digna de highlight
  en esta partida. Prueba con otra demo."*
- **Why it matters:** This is a factual lie. The analysis has not run yet, and the copy tells
  the user their match is worthless and to go find another demo. It is the exact failure class
  the Full Demo lib was built to avoid (`FULL_DEMO_ROUNDS_PENDING` vs `FULL_DEMO_RECAP_ERROR`),
  regressing on the neighbouring Shorts surface.
- **Practical fix:** Thread the job status into the page (it is already in the
  `/api/demos/jobs` row). Three-way the empty state exactly like Full Demo does: status not
  plan-ready → "Analizando la demo… las jugadas aparecerán aquí" (with a poll or a REINTENTAR
  button); plan ready and genuinely zero plays → keep the current copy; fetch failure → error
  copy. In `match-row.tsx`, give non-plan-ready rows a `StatusTag` ("Analizando") and demote
  their CTA. Add the copy-separation test, mirroring `full-demo.test.ts`.

### BLOCKER-2 — The QA review dialog can silently collapse a Full Demo into a Shorts reel

- **Where:** `ReviewResolutionDialog` in `web/components/videos/ready-card.tsx`, plus
  `EditOptions` (`web/components/clips/edit-options.tsx`) and `constrainEditConfig`
  (`web/lib/reel-brief.ts`).
- **User moment:** A full-demo render returns with QA warnings (`review_required`). The card's
  only primary action is RESOLVER REVISIÓN QA. The dialog requires changing "al menos una
  opción" to enable re-render, then offers: a 9:16/16:9 format toggle, an "ENTREGA OPCIONAL"
  block whose toggles turn off `matchRecap` / `voiceComms` / `nativeHud`, kill effects,
  transitions, hook text, kill counter, intro/outro, KeyDrop, and covers. Flipping the format
  toggle to 9:16 makes `constrainEditConfig` silently strip `matchRecap`, `voiceComms`, and
  `nativeHud` — the locked contract evaporates without a word, and the re-render produces a
  Shorts-shaped artifact under the `__full-demo` identity.
- **Why it matters:** This is the one surface where the locked Full Demo contract is fully
  editable, and its label literally calls the contract "OPCIONAL". HUD is additionally a
  capture-stage choice (recapture required), which the dialog only whispers inside a brief
  line ("no cambia sin recaptura") while presenting the toggle as live. The nearest clickable
  "one option" a frustrated user can change to unlock the re-render button is a contract
  violation.
- **Practical fix:** When `isLandscapeRecap(original)`, render the contract as read-only facts
  (reuse `FULL_DEMO_CONTRACT` rows), hide the format toggle, the ENTREGA OPCIONAL toggles, and
  every Shorts garnish block; keep only contract-safe corrections plus the existing
  "Aceptar como intencional" path. The Shorts review path keeps today's full dialog. This is a
  conditional-render change in one component, no API change.

### BLOCKER-3 — The recap-plan wait on `/full-demo/[id]` is a dead end, and an offline recap fetch is painted as a plan error

- **Where:** `web/app/(app)/full-demo/[id]/page.tsx` (single `useEffect` fetch, no polling, no
  retry affordance); `findRecapClips` in `web/lib/api/real.ts` (409 → `[]`);
  `FULL_DEMO_ROUNDS_PENDING` in `web/lib/full-demo.ts`.
- **User moment (wait):** The recap plan is generated after parse; the orchestrator returns 409
  ("recap plan not ready") until it exists, which the client maps to an empty plays list and the
  copy *"Esta demo no tiene plan de rondas todavía. Espera a que termine el parseo o elige
  otra."* The page never re-fetches. "Espera" is a promise the UI cannot keep: the user can wait
  forever on a page that will only update on a manual browser reload. The Library polls
  (`startPollLoop`); this page does not. The trailing "o elige otra" additionally shoos the user
  away from a demo that may be seconds from ready.
- **User moment (error):** `catch { setRecapError(true) }` ignores the error entirely. If the
  orchestrator goes down between `getMatch` and `findRecapClips`, a 503 renders as
  `FULL_DEMO_RECAP_ERROR` ("No se pudo cargar el plan de rondas… Recarga o elige otra demo") —
  the service-offline case is painted as a plan failure, breaking the offline/error separation
  the same page gets right for the match load.
- **Why it matters:** This is the single wait every Full Demo user hits between picking a POV
  and forging. A dead-end wait plus misclassified offline is "getting lost" at the exact centre
  of the flow.
- **Practical fix:** Reuse `startPollLoop` (fast while `plays.length === 0 && !recapError`, stop
  once rounds land or on error), and show the pending state as a live state (spinner + "Generando
  el plan de rondas…"), not a paragraph. Classify the recap failure with
  `classifyFullDemoLoadFailure` and reuse the offline copy for 503. Keep
  `FULL_DEMO_RECAP_ERROR` for real errors — the existing tests already pin its wording apart
  from pending and 404; extend them for the offline branch.

### FLOW-1 — First-run onboarding hides Full Demo and mis-states the product as vertical-only

- **Where:** `web/app/(app)/onboarding/page.tsx` (header copy) and
  `web/components/onboarding/guide-stage.tsx` (`DOORS`).
- **User moment:** Inicio says *"ClipHub convierte una demo o un stream en un vídeo vertical
  listo para publicar"* and offers three doors: Sube una demo, Corta un stream, Busca un jugador.
  There is no door — and no sentence — for the 16:9 Full Demo flow. A first-run creator whose
  goal is a full landscape match video has no evidence the product does it, other than nav item
  03 with an English label.
- **Why it matters:** "Vídeo vertical" is now false (the product ships two deliverables), and
  the onboarding funnel routes 100% of demo intent into the Shorts pipeline. Discovery of flow
  #2 depends on rail archaeology.
- **Practical fix:** Copy: "…en un Short vertical o en un vídeo completo 16:9, y lo hace entero
  en este PC." Doors: either a fourth door to `/full-demo` ("Demo completa a vídeo · 16:9, HUD
  nativo y comms"), or make the "Sube una demo" door name both outputs in its description. The
  right-hand "QUÉ PASA DESPUÉS" plate needs no change.

### FLOW-2 — The Shorts/Full-Demo fork happens too late: `/upload` funnels every demo into Shorts, and the fork is a second, duplicate dropzone

- **Where:** `web/app/upload/page.tsx` (single flow → `router.push('/matches/' + parsed.id)`;
  picker copy "¿A QUIÉN QUIERES CLIPEAR? … forjaremos sus mejores jugadas en un reel");
  `web/components/full-demo/demo-picker.tsx` (its own `SingleDemoParse` dropzone);
  `web/lib/nav.ts` (02 Subir demo vs 03 Full demo to video as sibling rail items).
- **User moment:** The nav presents "Subir demo" and "Full demo to video" as parallel numbered
  sections, but Subir demo is not neutral: its pipeline copy promises "9:16 para Shorts o 16:9
  para largo" (step 03 of `PIPELINE_STEPS`), then every parse lands on the Shorts constructor.
  A user who wanted the full video must realise mid-flow they are in the wrong pipeline, back
  out, find `/full-demo`, and meet a second dropzone that re-scans the same demo they just
  uploaded. Intent is asked twice (which rail item?) and honoured never (upload decides for
  you).
- **Why it matters:** This is the core "choosing Shorts vs Full Demo" moment and it is
  structurally absent. Two near-identical dropzones with different destinies is a coin-flip UX:
  which one you happened to click determines your product.
- **Practical fix (smallest):** Put the fork at the moment intent is already being collected —
  the `PlayerPicker` confirm bar. It already varies by `purpose`; let the `/upload` picker offer
  both continuations for the selected player: primary "FORJAR HIGHLIGHTS" (current behaviour)
  and secondary "VÍDEO COMPLETO 16:9" (parse, then route to `/full-demo/[id]` — the parse and
  plan are shared). `/full-demo`'s own picker stays for direct entry. No new screens, no new
  API.

### FLOW-3 — No cross-navigation between the two constructors for the same demo; `/full-demo` keeps an unexplained parallel demo list

- **Where:** `web/app/(app)/matches/[id]/page.tsx`, `web/app/(app)/full-demo/[id]/page.tsx`,
  `web/components/full-demo/demo-picker.tsx` (`listPlanReadyMatches`).
- **User moment:** From the Shorts constructor there is no way to reach `/full-demo/[id]` for
  the same match, and vice versa — even though both read the same parsed job. Meanwhile
  `/full-demo` lists only `PLAN_READY_STATUSES` jobs, so a demo mid-parse simply does not
  appear: a user who just uploaded and switched sections sees their demo in Partidas but not in
  Full Demo, with no explanation of the discrepancy.
- **Why it matters:** The two flows read as two disconnected products over one library. Users
  re-upload demos they already parsed (the dropzone invites exactly that), and the invisible
  parsing state in the picker looks like data loss.
- **Practical fix:** One quiet link each way on the two `[id]` pages ("Vídeo completo 16:9 →" /
  "Highlights 9:16 →" beside the back-link). In the Full Demo picker, list parsing jobs as
  disabled rows with a "Generando plan" tag instead of omitting them.

### FLOW-4 — The Full Demo brief speaks Shorts vocabulary and buries the four facts that matter

- **Where:** `web/app/(app)/full-demo/[id]/page.tsx` builds `briefItems` via `reelCreativeBrief`
  (`web/lib/reel-brief.ts`); rendered by `web/components/full-demo/capture-bar.tsx`. The index
  page (`web/app/(app)/full-demo/page.tsx`) separately renders `FULL_DEMO_CONTRACT`.
- **User moment:** The "Configuración exacta de captura" panel lists 13 rows, most of them
  Shorts negatives: "Efecto de kill: Limpio", "Transición: Corte", "Título / contador: Sin
  título automático · Sin contador", "Intro: No", "Outro: No", "KeyDrop: No", "Música: Sin
  música", "Portada: …". The four facts the user actually confirms (16:9, rondas en vivo, comms
  85%, HUD nativo) are visually equal to seven rows of "No". The same user saw a cleaner
  6-row version of these facts (`FULL_DEMO_CONTRACT`) one click earlier on the index page —
  two different renderings of one contract.
- **Why it matters:** Explicit negatives are a house rule for the *wire* (and must stay on the
  wire — `full-demo.test.ts` pins them), but the on-screen brief is a reading task. Density
  hides the signal, and vocabulary like "Efecto de kill" and "KeyDrop" leaks the wrong product
  into the 16:9 surface. Worse, "Intro: No / Outro: No" will directly contradict the visual
  target once the imfcnd-style roster-overlay open and scoreboard close ship as part of
  `matchRecap`: the recap bookends are not the Shorts "intro/outro", and the brief has no
  vocabulary slot to say so.
- **Practical fix:** Give `/full-demo/[id]` a brief built from `FULL_DEMO_CONTRACT` (same
  labels the user already saw) plus one collapsed row "Extras de Short: ninguno (sin efectos,
  transiciones, música, KeyDrop ni portada de reel)". Keep `FULL_DEMO_EDIT` and
  `buildEditRequest` byte-identical. When recap bookends land, they get their own row
  ("Resumen: roster de inicio + marcador final"), never the intro/outro labels.

### FLOW-5 — The POV identity on `/full-demo/[id]` is a subordinate clause, and vanishes silently

- **Where:** `web/app/(app)/full-demo/[id]/page.tsx` header
  (`${match.player ? `${match.player} · ` : ''}POV de todas sus rondas en vivo…`);
  `jobToMatchEnriched` in `web/lib/api/real.ts` (roster enrichment is best-effort and swallows
  failures).
- **User moment:** The POV was locked at parse time, but the page shows the player only as a
  prefix in the description line. When roster enrichment fails (its `catch` still lists the
  demo), `match.player` is absent and the description reads "POV de todas sus rondas en vivo…"
  with no antecedent for "sus" — the single most important fact of this deliverable (whose
  match video is this?) is missing without any indication that it is missing. There is also no
  statement anywhere that changing the POV requires re-parsing.
- **Why it matters:** For a full-match video the POV *is* the product. A user who parsed the
  wrong player will only discover it after a multi-round capture.
- **Practical fix:** A dedicated POV row above the capture bar: "POV: <player> · fijado al
  parsear · para otro jugador, vuelve a parsear la demo", with an explicit "POV sin resolver —
  recarga o re-parsea" fallback when the name is unknown. Copy only; the data is already on the
  page when enrichment succeeds.

### FLOW-6 — After INICIAR CAPTURA the user is dropped into the Library with no anchor, and the Library's empty state routes only to Shorts

- **Where:** `onCreate` in `web/app/(app)/full-demo/[id]/page.tsx` and
  `web/app/(app)/matches/[id]/page.tsx` (`router.push('/videos')`); `EmptyState` in
  `web/app/(app)/videos/page.tsx` (CTA "BUSCAR JUGADAS" → `/matches`).
- **User moment:** Both forge CTAs teleport to `/videos`. In a populated library the new card
  is just another tile in an auto-fill grid, waiting on the next poll tick, with nothing marking
  "this is the one you just started". Separately, a user who visits an empty Library is sent to
  Partidas with Shorts-only copy ("Elige una jugada…"), even though the description above it
  correctly says "Shorts y partidas completas comparten el mismo estado local".
- **Why it matters:** Progress is the promise that keeps a user from re-clicking the forge CTA
  (which `reelIdentity` dedupes, but the user does not know that). Losing the just-created
  artifact in a grid is a small repeated confusion tax.
- **Practical fix:** Navigate with the new video id (`/videos?nuevo=<id>`), scroll to and flash
  that card once (`studio-reveal` already exists as a treatment). Rewrite the empty-state copy
  to cover both flows and add a second (quiet) CTA to `/full-demo`.

### FLOW-7 — "16:9" in the Library does not mean "Full Demo": the two 16:9 products are indistinguishable at a glance

- **Where:** `web/components/videos/video-filters.tsx` (aspect chips),
  `web/components/videos/reel-card.tsx` (`reelFormatLabel` badge),
  `web/lib/reel-format.ts` (the Shorts constructor also offers `landscape-16x9`).
- **User moment:** A landscape Shorts *compilation* (chosen in the Shorts bar's 9:16/16:9
  toggle) and a Full Demo recap both wear the same "16:9" badge and land under the same filter
  chip. Only the auto-title ("N rondas — …") hints at which is which.
- **Why it matters:** The task's identity requirement — Full Demo must not read as a Shorts
  reel — currently holds in the data (`__full-demo`, `matchRecap`) but not in the UI. A creator
  scanning for "the full match video" has to parse titles.
- **Practical fix:** Derive a card tag from data already present: when
  `isLandscapeRecap(video.editConfig)`, add a "PARTIDA COMPLETA" `StatusTag` next to the format
  badge (and optionally a third filter chip "Partidas completas" mapped over the same
  predicate). No new fields.

### FLOW-8 — The playback dialog is sized for 9:16; a 16:9 render previews at ~40% scale

- **Where:** `web/components/videos/ready-card.tsx`, player `Dialog`
  (`DialogContent className="max-w-md"`).
- **User moment:** The "Ver" action on a finished Full Demo opens a 1920×1080 video inside a
  `max-w-md` (~28rem) dialog. The one thing this preview must let the user judge — native HUD
  and killfeed legibility, the very reason `nativeHud` is locked on — is unreadable at that
  scale.
- **Why it matters:** The download-then-check-in-another-app detour becomes mandatory for
  every full demo, undermining the in-app QA loop the review flow depends on.
- **Practical fix:** Branch the dialog width on `reelAspect(video.editConfig)`:
  `sm:max-w-4xl` for `16:9`, keep `max-w-md` for `9:16`. One class.

### NIT-1 — "Full demo to video" is the only English label in a Spanish product

- **Where:** `web/lib/nav.ts` (nav 03), `web/app/(app)/full-demo/page.tsx` and `[id]` page
  titles ("FULL DEMO TO VIDEO"), `web/app/(app)/full-demo/layout.tsx` metadata.
- **Why it matters:** Every sibling is Spanish (Partidas, Subir demo, Táctica, Jugadores,
  Biblioteca, Ajustes). The flagship new flow is the one users must mentally translate, and the
  title doubles as the back-link label, so the anglicism repeats on every screen of the flow.
- **Fix:** "Demo completa" (nav) / "DEMO COMPLETA A VÍDEO" (title). Route stays `/full-demo`.

### NIT-2 — Nav says "Biblioteca", the page says "TUS VÍDEOS"

- **Where:** `web/lib/nav.ts` (09 Biblioteca) vs `web/app/(app)/videos/page.tsx`
  (`title="TUS VÍDEOS"`). Empty states and other copy across the app say "Biblioteca"
  (e.g. onboarding's "Todas acaban en Biblioteca").
- **Fix:** Title "BIBLIOTECA". One string.

### NIT-3 — The `Unplug` icon claims "offline" on errors that are not offline

- **Where:** `web/app/(app)/full-demo/[id]/page.tsx`
  (`icon={loadFailure === null ? SearchX : Unplug}`) and the same pattern in
  `web/app/(app)/matches/[id]/page.tsx`.
- **Why it matters:** The copy correctly separates "servicio sin conexión" from "no se pudo
  cargar", but the icon collapses them: a 500 with the service up shows a pulled plug. Icon and
  copy disagree — a small lie.
- **Fix:** `AlertTriangle` for `'error'`, `Unplug` only for `'offline'`.

### NIT-4 — The QA dialog's "Brief efectivo" prints raw wire enums

- **Where:** `ReviewResolutionDialog` in `web/components/videos/ready-card.tsx` (`brief` array:
  "Formato: landscape-16x9", "Portada: generated-gameplay").
- **Why it matters:** Every other brief in the product renders labeled values via
  `reelCreativeBrief`; this one leaks internal identifiers into user-approved text. Since the
  checkbox says "Apruebo este brief exacto", the exact thing approved should be human-readable.
- **Fix:** Build these lines from `reelCreativeBrief(draft, …)` instead of template strings.
  (Subsumed by BLOCKER-2 for the full-demo branch.)

### NIT-5 — The Library skeleton hard-codes 9:16

- **Where:** `LibrarySkeleton` in `web/app/(app)/videos/page.tsx` (`aspect-[9/16]`).
- **Why it matters:** A full-demo-heavy library loads as a column of tall vertical ghosts that
  snap into short landscape cards — layout jump exactly where the comment says the skeleton
  exists to prevent it. Acceptable while 9:16 dominates; worth revisiting once Full Demo ships.
- **Fix:** Mixed skeleton (first tile 16:9) or remember the last-seen dominant aspect.

### NIT-6 — The capture bar's "16:9" is an orphaned meta string

- **Where:** `web/components/full-demo/capture-bar.tsx` (the standalone
  `<p aria-label="Formato del vídeo">16:9</p>` between summary and CTA).
- **Why it matters:** A bare ratio floating in the sticky bar reads as debug output; the same
  fact already appears in the brief row above it. Harmless, but it is the flow's most prominent
  bar and every element on it should earn its place.
- **Fix:** Fold it into the summary line ("N rondas · POV nativo · 16:9 · sin música") or drop
  it.

---

## Suggested implementation order

1. **BLOCKER-2** (QA dialog contract lockdown) — smallest surface, highest contract risk.
2. **BLOCKER-3** (recap wait poll + offline classification) — unblocks the core Full Demo wait;
   extend `full-demo.test.ts` for the offline branch.
3. **BLOCKER-1** (pending parse vs "sin jugadas") — mirrors the Full Demo taxonomy onto
   `/matches/[id]`; add the copy-separation test.
4. **FLOW-1 + FLOW-2 + FLOW-3** together — they are one story: intent fork at the picker,
   onboarding names both deliverables, cross-links between the two `[id]` constructors.
5. **FLOW-7 + FLOW-8** — Library identity and preview; both are data-derived one-liners.
6. **FLOW-4, FLOW-5, FLOW-6**, then NITs.

Every fix above is copy, conditional rendering, or reuse of an existing primitive
(`startPollLoop`, `StatusTag`, `FULL_DEMO_CONTRACT`, `reelCreativeBrief`, `studio-reveal`).
None requires new tokens, new components of consequence, new API endpoints, or any change to
`FULL_DEMO_EDIT`, `buildEditRequest`, or the tests that pin them.
