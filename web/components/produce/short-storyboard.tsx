import type { ReactNode } from 'react';
import { Crosshair } from 'lucide-react';
import { formatClock, SHORT_TARGET_SECONDS, type SelectionCue } from '@/lib/produce/short-selection';
import { cn } from '@/lib/utils';
import { playClock, playDescription } from '@/lib/produce/play-details';

const STORYBOARD_TITLE = 'Guion del Short';
const EMPTY_HINT = 'Los highlights elegidos aparecen aquí en el orden en que saldrán.';

/** The running order of the Short: real cues from the plan, not a placeholder frame. */
export function ShortStoryboard({
  cues,
  totalSeconds,
}: {
  cues: SelectionCue[];
  totalSeconds: number;
}): ReactNode {
  const over = totalSeconds > SHORT_TARGET_SECONDS;
  const fill = Math.min(100, (totalSeconds / SHORT_TARGET_SECONDS) * 100);
  return (
    <section aria-label={STORYBOARD_TITLE} className="studio-panel flex min-w-0 flex-col gap-3 p-4 @[80rem]/content:gap-4 @[80rem]/content:p-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-meta uppercase tracking-ultra text-fg-3">{STORYBOARD_TITLE}</span>
        <span className={cn('font-mono text-meta tabular-nums uppercase tracking-wider', over ? 'text-warning' : 'text-primary')}>
          {formatClock(totalSeconds)} / {formatClock(SHORT_TARGET_SECONDS)}
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
              <span className="w-6 shrink-0 font-mono text-meta tabular-nums text-primary">
                {String(index + 1).padStart(2, '0')}
              </span>
              <span className="flex min-w-0 flex-1 flex-col">
                <span title={playDescription(cue.play)} className="font-display text-label font-bold text-fg-1">Ronda {cue.play.round}{cue.play.startSeconds !== undefined ? ` · ${playClock(cue.play.startSeconds)}` : ''}</span>
                <span className="font-mono text-label tabular-nums text-fg-2">
                  Short {formatClock(cue.startAt)} · {cue.seconds} s
                </span>
              </span>
              <span className="flex shrink-0 items-center gap-1 font-mono text-meta tabular-nums text-fg-2">
                <Crosshair aria-hidden className="size-3.5" />
                {cue.play.kills}
              </span>
            </li>
          ))}
        </ol>
      )}
      {over ? (
        <p className="text-body-sm text-warning">Pasa del minuto: quita un highlight o acepta un Short más largo.</p>
      ) : null}
    </section>
  );
}
