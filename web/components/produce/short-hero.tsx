import type { ReactNode } from 'react';
import type { PlaySide } from '@/lib/api/types';
import { formatClock, SHORT_TARGET_SECONDS, type SelectionCue } from '@/lib/produce/short-selection';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { cn } from '@/lib/utils';

/** Side colours carry no foreground token; the page ground reads on both in each theme. */
const SIDE_BLOCK: Readonly<Record<PlaySide, string>> = {
  CT: 'bg-side-ct text-background',
  T: 'bg-side-t text-background',
};
/** Below this many seconds the room block is too narrow for its label. */
const ROOM_LABEL_MIN_SECONDS = 6;

export type ShortHeroProps = {
  tone: FormatTone;
  title: string;
  /** Map, player and what to do here, under the title. */
  meta: ReactNode;
  /** Status lines (draft restored, rounds from the analysis) under the meta line. */
  notices?: ReactNode;
  cues: readonly SelectionCue[];
  totalSeconds: number;
  /** The length the video should stay within; a Short is one minute. */
  targetSeconds: number;
};

/** The flow's band: title, the running total against the target, and the filmstrip of chosen plays. */
export function ShortHero({ tone, title, meta, notices, cues, totalSeconds, targetSeconds }: ShortHeroProps): ReactNode {
  const over = totalSeconds > targetSeconds;
  return (
    <section aria-labelledby="short-hero-title" className={cn('flex min-w-0 flex-col gap-4 rounded-2xl border px-5 py-5 @[56rem]/content:px-6', FORMAT_TONE[tone].tint, FORMAT_TONE[tone].border)}>
      <div className="flex flex-wrap items-end gap-x-5 gap-y-3">
        <div className="flex min-w-0 flex-[1_1_22rem] flex-col gap-1.5">
          <h1 id="short-hero-title" className="wrap-anywhere font-display text-display-sm font-semibold text-fg-1">{title}</h1>
          <p className="wrap-anywhere text-body-sm text-fg-2">{meta}</p>
          {notices}
        </div>
        <p className="flex items-baseline gap-2 tabular-nums">
          <span className={cn('font-mono text-display font-medium leading-none tracking-[-0.03em]', over ? 'text-warning' : 'text-fg-1')}>{formatClock(totalSeconds)}</span>
          {/* No tilde: beside the figures it reads as a minus sign ("-0:00"). */}
          <span className="text-body text-fg-2">de {formatClock(targetSeconds)} aprox.</span>
        </p>
      </div>
      <Filmstrip tone={tone} cues={cues} totalSeconds={totalSeconds} targetSeconds={targetSeconds} />
      {over ? (
        <p className="text-body-sm text-warning">
          {targetSeconds === SHORT_TARGET_SECONDS
            ? 'Pasa del minuto: quita una jugada o acepta un Short más largo.'
            : `Pasa de ${formatClock(targetSeconds)}: quita una jugada o acepta un vídeo más largo.`}
        </p>
      ) : null}
    </section>
  );
}

type FilmstripProps = Omit<ShortHeroProps, 'title' | 'meta' | 'notices'>;

/** One block per chosen play, as wide as its length, then the room left before the target. */
function Filmstrip({ tone, cues, totalSeconds, targetSeconds }: FilmstripProps): ReactNode {
  const room = Math.max(0, Math.round(targetSeconds - totalSeconds));
  const label = cues.length === 0
    ? 'Ninguna jugada elegida todavía'
    : `${cues.length} ${cues.length === 1 ? 'jugada' : 'jugadas'} en orden, ${formatClock(totalSeconds)}`;
  return (
    <div role="img" aria-label={label} className="flex h-9 min-w-0 gap-1">
      {cues.map((cue, index) => (
        <span key={cue.play.id} style={{ flex: `${cue.seconds} 1 0` }}
          className={cn('flex min-w-1 items-center overflow-hidden whitespace-nowrap rounded-md px-2 text-meta font-bold tabular-nums',
            cue.play.side ? SIDE_BLOCK[cue.play.side] : FORMAT_TONE[tone].fill)}>
          {index + 1}{cue.play.death ? '' : ` · ${cue.play.kills}K`}
        </span>
      ))}
      {room > 0 ? (
        <span style={{ flex: `${room} 1 0` }}
          className={cn('flex min-w-1 items-center overflow-hidden whitespace-nowrap rounded-md border-[1.5px] border-dashed px-2 text-meta text-fg-3', FORMAT_TONE[tone].border)}>
          {room > ROOM_LABEL_MIN_SECONDS ? `Caben ${room} s más` : ''}
        </span>
      ) : null}
    </div>
  );
}
