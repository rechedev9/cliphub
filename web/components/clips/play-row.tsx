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

/** Side colours carry no foreground token; the page ground reads on both in each theme. */
const SIDE_CHIP: Readonly<Record<PlaySide, string>> = {
  CT: 'bg-side-ct text-background',
  T: 'bg-side-t text-background',
};
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
        'group/play flex w-full items-center gap-3 rounded-xl border-[1.5px] px-3 py-2.5 text-left',
        'transition-colors duration-(--dur-fast) ease-standard',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
        selected ? cn(toneClasses.tint, toneClasses.border) : 'border-transparent bg-surface-2 hover:bg-surface-3',
        blocked && 'cursor-not-allowed opacity-70 hover:bg-surface-2',
      )}
    >
      <SelectionMark selected={selected} className={cn('size-5 rounded-md', selected && cn(toneClasses.bg, 'border-transparent', TICK_TEXT[tone]))} />
      {play.side ? (
        <span className={cn('grid h-8 min-w-9 shrink-0 place-items-center rounded-lg px-1.5 text-meta font-extrabold', SIDE_CHIP[play.side])}>{play.side}</span>
      ) : null}
      {play.thumbnailUrl ? <PlayThumbnail play={play} compact className="hidden h-9 w-16 rounded-md @[30rem]/reel:flex" /> : null}

      {/* min-w-0 lets the meta shrink instead of forcing horizontal scroll. */}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className={cn('truncate font-display text-body-sm font-bold', selected ? 'text-fg-1' : 'text-fg-2')}>
          {playTitle(play, ordinal)}
        </span>
        <span className="text-meta text-fg-3">{playDescription(play)}</span>
        {blocked ? (
          <span className="text-meta text-fg-3">A la vez que la jugada elegida de {clashWith}</span>
        ) : null}
        {playSourceRange(play) ? <span className="text-label tabular-nums text-fg-3">Demo {playSourceRange(play)}</span> : null}
      </span>

      <span className="ml-auto flex shrink-0 flex-col items-end tabular-nums">
        {play.death ? null : (
          <span className="flex items-center gap-2">
            {play.kills >= 5 ? <StatusTag tone="primary">ACE</StatusTag> : null}
            <span className={cn('font-display text-title font-extrabold', play.kills >= STANDOUT_KILLS ? toneClasses.text : 'text-fg-1')}>{play.kills}K</span>
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

