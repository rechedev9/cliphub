'use client';

import type { RenderFormat } from '@/lib/api/types';
import { canForgeReel, type CreativeBriefItem } from '@/lib/reel-brief';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export type CreateReelBarProps = {
  /**
   * Selection summary, or null when nothing is picked. One highlight reuses
   * its own label ("1K · Ronda 1"); 2+ summarize as a count plus rounds
   * ("3 jugadas · Rondas 1, 6, 9") — see lib/format#playsSelectionLabel.
   */
  selectionLabel: string | null;
  /** Label of the chosen preset, or null when none chosen. */
  presetLabel: string | null;
  /** Title of the chosen soundtrack, or null when the reel has no music. */
  songTitle: string | null;
  /** False until the user picks a track or explicitly chooses no music. */
  musicDecided: boolean;
  /** Reel aspect (the mockup's 9:16 / 16:9 segmented toggle). */
  format: RenderFormat;
  onFormatChange: (format: RenderFormat) => void;
  /** Whether a render is in flight (spinner + disabled). */
  creating: boolean;
  briefItems: CreativeBriefItem[];
  briefApproved: boolean;
  onBriefApprovedChange: (approved: boolean) => void;
  onCreate: () => void;
};

const FORMAT_ITEMS: Array<{ value: RenderFormat; label: string }> = [
  { value: 'short-9x16', label: '9:16' },
  { value: 'landscape-16x9', label: '16:9' },
];

/**
 * CreateReelBar — the sticky bottom action bar: the mono REEL summary, the exact
 * creative brief with its approval gate, the 9:16/16:9 aspect toggle and the
 * notched cyan FORJAR REEL CTA. Enabled once at least one highlight and a preset
 * are chosen and music is decided; 2+ selected highlights render as one concatenated reel.
 *
 * The brief and its checkbox stay inside this bar on purpose: the approval gate
 * is a product contract ("approval must answer a shown brief"), so the thing
 * being approved has to travel with the button that acts on it.
 *
 * It bleeds to the shell gutter with `-mx-(--shell-gutter)`, matching `<main>`'s
 * padding token exactly — the old `-mx-4 md:-mx-8` matched the layout at no
 * breakpoint once the gutter became fluid. No backdrop blur: a full-width sticky
 * strip re-reads its backdrop on every scroll frame, which is the one blur cost
 * the performance budget rules out outright.
 */
export function CreateReelBar({
  selectionLabel,
  presetLabel,
  songTitle,
  musicDecided,
  format,
  onFormatChange,
  creating,
  briefItems,
  briefApproved,
  onBriefApprovedChange,
  onCreate,
}: CreateReelBarProps) {
  const configured = selectionLabel != null && presetLabel != null && musicDecided;
  const ready = canForgeReel({
    briefApproved,
    creating,
    hasPreset: presetLabel !== null,
    selectionCount: selectionLabel === null ? 0 : 1,
    musicDecided,
  });

  return (
    <div className="sticky bottom-0 z-20 -mx-(--shell-gutter) mt-2 border-t border-border-accent bg-surface-1 px-(--shell-gutter) py-4 shadow-[0_-12px_28px_-18px_oklch(0.02_0.02_264/0.9)]">
      <div className="flex flex-col gap-4">
        <section
          className="studio-panel px-4 py-3"
          aria-labelledby="creative-brief-title"
        >
          <p id="creative-brief-title" className="font-mono text-meta uppercase tracking-wider text-primary">
            Brief creativo exacto
          </p>
          <dl className="mt-2.5 grid gap-x-6 gap-y-1.5 text-body-sm @[42rem]/content:grid-cols-2 @[70rem]/content:grid-cols-3">
            {briefItems.map((item) => (
              <div key={item.label} className="flex min-w-0 gap-1.5">
                <dt className="shrink-0 text-fg-3">{item.label}:</dt>
                <dd className="truncate text-fg-1" title={item.value}>{item.value}</dd>
              </div>
            ))}
          </dl>
          <label className="mt-3.5 flex min-h-10 items-center gap-2.5 text-body-sm text-fg-1">
            <input
              type="checkbox"
              checked={briefApproved}
              disabled={!configured || creating}
              onChange={(event) => onBriefApprovedChange(event.target.checked)}
              className="size-5 shrink-0 cursor-pointer accent-primary disabled:cursor-not-allowed disabled:opacity-50"
            />
            Apruebo todas estas decisiones antes de iniciar la captura o el render.
          </label>
        </section>

        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
          <div className="min-w-0 flex-1">
            <p className="font-mono text-meta uppercase tracking-widest text-fg-3">REEL</p>
            {configured ? (
              <p className="mt-1 truncate font-mono text-body uppercase text-fg-1">
                {selectionLabel}
                <span className="text-fg-3"> · </span>
                <span className="text-primary">{presetLabel}</span>
                {songTitle ? <span className="text-fg-2"> · ♪ {songTitle}</span> : <span className="text-fg-3"> · sin música</span>}
              </p>
            ) : (
              <p className="mt-1 truncate text-body-sm text-fg-2">
                {forgeHint(selectionLabel, presetLabel)}
              </p>
            )}
          </div>

          <div
            role="group"
            aria-label="Formato del reel"
            className="flex shrink-0 font-mono text-label tracking-wider"
          >
            {FORMAT_ITEMS.map((item) => (
              <button
                key={item.value}
                type="button"
                aria-pressed={format === item.value}
                disabled={creating}
                onClick={() => onFormatChange(item.value)}
                className={cn(
                  'inline-flex min-h-11 items-center px-5 transition-colors duration-(--dur-fast) ease-standard',
                  'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
                  'disabled:pointer-events-none disabled:opacity-50',
                  format === item.value
                    ? 'bg-primary text-primary-foreground'
                    : 'border border-border-strong text-fg-2 hover:border-primary/55 hover:text-fg-1',
                )}
              >
                {item.label}
              </button>
            ))}
          </div>

          {/* A clipped CTA cannot paint an outer ring beyond its polygon, so the
              focus outline is pulled inside the notch. */}
          <Button
            variant="hero"
            size="lg"
            disabled={!ready}
            loading={creating}
            loadingText="FORJANDO REEL…"
            onClick={onCreate}
            className="neon-notch shrink-0 focus-visible:-outline-offset-4"
          >
            FORJAR REEL
          </Button>
        </div>
      </div>
    </div>
  );
}

function forgeHint(selectionLabel: string | null, presetLabel: string | null): string {
  if (selectionLabel == null) return 'Elige al menos una jugada para empezar.';
  if (presetLabel == null) return 'Elige un preset para continuar.';
  return 'Decide la música: un tema o sin música.';
}
