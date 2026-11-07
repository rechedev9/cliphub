'use client';

import type { Play } from '@/lib/api/types';
import { PlayThumbnail } from './play-thumbnail';
import { playDescription, playSourceRange } from '@/lib/produce/play-details';
import { StatusTag } from '@/components/studio/status-tag';
import { cn } from '@/lib/utils';
import { playTitle, type RoundOrdinal } from '@/lib/produce/round-ordinals';
import { SelectionMark } from './selection-mark';

export type PlayRowProps = {
  play: Play;
  selected: boolean;
  /** 1-based position of this play in the reel, or null when it is not picked. */
  reelPosition: number | null;
  /** Set when the round has several plays, so repeated rows stay distinguishable. */
  ordinal?: RoundOrdinal;
  /** Name of the player whose chosen play happens at the same time; the row cannot be picked. */
  clashWith?: string;
  onToggle: () => void;
};

/** Compact highlight selector; each selected row shows its output position. */
export function PlayRow({ play, selected, reelPosition, ordinal, clashWith, onToggle }: PlayRowProps) {
  const blocked = !selected && clashWith !== undefined;
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={selected}
      disabled={blocked}
      className={cn(
        'group/play flex w-full items-center gap-3 border-b border-border-subtle px-4 py-3 @[80rem]/content:px-5 text-left last:border-b-0',
        'transition-colors duration-(--dur-fast) ease-standard',
        'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
        selected ? 'bg-primary/8' : 'bg-surface-2 hover:bg-surface-3',
        blocked && 'cursor-not-allowed hover:bg-surface-2',
      )}
    >
      <SelectionMark selected={selected} />
      {play.thumbnailUrl ? <PlayThumbnail play={play} compact className="hidden h-9 w-16 @[30rem]/reel:flex" /> : null}

      {/* min-w-0 lets the meta shrink instead of forcing horizontal scroll. */}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span
          className={cn(
            'truncate font-display text-body-sm font-bold',
            selected ? 'text-fg-1' : 'text-fg-2',
          )}
        >
          {playTitle(play, ordinal)}
        </span>
        <span className="text-body-sm text-fg-2">
          {playDescription(play)}
        </span>
        {blocked ? (
          <span className="text-body-sm text-fg-3">A la vez que la jugada elegida de {clashWith}</span>
        ) : null}
        {playSourceRange(play) ? <span className="text-label tabular-nums text-fg-2">Demo {playSourceRange(play)}</span> : null}
      </span>

      <span className="ml-auto flex shrink-0 items-center gap-3">
        {reelPosition !== null ? (
          <span className="hidden text-meta tabular-nums text-fg-2 @[26rem]/reel:inline">
            #{String(reelPosition).padStart(2, '0')}
          </span>
        ) : null}
        {/* The description already counts the kills; only an ace stands out. */}
        {play.kills >= 5 ? <StatusTag tone="primary">ACE</StatusTag> : null}
      </span>
    </button>
  );
}
