'use client';

import { useEffect, useRef } from 'react';
import type { ReactNode, RefObject } from 'react';
import { Pause, Play, Repeat } from 'lucide-react';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalEvent } from '@/lib/api/tactical';
import { isBombKind } from '@/lib/replay/layers';
import { postPlantSpan } from '@/lib/replay/map-hud';
import { SCRUB_STEP_SECONDS } from '@/lib/replay/playback';
import { TACTICAL_REPLAY_SPEEDS, roundClockLabelFor } from '@/lib/tactical-timeline';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { eventKindLabel } from '@/lib/tactical-labels';
import { formatRate } from '@/lib/format';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { isWholeRound, tramoLabel } from '@/lib/replay/tramo';
import { QuickTramos, TramoBracket } from './tramo-bar';
import type { TramoControl } from './tramo-bar';

/** Kills in the killer's colour, the bomb in white, grenades as short grey ticks. */
function tickClass(event: TacticalEvent): string {
  if (event.kind === TACTICAL_EVENT_KINDS.kill) return cn('h-full', event.side === TACTICAL_SIDES.t ? 'bg-side-t' : 'bg-side-ct');
  if (isBombKind(event.kind)) return 'h-full bg-fg-1';
  return 'h-[40%] bg-fg-4';
}

/**
 * The transport in one row: play, the clock, the round as a track with a tick
 * per event and the scrubber over it, the speeds and the loop. The scrubber is
 * a real range input, so the replay works from the keyboard alone.
 */
export function ReplayTimeline({
  timeline,
  events,
  playing,
  speed,
  loop,
  scrubRef,
  clockRef,
  onTogglePlay,
  onSpeedChange,
  onToggleLoop,
  onSeek,
  onScrubbingChange,
  below,
  tramo,
}: {
  timeline: RoundTimeline;
  events: readonly TimelineEvent[];
  playing: boolean;
  speed: number;
  loop: boolean;
  scrubRef: RefObject<HTMLInputElement | null>;
  clockRef: RefObject<HTMLSpanElement | null>;
  onTogglePlay: () => void;
  onSpeedChange: (speed: number) => void;
  onToggleLoop: () => void;
  onSeek: (seconds: number) => void;
  /** The scrubber is pressed or let go; the release can land outside it, even outside the window. */
  onScrubbingChange: (scrubbing: boolean) => void;
  /** Drawn under the track on its scale, such as the win probability. */
  below?: ReactNode;
  /** The span the superposed rounds play in, drawn over the track; absent while one round plays. */
  tramo?: TramoControl;
}): ReactNode {
  const releaseRef = useRef<(() => void) | null>(null);
  const scrubbingChangeRef = useRef(onScrubbingChange);
  scrubbingChangeRef.current = onScrubbingChange;
  useEffect(() => () => releaseRef.current?.(), []);
  const startScrub = (): void => {
    if (releaseRef.current !== null) return;
    const release = (): void => {
      window.removeEventListener('pointerup', release);
      window.removeEventListener('pointercancel', release);
      window.removeEventListener('blur', release);
      releaseRef.current = null;
      scrubbingChangeRef.current(false);
    };
    window.addEventListener('pointerup', release);
    window.addEventListener('pointercancel', release);
    window.addEventListener('blur', release);
    releaseRef.current = release;
    scrubbingChangeRef.current(true);
  };
  const freeze = timeline.durationSeconds > 0 ? timeline.freezeSeconds / timeline.durationSeconds : 0;
  const postPlant = timeline.durationSeconds > 0 ? postPlantSpan(events, timeline.durationSeconds) : null;
  return (
    <div className="grid grid-cols-[auto_auto_minmax(0,1fr)_auto] items-start gap-x-3">
      <Button type="button" size="icon-sm" variant="outline" onClick={onTogglePlay} aria-label={playing ? 'Pausar' : 'Reproducir'} title="Espacio: reproducir o pausar · ←/→: 1 s · Mayús + ←/→: evento · [ ]: ronda">
        {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
      </Button>
      <span ref={clockRef} className="flex h-10 w-[6ch] items-center justify-end font-display text-title font-semibold tabular-nums text-fg-1">
        {roundClockLabelFor(timeline, 0)}
      </span>
      <div className="flex min-w-0 flex-col gap-1">
        <div className="relative h-10">
          <div className="pointer-events-none absolute inset-x-[3px] top-1/2 h-5 -translate-y-1/2 overflow-hidden rounded-sm bg-surface-0 shadow-[inset_0_0_0_1px_var(--border)]">
            <div className="absolute inset-y-0 left-0 bg-surface-3" style={{ width: `${freeze * 100}%` }} aria-hidden />
            {postPlant === null ? null : (
              <div
                className="absolute inset-y-0 overflow-hidden bg-destructive/20 pl-1 text-meta leading-5 font-semibold text-destructive"
                style={{ left: `${(postPlant.from / timeline.durationSeconds) * 100}%`, width: `${((postPlant.to - postPlant.from) / timeline.durationSeconds) * 100}%` }}
                aria-hidden
              >
                Bomba
              </div>
            )}
            {events.map((entry, index) => (
              <span
                key={`${entry.event.kind}-${entry.event.tick}-${entry.event.actor_slot ?? 'x'}-${index}`}
                className={cn('absolute bottom-0 w-0.5 -translate-x-1/2', tickClass(entry.event))}
                style={{ left: `${entry.fraction * 100}%` }}
                title={eventKindLabel(entry.event.kind)}
                aria-hidden
              />
            ))}
          </div>
          <input
            ref={scrubRef}
            type="range"
            min={0}
            max={timeline.durationSeconds}
            step={SCRUB_STEP_SECONDS}
            defaultValue={0}
            onPointerDown={(event) => {
              if (event.button === 0) startScrub();
            }}
            onChange={(event) => onSeek(Number(event.target.value))}
            aria-label="Posición de la reproducción"
            className={cn(
              'absolute inset-0 h-10 w-full cursor-pointer appearance-none bg-transparent outline-none',
              '[&::-webkit-slider-runnable-track]:h-10 [&::-webkit-slider-runnable-track]:bg-transparent',
              '[&::-webkit-slider-thumb]:mt-[5px] [&::-webkit-slider-thumb]:h-[30px] [&::-webkit-slider-thumb]:w-1.5 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-[2px] [&::-webkit-slider-thumb]:bg-fg-1 [&::-webkit-slider-thumb]:shadow-[0_0_0_2px_var(--surface-0)]',
              '[&::-moz-range-track]:h-10 [&::-moz-range-track]:bg-transparent',
              '[&::-moz-range-thumb]:h-[30px] [&::-moz-range-thumb]:w-1.5 [&::-moz-range-thumb]:rounded-[2px] [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-fg-1',
              FOCUS_RING,
            )}
          />
          {tramo === undefined ? null : (
            <div className="pointer-events-none absolute inset-x-[3px] top-1/2 h-5 -translate-y-1/2">
              <TramoBracket tramo={isWholeRound(tramo.tramo) ? { from: 0, to: timeline.durationSeconds } : tramo.tramo} duration={timeline.durationSeconds} onChange={tramo.onChange} />
            </div>
          )}
        </div>
        {tramo === undefined ? null : (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-meta font-medium text-fg-1">{tramoLabel(tramo.tramo)}</span>
            <QuickTramos tramo={tramo.tramo} onChange={tramo.onChange} plant={false} />
          </div>
        )}
        {below}
      </div>
      <div className="flex h-10 items-center gap-1">
        <div className="flex items-center rounded-md border border-border-strong bg-surface-1 p-0.5" role="group" aria-label="Velocidad">
          {TACTICAL_REPLAY_SPEEDS.map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => onSpeedChange(option)}
              aria-pressed={speed === option}
              className={cn(
                'h-7 min-w-9 rounded-sm px-1.5 text-meta tabular-nums outline-none transition-[background-color,color] duration-(--dur-fast)',
                FOCUS_RING,
                speed === option ? 'bg-surface-4 font-semibold text-fg-1 shadow-[inset_0_-2px_0_0_var(--primary)]' : 'text-fg-2 hover:text-fg-1',
              )}
            >
              {formatRate(option)}
            </button>
          ))}
        </div>
        <Button type="button" size="icon-xs" variant={loop ? 'outline-primary' : 'ghost'} onClick={onToggleLoop} aria-pressed={loop} aria-label="Repetir en bucle" className="size-8">
          <Repeat aria-hidden />
        </Button>
      </div>
    </div>
  );
}
