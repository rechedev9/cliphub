'use client';

import { Crosshair } from 'lucide-react';
import type { Play } from '@/lib/api/types';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { cn } from '@/lib/utils';
import { SelectionMark } from './selection-mark';

export type PlayRowProps = {
  play: Play;
  selected: boolean;
  /** 1-based position of this play in the reel, or null when it is not picked. */
  reelPosition: number | null;
  onToggle: () => void;
};

/**
 * Kill badge by frag count. ACE (5K) is the cyan chip, 2K-4K neutral, 1K quiet.
 * CLUTCH's magenta chip exists in the mockup only; the plan has no clutch data
 * to drive it, and inventing one would be a fabricated fact.
 */
function killBadge(kills: number): { label: string; tone: StatusTagTone } {
  if (kills >= 5) return { label: 'ACE', tone: 'primary' };
  return { label: `${kills}K`, tone: 'neutral' };
}

/**
 * The frame. `Play.thumbnailUrl` is finally read — it has existed in the typed
 * client since the killplan mapper and no component had ever rendered it — with
 * the seeded `ReelCover` painted *underneath* rather than as an either/or
 * fallback, so a cover URL the app cannot load (its CSP is
 * `img-src 'self' data: blob:`, which blocks any off-origin thumbnail outright)
 * degrades to the brand plate instead of a broken image box.
 *
 * Painting it underneath is necessary but not sufficient: a failed <img> still
 * renders the browser's broken-image glyph on top of the plate, which is what
 * `CoverImage` exists to prevent.
 */
function PlayFrame({ play, selected }: { play: Play; selected: boolean }) {
  return (
    <span className="shrink-0 [perspective:620px]">
      <span
        className={cn(
          'relative block aspect-video w-28 transform-3d @[30rem]/reel:w-32 @[52rem]/reel:w-36',
          'transition-transform duration-(--dur-base) ease-standard',
          '[transform:rotateY(calc(var(--frame-turn)*var(--shell-depth)))_translateZ(calc(var(--frame-z)*var(--shell-depth)))]',
          selected
            ? '[--frame-turn:0deg] [--frame-z:14px]'
            : '[--frame-turn:-7deg] [--frame-z:-12px] group-hover/play:[--frame-turn:-3deg] group-hover/play:[--frame-z:-2px]',
        )}
      >
        <span
          className={cn(
            'absolute inset-0 overflow-hidden border transition-[border-color,filter] duration-(--dur-base) ease-standard',
            selected ? 'border-primary' : 'border-border-strong brightness-75 group-hover/play:brightness-100',
          )}
        >
          <ReelCover seed={play.id} plain className="absolute inset-0" />
          <CoverImage src={play.thumbnailUrl} className="absolute inset-0" />
          <span
            aria-hidden
            className="pointer-events-none absolute inset-0 bg-gradient-to-t from-surface-0/85 via-transparent to-transparent"
          />
        </span>

        {/* Round slug and kill badge ride the frame's rotation but stand off its
            face, so the frame reads as a physical plate with labels on top. */}
        <span
          aria-hidden
          className="absolute bottom-1.5 left-1.5 border border-border-strong bg-surface-0/85 px-1.5 py-0.5 font-mono text-meta tabular-nums text-fg-2 [transform:translateZ(calc(12px*var(--shell-depth)))]"
        >
          R{String(play.round).padStart(2, '0')}
        </span>
      </span>
    </span>
  );
}

/**
 * PlayRow — one highlight in the vertical selector (the layout the E2E suite
 * and design.md pin down; the horizontal filmstrip stays retired).
 *
 * Selection is staged as a physical action rather than as a checkbox tint:
 * unselected frames sit turned away and pushed back, as if racked in a film bin,
 * and picking one swings it square to the viewer, brings it forward and lights
 * its edge cyan. Everything is `transform`/`filter` on one element, multiplied
 * by `--shell-depth`, so the whole choreography collapses to the flat ring +
 * tint under the efficiency profile, reduced motion and forced colours.
 */
export function PlayRow({ play, selected, reelPosition, onToggle }: PlayRowProps) {
  const badge = killBadge(play.kills);

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={selected}
      className={cn(
        'group/play flex w-full items-center gap-3.5 border-b border-border-subtle px-3 py-3 text-left last:border-b-0',
        'transition-colors duration-(--dur-fast) ease-standard',
        'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
        selected ? 'bg-primary/8' : 'bg-surface-2 hover:bg-surface-3',
      )}
    >
      <SelectionMark selected={selected} />
      <PlayFrame play={play} selected={selected} />

      {/* min-w-0 lets the meta shrink instead of forcing horizontal scroll. */}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span
          className={cn(
            'truncate font-display text-body-lg font-bold uppercase',
            selected ? 'text-fg-1' : 'text-fg-2',
          )}
        >
          Ronda {play.round}
        </span>
        <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
          {play.weapon ?? `${play.kills} ${play.kills === 1 ? 'kill' : 'kills'}`}
        </span>
      </span>

      <span className="ml-auto flex shrink-0 items-center gap-3">
        {reelPosition !== null ? (
          <span className="hidden font-mono text-meta uppercase tracking-wider tabular-nums text-primary @[26rem]/reel:inline">
            #{String(reelPosition).padStart(2, '0')}
          </span>
        ) : null}
        {/* The Crosshair also anchors the E2E pick-a-clip selector
            (button:has(.lucide-crosshair)); keep it inside this button. */}
        <StatusTag tone={badge.tone} icon={Crosshair} className="tabular-nums">
          {badge.label}
        </StatusTag>
      </span>
    </button>
  );
}
