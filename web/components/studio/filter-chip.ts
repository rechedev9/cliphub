/**
 * Shared visual contract for Studio segmented controls.
 *
 * SUPERSEDED by `ToggleGroup variant="filter"` (`components/ui/toggle.tsx`). It
 * stays exported while the tactical filters migrate onto that variant.
 *
 * v4 changes: the 1.44:1 tinted edge became `--border-strong` (4.01:1, the WCAG
 * 1.4.11 floor for a control boundary), the invented `text-xs` +
 * `tracking-[0.12em]` pair became the `text-meta` step, and the transition reads
 * the motion tokens instead of Tailwind's default timing. 44px tall per
 * design.md's segmented-control rule.
 *
 * KNOWN GAP, and the reason the variant should land soon: `ToggleGroupItem`
 * pipes this through `cn()`, and tailwind-merge classifies a custom `--text-*`
 * theme key as a text COLOUR — so `text-meta` loses to `text-fg-2` and the chip
 * falls back to `toggleVariants`' `text-sm`. A component that owns its own
 * element can concatenate the step outside the merge (see `status-tag.tsx`); a
 * bare class string handed to someone else's `cn()` cannot. Teaching `cn` the v4
 * font-size group in `lib/utils.ts` is the real fix.
 */
export const STUDIO_FILTER_CHIP_CLASS =
  'h-11 min-w-16 shrink-0 border border-border-strong bg-surface-2 px-4 font-mono text-meta uppercase text-fg-2 transition-colors duration-(--dur-fast) ease-standard hover:border-border-accent hover:bg-surface-3 hover:text-fg-1 data-[state=on]:border-primary data-[state=on]:bg-primary data-[state=on]:font-semibold data-[state=on]:text-primary-foreground';
