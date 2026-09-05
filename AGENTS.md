# TickCut / ClipHub Studio

Repository-wide guidance. The checkout may be named `tickcut`; the current
user-facing product is **ClipHub Studio**. Preserve existing product names and
the `zv` CLI contract unless the task explicitly changes them.

## Project context

ClipHub is a Windows-local workstation for turning CS2 demos and stream videos
into finished videos. The Go orchestrator owns jobs, plans, capture, rendering
and artifacts. `web/` is the Next.js UI embedded by `desktop/` (Electron).
`landing/` is a separate marketing site with its own presentation.

## Product and implementation boundaries

- Demo facts and durable plans are authoritative. Preserve the shared pipeline
  across CLI and Studio; business rules belong in the owning Go package.
- Keep the three creation contracts distinct: CS2 Shorts combine selected plays
  into a vertical 9:16 video; Full Demo produces a horizontal 16:9 recap with
  native HUD and team comms; streams produce a separate Short per selected cut.
  Share presentation primitives without merging their plans or settings.
- Choosing a format or confirming a player is navigation/analysis. Preserve the
  product's review-before-production gate and invalidate approval when its
  effective configuration changes. Explain changes that require recapture,
  especially HUD settings, before starting expensive work.
- Production UI uses the typed real client in `web/lib/api`. Keep job statuses
  derived from `JOB_STATUSES`; no fixture fallback when the local service fails.
  Keep API parsing and error mapping out of components.
- Preserve the same-origin proxy, local mutation guards and Electron boundaries.
  Orchestrator tokens and URLs stay server-side; credentials never enter client
  bundles. Follow `callOrchestrator` and existing error-code contracts.
- Library publication assistance prepares the file and factual metadata, then
  opens YouTube Studio. Preserve the manual upload flow and existing QA gates;
  downloading a ready MP4 does not require selecting a cover candidate.
- Capture uses the existing Windows/HLAE tooling, windowed CS2 and one capture
  lane. Reuse the tool resolver and process ownership rules. UI verification
  must not silently initiate real capture, publishing or terminate user processes.
- Keep demos, generated media, credentials and local test artifacts outside Git.
  Format and change only files needed for the task. Follow existing strict
  TypeScript rules and package boundaries instead of introducing parallel types.

## Studio visual rules

- Preserve the broadcast control-room identity: navy surfaces, cyan actions,
  Chakra Petch display/body text and Share Tech Mono numeric/technical readouts.
  Use the existing Lucide icon family and brand assets, with consistent icon
  sizes and treatment across screens.
- `web/app/globals.css` owns surface, foreground, border, elevation, type, width
  and motion tokens. Use opaque surface steps for panel depth. Text uses the
  `--fg-*` roles without opacity suffixes; `--fg-4` is not a text color.
  Preserve the type scale's 12px floor and the shared content measures.
- Apply color by meaning: cyan for primary actions, stream/magenta for its
  defined REC/stream roles, success for completion, warning for review and red
  for errors/destructive actions. Pair state color with a readable label.
- Build through tokens → `components/ui` → `components/studio` and
  `components/brand` → `components/shell` → domain components → pages.
  Reuse `StudioPageHeader`, `StudioEmptyState`, `StatusTag`, `LongOperation`,
  `MediaFrame`, `Field` and existing selection/filter patterns.
- Fix repeated visual decisions in their shared component. Pages must not
  redefine button geometry, focus rings, media frames or status pills. Preserve
  existing geometry: panels and HUD controls need not share the same radius.
- Give each screen a clear next action and a hierarchy based on the creator's
  task. Keep previews, selections and production decisions prominent. Group
  related controls; reveal advanced settings when needed. Add containers,
  badges, icons and effects only when they communicate useful structure or state.
- Use actual media and metadata when available. Preserve the output aspect
  ratio in previews and cards; distinguish a cover from a playable video.
  Missing media gets an honest placeholder and recovery path.
- Use shared duration/easing tokens for feedback and transitions. Keep frequent
  polling and progress updates visually calm; honor reduced motion and existing
  `--shell-depth` and forced-colors behavior. Do not add a competing palette,
  font family, decorative HUD or a second design system inside a page.
- Apply these Studio rules to `web/` and its desktop presentation. Follow
  `landing/`'s own tokens and composition for marketing work.

## Interaction, state and copy

- Name the result before asking for input. Preserve source, selected player,
  chosen format and relevant options through import, navigation and recovery.
  Format-specific settings and approvals remain independent. Do not imply that
  a draft survives reload unless persistence is implemented and verified.
- Derive progress from actual job/render state. Distinguish loading, queueing,
  scanning, analysis, capture, rendering, review, ready, offline and failure.
  Show a percentage or ETA only when backed by real measurements; otherwise
  show the current stage. No active jobs does not mean capture is available.
- Empty means a successful response with no results. A pending analysis is not
  “no highlights”; unavailable data is not zero; a service error is not a missing
  demo. Preserve stale-data notices and the user's work during transient errors.
- Every blocked primary action explains what is missing nearby. Errors name
  what failed and an available next step. Retry/cancel controls must reflect
  backend capabilities and must not duplicate an in-flight production job.
- Keep geometry and context stable while loading or polling: reserve media
  dimensions, match skeletons to final rows/cards, preserve button size during
  loading, and retain focus, selection and scroll position where appropriate.
  After creating a job, make its destination and progress easy to find.
- Use clear Spanish in Studio, following existing terminology: jugadores,
  jugadas, analizar, crear vídeo, revisar y descargar. Explain technical terms
  only when they help a decision; keep raw codes and paths in diagnostics.
  Use shared formatters, explicit units and readable tabular numbers.
- Keep labels persistent, focus visible and controls keyboard accessible.
  Essential information cannot depend on hover. Preserve modal focus handling,
  Escape behavior and native file selection alongside drag-and-drop. Keep
  interactive targets at least 40px, including icon-only controls.
- Responsive work adapts composition and interaction. At narrow widths, stack
  actions, wrap long values and keep production bars from covering controls.
  Never shrink text below the type floor or hide the cause with page-level
  horizontal clipping. Test long player names, filenames and error messages.

## Verification and completion

Run the checks for the affected package, using its actual scripts:

| Change | Checks |
| --- | --- |
| `web/` code | `pnpm --dir web run lint`, `pnpm --dir web run typecheck`, `pnpm --dir web run test:unit` |
| `desktop/` code | `pnpm --dir desktop run lint`, `pnpm --dir desktop run typecheck`, `pnpm --dir desktop run test:unit` |
| Go behavior | Focused package tests; `go test ./... -count=1 -timeout 3m` when shared contracts are affected |
| `landing/` code | `pnpm --dir landing run build` |
| Documentation only | Review accuracy, local links and `git diff --check`; runtime suites are unnecessary |

- For UI changes, inspect the rendered screen and exercise the affected flow,
  including relevant loading, error and recovery states. Shell, token and upload
  changes require the applicable `web/e2e` suites via `pnpm --dir web run test:e2e`.
  Run these tests against the production standalone build of the web app.
- For shared layout/responsive changes, validate the widths in
  [web/e2e/contract.ts](web/e2e/contract.ts): currently 390, 768, 1024, 1280,
  1440 and 1920px. Check keyboard access, 200% zoom and horizontal overflow.
- Add meaningful regression coverage for behavior fixes. Keep screenshots and
  test media in temporary/ignored locations. Browser fixtures verify UI
  behavior; they do not certify HLAE capture, final media quality or Electron IPC.
- Report what changed, which checks actually ran and any unverified boundary.
  A documentation change does not require launching Studio or producing media.
