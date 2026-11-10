'use client';

import { useEffect, useRef } from 'react';
import type { ReactNode, RefObject } from 'react';
import { Pause, Play, Repeat, Timer } from 'lucide-react';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalEvent, TacticalTimeout } from '@/lib/api/tactical';
import { isBombKind } from '@/lib/replay/event-kinds';
import { postPlantSpan, timeoutSpans } from '@/lib/replay/map-hud';
import { SCRUB_STEP_SECONDS } from '@/lib/replay/playback';
import { roundClockLabelFor } from '@/lib/tactical-timeline';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { TramoBar } from './tramo-bar';
import type { TramoControl } from './tramo-bar';
import { CLOCK_BOX, PLAYING_STRIP, SCRUB_INPUT, SpeedPicker } from './time-strip';

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
  tramo,
  timeouts = [],
}: {
  timeline: RoundTimeline;
  events: readonly TimelineEvent[];
  /** The round's timeouts, drawn as bands in its freeze time while one round plays. */
  timeouts?: readonly TacticalTimeout[];
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
  const span = (from: number, to: number): { left: string; width: string } => ({
    left: `${(from / timeline.durationSeconds) * 100}%`,
    width: `${((to - from) / timeline.durationSeconds) * 100}%`,
  });
  return (
    <div className="grid grid-cols-[auto_auto_minmax(0,1fr)_auto_auto] items-center gap-x-1.5 gap-y-2">
      <Button type="button" size="icon-sm" variant="outline" className="mr-1.5 size-10" onClick={onTogglePlay} aria-label={playing ? 'Pausar' : 'Reproducir'} title="Espacio: reproducir o pausar · ←/→: 1 s · Mayús + ←/→: evento · [ ] o J K: ronda">
        {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
      </Button>
      <div className={CLOCK_BOX}>
        <Timer aria-hidden className="size-4 text-fg-3" />
        <span ref={clockRef} className="min-w-[7ch] text-center">
          {roundClockLabelFor(timeline, 0)}
        </span>
      </div>
      <div className={PLAYING_STRIP}>
        <div className="pointer-events-none absolute inset-0" aria-hidden>
          <div className="absolute inset-y-0 left-0 bg-surface-3/60" style={{ width: `${freeze * 100}%` }} />
          {timeoutSpans(timeouts, timeline).map((timeout) => (
            <div key={`${timeout.side}-${timeout.from}`} className={cn('absolute inset-y-0', timeout.side === TACTICAL_SIDES.t ? 'bg-side-t/20' : 'bg-side-ct/20')} style={span(timeout.from, timeout.to)} />
          ))}
          {postPlant === null ? null : <div className="absolute inset-y-0 border-r-2 border-fg-1 bg-destructive/25" style={span(postPlant.from, postPlant.to)} />}
          {events.map((entry, index) => (
            <span
              key={`${entry.event.kind}-${entry.event.tick}-${entry.event.actor_slot ?? 'x'}-${index}`}
              className={cn('absolute bottom-0 w-px', tickClass(entry.event))}
              style={{ left: `${entry.fraction * 100}%` }}
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
            className={SCRUB_INPUT}
          />
      </div>
      <div className={CLOCK_BOX}>
        <Timer aria-hidden className="size-4 text-fg-3" />
        {roundClockLabelFor(timeline, timeline.durationSeconds)}
      </div>
      <div className="ml-1.5 flex h-10 items-center gap-1">
        <SpeedPicker speed={speed} onChange={onSpeedChange} />
        <Button type="button" size="icon-xs" variant={loop ? 'outline-primary' : 'ghost'} onClick={onToggleLoop} aria-pressed={loop} aria-label="Repetir en bucle" className="size-8">
          <Repeat aria-hidden />
        </Button>
      </div>
      {tramo === undefined ? null : (
        <div className="col-span-full">
          <TramoBar tramo={tramo.tramo} onChange={tramo.onChange} plant={false} />
        </div>
      )}
    </div>
  );
}
