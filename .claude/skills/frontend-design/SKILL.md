---
name: frontend-design
description: ClipHub Studio visual work. Use when changing UI, CSS, layout, components, copy, empty states, or anything the user sees in web/. Reads ~/.grok/design.md then tokens in web/app/globals.css. Never invent tokens or use Claude Design.
license: MIT
metadata:
  zv-catalog: "false"
---

# Frontend Design

This skill is anti-slop, not a second design system. ClipHub has no product `design.md`. Tokens in `web/app/globals.css` are the source of truth; this skill holds identity and hierarchy.

## Load first

1. Read `~/.grok/design.md` (shared method + component source indexes). Fetch those indexes in full when borrowing a module.
2. Read tokens in `web/app/globals.css`. Every surface, foreground, border, elevation, type step, and color role comes from there.
3. Read `web/CLAUDE.md` for TypeScript, React, and proxy rules.
4. Follow the hierarchy below. Do not restyle a page when a primitive or studio kit component already owns that job.

Stop if the request would invent a parallel palette, call Claude Design, run `/design-sync`, or ship a one-off HTML mock instead of editing `web/`.

## Hierarchy

1. tokens in `app/globals.css`
2. primitives in `components/ui`
3. the shared kit in `components/studio` and the identity layer in `components/brand`
4. the shell in `components/shell`
5. domain components (`matches`, `clips`, `upload`, `videos`, `feed`, `streams`, `series`, `settings`)
6. pages and responsive composition

Pages must not duplicate title blocks, empty-state layouts, button geometry, input focus rules, status pills, icon tiles, media frames, or filter-chip styling when a shared component exists.

## Contract wins

`web/app/globals.css` plus this skill beat generic taste on every conflict.

- Do not take an "aesthetic risk" that adds a typeface, hex, glow, glass/alpha surface, or decorative HUD.
- Do not introduce Inter, Roboto, Open Sans, Lato, purple-on-white, cream + terracotta, acid-green-on-black, or broadsheet defaults.
- Do not write text with an alpha suffix. Pick a `--fg-*` step.
- Depth is an opaque surface step plus a 1px bevel plus a shadow. Never transparency.
- Studio is a calm broadcast control room, not a generic SaaS dashboard and not a cyberpunk skin.

This skill applies to `web/`. `landing/` is a separate hosted marketing surface; do not apply Studio tokens there unless the task says so.

## Free axes only

These are the only places this skill may push:

- Copy: name things the user controls, active voice, sentence case, one job per string. Errors say what failed and how to fix it. Empty states invite an action.
- Structure: eyebrows, dividers, and numbering only when they encode a real sequence or state. Decoration that does not serve the brief gets cut.
- Motion: one restrained moment beats scattered effects. Respect `prefers-reduced-motion`. Extra animation reads as generated.
- Self-critique: the signature is already the control-room identity. Spend boldness nowhere else. Before finishing, remove one accessory.

## Process

1. Name the surface or component and the single job of the change.
2. List the existing tokens and primitives you will use. If a needed token is missing, say so and stop; do not invent hex.
3. Build through the hierarchy above.
4. Critique the result against `globals.css` and this skill, not against generic "distinctive" taste. If a screenshot tool exists, use it.

Do not announce this process in the reply.
