import type { ReactNode } from 'react';
import { Crosshair } from 'lucide-react';
import { formatClock, type SelectionCue } from '@/lib/produce/short-selection';
import { playClock, playDescription } from '@/lib/produce/play-details';
import { playTitle, type RoundOrdinal } from '@/lib/produce/round-ordinals';

const STORYBOARD_TITLE = 'Guion del Short';
const EMPTY_HINT = 'Las jugadas elegidas aparecen aquí en el orden en que saldrán.';

/** The running order of the Short: real cues from the plan, not a placeholder frame. */
export function ShortStoryboard({
  cues,
  ordinals,
  title = STORYBOARD_TITLE,
  videoLabel = 'Short',
}: {
  cues: SelectionCue[];
  /** Ordinals computed over every play of the match, so they match the play list. */
  ordinals: ReadonlyMap<string, RoundOrdinal>;
  title?: string;
  /** What the cue clock is measured in: "Short 0:12". */
  videoLabel?: string;
}): ReactNode {
  return (
    <section aria-label={title} className="flex w-full min-w-0 flex-col gap-2 rounded-2xl bg-surface-2 p-4">
      <h2 className="font-display text-body font-bold text-fg-1">{title}</h2>
      {cues.length === 0 ? (
        <p className="text-body-sm text-fg-3">{EMPTY_HINT}</p>
      ) : (
        <ol className="flex max-h-64 flex-col gap-2 overflow-y-auto overscroll-y-contain" tabIndex={0} aria-label="Jugadas en orden de salida">
          {cues.map((cue, index) => (
            <li key={cue.play.id} className="flex items-center gap-3 py-1">
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
    </section>
  );
}
