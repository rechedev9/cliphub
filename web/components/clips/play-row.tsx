'use client';

import type { Play, PlaySide } from '@/lib/api/types';
import { PlayThumbnail } from './play-thumbnail';
import { playDescription, playSourceRange } from '@/lib/produce/play-details';
import { estimatedPlaySeconds } from '@/lib/produce/short-selection';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { StatusTag } from '@/components/studio/status-tag';
import { cn } from '@/lib/utils';
import { playTitle, type RoundOrdinal } from '@/lib/produce/round-ordinals';
import { SelectionMark } from './selection-mark';

/** The side shows as a stripe on the card's edge and a label in the same colour. */
const SIDE_STRIPE: Readonly<Record<PlaySide, string>> = { CT: 'bg-side-ct', T: 'bg-side-t' };
const SIDE_LABEL: Readonly<Record<PlaySide, string>> = { CT: 'text-side-ct', T: 'text-side-t' };
/** The tick sits on the solid format colour, so it takes that colour's foreground. */
const TICK_TEXT: Readonly<Record<FormatTone, string>> = {
  short: 'text-format-short-foreground',
  full: 'text-format-full-foreground',
  stream: 'text-format-stream-foreground',
  summary: 'text-format-summary-foreground',
};
/** From a 4K the kill count takes the format colour. */
const STANDOUT_KILLS = 4;

export type PlayRowProps = {
  play: Play;
  selected: boolean;
  /** 1-based position of this play in the reel, or null when it is not picked. */
  reelPosition: number | null;
  /** Set when the round has several plays, so repeated rows stay distinguishable. */
  ordinal?: RoundOrdinal;
  /** Name of the player whose chosen play happens at the same time; the row cannot be picked. */
  clashWith?: string;
  /** The format colour of the flow: it marks the selected card. */
  tone?: FormatTone;
  onToggle: () => void;
};

/** A selectable play card; each selected card shows its output position. */
export function PlayRow({ play, selected, reelPosition, ordinal, clashWith, tone = 'short', onToggle }: PlayRowProps) {
  const blocked = !selected && clashWith !== undefined;
  const toneClasses = FORMAT_TONE[tone];
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={selected}
      disabled={blocked}
      className={cn(
        'group/play relative flex w-full items-center gap-3 overflow-hidden rounded-lg border px-3 py-2.5 text-left',
        'transition-colors duration-(--dur-fast) ease-standard',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
        selected ? 'border-border-strong bg-surface-3' : 'border-border-subtle bg-surface-2 hover:bg-surface-3',
        blocked && 'cursor-not-allowed opacity-70 hover:bg-surface-2',
      )}
    >
      <SelectionMark selected={selected} className={cn('size-5 rounded-md', selected && cn(toneClasses.bg, 'border-transparent', TICK_TEXT[tone]))} />
      {play.side ? (
        <>
          <span aria-hidden className={cn('absolute inset-y-0 left-0 w-1', SIDE_STRIPE[play.side])} />
          <span className={cn('w-5 shrink-0 font-mono text-meta font-semibold', SIDE_LABEL[play.side])}>{play.side}</span>
        </>
      ) : null}
      {play.thumbnailUrl ? <PlayThumbnail play={play} compact className="hidden h-9 w-16 rounded-md @[30rem]/reel:flex" /> : null}

      {/* min-w-0 lets the meta shrink instead of forcing horizontal scroll. */}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className={cn('truncate font-display text-body-sm font-semibold', selected ? 'text-fg-1' : 'text-fg-2')}>
          {playTitle(play, ordinal)}
        </span>
        <span className="text-meta text-fg-3">{playDescription(play)}</span>
        {blocked ? (
          <span className="text-meta text-fg-3">A la vez que la jugada elegida de {clashWith}</span>
        ) : null}
        {playSourceRange(play) ? <span className="font-mono text-meta text-fg-3">Demo {playSourceRange(play)}</span> : null}
      </span>

      <span className="ml-auto flex shrink-0 flex-col items-end tabular-nums">
        {play.death ? null : (
          <span className="flex items-center gap-2">
            {play.kills >= 5 ? <StatusTag tone="primary">ACE</StatusTag> : null}
            <span className={cn('font-mono text-title font-semibold', play.kills >= STANDOUT_KILLS ? toneClasses.text : 'text-fg-1')}>{play.kills}K</span>
          </span>
        )}
        <span className="text-meta text-fg-3">
          {reelPosition !== null ? <span className="hidden @[26rem]/reel:inline">#{String(reelPosition).padStart(2, '0')} · </span> : null}
          {Math.round(estimatedPlaySeconds(play))} s
        </span>
      </span>
    </button>
  );
}

