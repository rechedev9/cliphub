# ClipHub Studio — design system v4

This file is the presentation contract for `web/`.
The typed API layer, polling, local/cloud routing, and the demo-to-render state machines are not part of the visual system and must remain stable during UI work.

## Product idea

ClipHub is a local-first replay workstation: a focused CS2 production tool, not a generic SaaS dashboard and not a decorative cyberpunk HUD.
The interface should feel like a calm broadcast control room: technical, dense where data is useful, and quiet around the current action.

The visual identity is:

- blue-black canvas and layered navy surfaces, with depth carried by an opaque surface step plus a 1px bevel plus a shadow — never by transparency;
- cyan for primary action, selection, navigation, and keyboard focus;
- vermilion for the wordmark micro-accent; magenta remains reserved for stream-specific signals such as REC, facecam, music, and likes;
- mint for ready/success, amber for warnings/expiry, red for failures and destructive actions;
- angular geometry, thin borders, restrained corner cuts, and minimal glow;
- one global light direction, and a receding perspective floor on the canvas only.

## Bottom-up hierarchy

Visual changes are made in this order:

1. tokens in `app/globals.css`;
2. primitives in `components/ui`;
3. the shared kit in `components/studio` and the identity layer in `components/brand`;
4. the shell in `components/shell`;
5. domain components (`matches`, `clips`, `upload`, `videos`, `feed`, `streams`, `series`, `news`, `settings`);
6. pages and responsive composition.

Pages must not duplicate title blocks, empty-state layouts, button geometry, input focus rules, status pills, icon tiles, media frames, or filter-chip styling when a shared component exists.

## Foundations

The CSS variables in `app/globals.css` are the source of truth.
Every value below was validated against WCAG by measurement, not by eye.

### Surfaces

Six opaque steps, monotonic in lightness, single hue.
Alpha is never used to fake a layer: compositing a panel over the canvas at 94% was what made v3's panels measure 1.023:1 against the page they sat on.

| Token | Use |
| --- | --- |
| `--surface-0` | void: sidebar, chrome gutters, inset wells |
| `--surface-1` | app canvas |
| `--surface-2` | standard panel, card |
| `--surface-3` | raised or selected panel, field on a panel |
| `--surface-4` | popover, dropdown, select, dialog |
| `--surface-5` | control on a popover |

A popover must always sit higher on the ramp than the panel it floats over.

### Foreground

`--fg-1` headings and essential content, `--fg-2` descriptions and metadata, `--fg-3` de-emphasised labels, `--fg-4` decorative hairlines only.
`--fg-3` is the AA floor and clears 4.5:1 on every surface step.
**`--fg-4` may never carry text.**
Never write text with an alpha suffix (`text-muted-foreground/60`): pick a ramp step instead.

### Borders

`--border` for non-interactive panel edges, `--border-strong` for **any** control boundary (WCAG 1.4.11 needs 3:1; it measures 4.01:1), `--border-accent` when the edge should carry the cyan brand cue.
`--edge-light` and `--edge-shade` are the 1px inset pair that turns a rectangle into a physical plate.

### Elevation

`--elev-0` … `--elev-5`, each built as bevel + key + ambient, mapped onto `shadow-sm` … `shadow-2xl`.
Panels use `--elev-1`, interactive hover `--elev-2`, raised or selected `--elev-3`, popovers `--elev-4`, dialogs and sheets `--elev-5`.
Glows are `--glow-primary-sm/md/lg` and `--glow-stream-md`.
Note that Tailwind cannot splice a colour into a bare `var(--elev-N)`, so `shadow-primary/15` and friends are inert — use a glow token for brand emphasis.

### Type

Display/UI is Chakra Petch, operational metadata and numbers are Share Tech Mono with tabular figures.
Use the scale steps, never an arbitrary size: `text-meta` (12px, the hard floor) · `text-label` · `text-body-sm` · `text-body` (15/24, the default) · `text-body-lg` · `text-title` · `text-section` · `text-display-sm` · `text-display` · `text-hero` · `text-stat`.
Tracking is `tracking-tight/wide/wider/widest/ultra`.
Wide tracking and full uppercase are reserved for eyebrows, states, and compact metadata — not paragraphs.

Because tailwind-merge classifies a custom `--text-*` theme key as a text colour, `cn('text-meta', 'text-fg-2')` would drop the size; the merge config is extended for the scale, and any new step must be added there.

### Motion

Durations are `--dur-instant/fast/base/slow/data` and easings `--ease-standard/entrance/exit/pop`.
No ad-hoc `duration-150 ease-out`.

### Space and controls

Use the scale `4, 8, 12, 16, 24, 32, 48, 64, 96`.

- Default button/input: 44px high. Small button/icon control: at least 40px.
- Panel padding: 16–24px. Main content gutter: `--shell-gutter`.
- Sidebar row: 48px. Command strip: 56px.
- Focus: a visible 2px cyan **outline** with a 2px offset, from the exported `FOCUS_RING`. An outline is used rather than a ring because a ring's offset must paint a solid colour, which produced a black halo on every control inside a panel. No primitive may carry a bare `outline-none`.

## Depth and 3D

Depth is a first-class part of the identity, and all of it is CSS transforms, gradients and one hand-written WebGL layer — **no 3D dependency is permitted**.

- `ShellCanvas` paints the room: horizon wash, two-level lattice, a CSS-3D perspective floor, scanlines and a vignette. It is fixed, static, `aria-hidden`, and behind everything.
- `StudioAmbient` adds a half-resolution, 30fps, low-power WebGL depth field. It renders a single static frame instead of animating whenever motion is not appropriate, and removes itself when WebGL is unavailable.
- `.studio-tilt` / `TiltSurface` gives media surfaces a pointer-tracked parallax. It writes `--tilt-x`/`--tilt-y` as **normalised −1..1** values and `--px`/`--py` as 0..100; the ceiling lives in `--tilt-max`, never in the JS.
- `.pipeline-rail` stages the pipeline as a depth ladder. Its static arrangement deliberately survives the efficiency profile — only its transition degrades.
- `.studio-rim`, `.studio-scanline` and the dialog's Z-axis entrance complete the vocabulary.

**Every effect multiplies by the single `--shell-depth` scalar**, which drops to 0 under `prefers-reduced-motion`, `forced-colors`, `data-performance-profile="efficiency"`, `data-window-activity="inactive"` and `data-capture-active="true"`.
The last one matters most: a capture shares the GPU with `cs2.exe`, so nothing decorative may contend with it.
Adding a new effect means reading that scalar — not writing a new media query.

Readability outranks depth everywhere. Tilt ceilings on dense text surfaces stay near 1°, and no depth effect may move a text baseline enough to be legible as motion.

## Shared patterns

`components/studio` is the shared kit; reach for it before writing a class string.

- `StudioPageHeader` — title, description, optional actions. It renders **no** section eyebrow: the command strip states the current section persistently, so a second one under it was a literal repetition. Screens outside the app shell (`/upload`) render `SectionEyebrow` themselves.
- `StudioEmptyState` — bounded, actionable panel with an icon, H2, concise copy, one primary action, an optional secondary action, and an optional trust/status line. Normally 640–760px wide.
- `StatusTag` — every state pill. Square HUD geometry, semantic tone, optional dot or icon.
- `IconTile` — every bordered icon square.
- `MediaFrame` — every media box: aspect, cover fallback, corner slots, scrim, scanline, and a hover action layer that is always keyboard-reachable and always visible on touch. `capHeight` bounds a portrait frame in a grid.
- `CoverImage` — any API-supplied cover. It unmounts on failure, including the SSR case where the error fires before React attaches a handler; the CSP is `img-src 'self' data: blob:`, so an off-origin cover is guaranteed to fail, and a bare `<img>` would paint a stretched broken-image glyph over the fallback art.
- `SelectableCard` — any card that is a choice. Real focus ring, `aria-pressed`, accent edge.
- `LongOperation` — anything slow, with a stage label, real progress and `aria-live`.
- `StudioDataRow`, `StudioBackLink`, `TiltSurface` complete the set.

Buttons use the `Button` variants (`hero`, `stream`, `success`, `warning`, `outline-primary`, `loading`) rather than hand-rolled class strings.
Fields use `Field` so labels, hints, errors and `aria-describedby` are wired once.

## Shell

- Desktop sidebar: 240px, grouped by purpose, with an extruded active key and a real collapsible icon rail whose state persists server-side.
- Command strip: 56px across the full content inset, carrying the sidebar toggle, the breadcrumb, the live job transport and, while CS2 is recording, the magenta capture pip. The mobile bar is its narrow variant, not a separate header.
- Main content: `@container/content`, max 1440px, pinned to the sidebar edge with a fluid `--shell-gutter`.
- The shell is two columns, sidebar and content. There is no third rail: capture readiness lives at the foot of the sidebar.
- Global states are designed, not defaults: `loading.tsx`, `error.tsx`, `global-error.tsx` and an in-shell `not-found.tsx`.

`/upload` remains outside the authenticated route group because it supports the no-login flow.
It uses a compact standalone top bar and the same content tokens, widths, typography, and controls as the Studio shell.

## Responsive

**Domain components key their breakpoints to `@container/content`, not to the viewport.**
The content column is narrower than the window — a 1280px viewport leaves ~960px once the 240px sidebar and the `--shell-gutter` insets are taken out — so viewport variants fired at widths the layout never actually had.

Validate at 390, 768, 1024, 1280, 1440, and 1920px and at 200% zoom.

- Horizontal overflow must be 0 at every width. Measure it; do not assume it.
- Below the container thresholds, forms and paired actions stack; filters may scroll.
- Cards collapse to one column without horizontal overflow.

## Screen contracts

### Matches

The Studio inbox. Empty state routes to demo upload and stream import.
Populated rows are dense scoreboard surfaces: map still, map/context, `MatchScore`, K/D/A/MVP/KD, timestamp/highlight count, and a clear 44px action.
The row lifts and tilts by ~1° under the pointer with a specular sweep driven by one listener on the list.

### Matches detail

The highlight selector, and the most important screen in the product.
Two panes above `@[64rem]`: the vertical plays list, and a sticky reel-build column holding preset, edit options and music.
Selection is a physical action — an unselected frame sits turned away and pushed back, and picking it swings it square and lights its edge.
The selector stays a **vertical list**; the obsolete horizontal filmstrip contract is not revived.
The creative brief and its approval checkbox travel with the FORJAR REEL button, because approval must answer a shown brief.

### Upload

Keep the real `scan → player picker → parse` flow and the file input exactly as they are.
The dropzone is one large keyboard-operable target with clear drag, focus, error, scanning, picking, offline, and parsing states.
State that processing is local in readable text.

### Stream clips

The source panel uses a neutral surface; magenta identifies the stream action, not the whole background.
URL and MP4 paths remain equally discoverable.
The render stage has a real render UI built on `LongOperation`, and results are presented in framed media, not bare `<video>` elements.
Do not mix visual refactoring with changes to acquisition or render state.

### Library

Auto-fill grid of `ReelCard`, whose frame aspect is driven by the real `editConfig.format` — a shorts tool whose library crops every 9:16 reel to 16:9 is showing something it did not make.
Stage and progress share one track along the card's bottom edge.
Ready, rendering and failed are states of one card, not three card languages.
Preserve real media URLs, publishing, deletion, and polling semantics.

### Feed

Responsive portrait-biased card grid.
The play control is the card's dominant object; the like control is a quiet ghost with a mono count.
Community/like details may use magenta; global selection remains cyan.

## Accessibility

- Standard text contrast at least 4.5:1; controls and focus at least 3:1.
- Interactive targets at least 40px and normally 44px. Visually hidden 1×1 inputs are exempt only when a real label control fronts them.
- Use `aria-current` for navigation, `aria-live` or `role=alert` for async status/errors, and accessible names for icon actions.
- Never rely on colour — or on depth, which is weaker — to communicate state; pair it with text or an icon.
- Respect `prefers-reduced-motion` and `forced-colors`.
- Never hide essential controls behind hover only; keyboard focus must reveal equivalent actions.

## Functional invariants

- Keep `web/lib/api/*`, API field names, object URLs, polling, and local/cloud routing stable unless the task explicitly changes behaviour.
- The initial poll tick runs even on an unfocused window; only the repeating loop suspends. A screen that holds its skeleton until the user focuses the window reads as a hung app.
- Preserve `/upload` file input and roster flow.
- Preserve accessible labels and existing E2E hooks such as `data-slot="card"`, `data-testid="player-avatar"`, download/delete labels, and the sticky reel action.
- Use real API data. Never fabricate progress, duration, output format, or media availability merely to fill a design.
