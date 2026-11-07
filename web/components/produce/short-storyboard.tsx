import type { ReactNode } from 'react';
import { Crosshair } from 'lucide-react';
import { formatClock, SHORT_TARGET_SECONDS, type SelectionCue } from '@/lib/produce/short-selection';
import { cn } from '@/lib/utils';
import { playClock, playDescription } from '@/lib/produce/play-details';
import { playTitle, type RoundOrdinal } from '@/lib/produce/round-ordinals';

const STORYBOARD_TITLE = 'Guion del Short';
const EMPTY_HINT = 'Los highlights elegidos aparecen aquí en el orden en que saldrán.';

/** The running order of the Short: real cues from the plan, not a placeholder frame. */
export function ShortStoryboard({
  cues,
  totalSeconds,
  ordinals,
  title = STORYBOARD_TITLE,
  targetSeconds = SHORT_TARGET_SECONDS,
  videoLabel = 'Short',
}: {
  cues: SelectionCue[];
  totalSeconds: number;
  /** Ordinals computed over every play of the match, so they match the play list. */
  ordinals: ReadonlyMap<string, RoundOrdinal>;
  title?: string;
  /** The length the video should stay within; a Short is one minute. */
  targetSeconds?: number;
  /** What the cue clock is measured in: "Short 0:12". */
  videoLabel?: string;
}): ReactNode {
  const over = totalSeconds > targetSeconds;
  const fill = Math.min(100, (totalSeconds / targetSeconds) * 100);
  return (
    <section aria-label={title} className="studio-panel flex min-w-0 flex-col gap-3 p-4 @[80rem]/content:gap-4 @[80rem]/content:p-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-body-sm font-semibold text-fg-2">{title}</span>
        <span className={cn('text-meta tabular-nums', over ? 'text-warning' : 'text-fg-1')}>
          {formatClock(totalSeconds)} / {formatClock(targetSeconds)}
        </span>
      </div>
      <span className={cn('studio-bar', over ? 'text-warning' : 'text-primary')} aria-hidden>
        <span style={{ width: `${fill}%` }} />
      </span>
      {cues.length === 0 ? (
        <p className="text-body-sm text-fg-3">{EMPTY_HINT}</p>
      ) : (
        <ol className="flex max-h-64 flex-col gap-3 @[80rem]/content:gap-4 overflow-y-auto overscroll-y-contain @[56rem]/content:max-h-[clamp(15rem,calc(100dvh-32rem),32rem)]" tabIndex={0} aria-label="Jugadas en orden de salida">
          {cues.map((cue, index) => (
            <li key={cue.play.id} className="flex items-center gap-3 py-1 @[80rem]/content:py-2">
              <span className="w-6 shrink-0 text-meta tabular-nums text-fg-3">
                {String(index + 1).padStart(2, '0')}
              </span>
              <span className="flex min-w-0 flex-1 flex-col">
                <span title={playDescription(cue.play)} className="font-display text-label font-bold text-fg-1">
                  <span className="whitespace-nowrap">{playTitle(cue.play, ordinals.get(cue.play.id))}</span>
                  {cue.play.startSeconds !== undefined ? <> · <span className="whitespace-nowrap">{playClock(cue.play.startSeconds)}</span></> : null}
                </span>
                {/* Numbers keep their units: the line may break at the separator, never inside "6.8 s". */}
                <span className="text-label tabular-nums text-fg-2">
                  <span className="whitespace-nowrap">{videoLabel} {formatClock(cue.startAt)}</span> · <span className="whitespace-nowrap">{cue.seconds} s</span>
                </span>
              </span>
              <span className="flex shrink-0 items-center gap-1 text-meta tabular-nums text-fg-2">
                <Crosshair aria-hidden className="size-3.5" />
                {cue.play.kills}
              </span>
            </li>
          ))}
        </ol>
      )}
      {over ? (
        <p className="text-body-sm text-warning">
          {targetSeconds === SHORT_TARGET_SECONDS
            ? 'Pasa del minuto: quita un highlight o acepta un Short más largo.'
            : `Pasa de ${formatClock(targetSeconds)}: quita una jugada o acepta un vídeo más largo.`}
        </p>
      ) : null}
    </section>
  );
}
