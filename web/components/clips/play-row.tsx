'use client';

import { Crosshair } from 'lucide-react';
import type { Play } from '@/lib/api/types';
import { PlayThumbnail } from './play-thumbnail';
import { playDescription, playSourceRange } from '@/lib/produce/play-details';
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

/** Compact highlight selector; each selected row shows its output position. */
export function PlayRow({ play, selected, reelPosition, onToggle }: PlayRowProps) {
  const badge = killBadge(play.kills);

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={selected}
      className={cn(
        'group/play flex w-full items-center gap-3 border-b border-border-subtle px-4 py-3 @[80rem]/content:px-5 text-left last:border-b-0',
        'transition-colors duration-(--dur-fast) ease-standard',
        'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
        selected ? 'bg-primary/8' : 'bg-surface-2 hover:bg-surface-3',
      )}
    >
      <SelectionMark selected={selected} />
      {play.thumbnailUrl ? <PlayThumbnail play={play} compact className="hidden h-9 w-16 @[30rem]/reel:flex" /> : null}

      {/* min-w-0 lets the meta shrink instead of forcing horizontal scroll. */}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span
          className={cn(
            'truncate font-display text-body-sm font-bold uppercase',
            selected ? 'text-fg-1' : 'text-fg-2',
          )}
        >
          Ronda {play.round}
        </span>
        <span className="text-body-sm text-fg-2">
          {playDescription(play)}
        </span>
        {playSourceRange(play) ? <span className="font-mono text-label tabular-nums text-fg-2">Demo {playSourceRange(play)}</span> : null}
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
