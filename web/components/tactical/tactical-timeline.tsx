'use client';

import type { ReactNode, RefObject } from 'react';
import { Pause, Play, Repeat } from 'lucide-react';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalEvent } from '@/lib/api/tactical';
import { TACTICAL_REPLAY_SPEEDS, roundClockLabelFor } from '@/lib/tactical-timeline';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { eventKindLabel } from '@/lib/tactical-labels';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** Marker colour per event family; kills take the attacker's side colour. */
function markerClass(event: TacticalEvent): string {
  if (event.kind === TACTICAL_EVENT_KINDS.kill) {
    return event.side === TACTICAL_SIDES.t ? 'bg-warning' : 'bg-primary';
  }
  if (event.kind === TACTICAL_EVENT_KINDS.defuse) return 'bg-success';
  if (
    event.kind === TACTICAL_EVENT_KINDS.plant ||
    event.kind === TACTICAL_EVENT_KINDS.explode
  ) {
    return 'bg-destructive';
  }
  return 'bg-muted-foreground';
}

function markerHeight(event: TacticalEvent): string {
  if (event.kind === TACTICAL_EVENT_KINDS.plant) return 'h-full';
  if (event.kind === TACTICAL_EVENT_KINDS.kill) return 'h-3/4';
  return 'h-1/2';
}

/**
 * The transport: a real `<input type="range">` an analyst can tab to and drag,
 * with the round's shape drawn behind it — the freeze-time band, then a mark for
 * every event. The canvas mirrors this control; it is never the other way round,
 * so the replay stays operable from the keyboard alone.
 */
export function TacticalTimelineBar({
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
}): ReactNode {
  const freezeFraction =
    timeline.durationSeconds > 0 ? timeline.freezeSeconds / timeline.durationSeconds : 0;

  return (
    <div className="flex flex-col gap-3">
      <div className="relative h-10">
        <div className="pointer-events-none absolute inset-x-0 top-1/2 h-3 -translate-y-1/2 overflow-hidden rounded-sm border border-border/70 bg-background/60">
          <div
            className="absolute inset-y-0 left-0 border-r border-primary/40 bg-muted/70"
            style={{ width: `${freezeFraction * 100}%` }}
            aria-hidden
          />
          {events.map((entry, index) => (
            <span
              // See tactical-event-list.tsx: kind/tick/slots are not unique,
              // so the index completes the key for the same ordered slice.
              key={`${entry.event.kind}-${entry.event.tick}-${entry.event.actor_slot ?? 'x'}-${entry.event.target_slot ?? 'x'}-${index}`}
              title={eventKindLabel(entry.event.kind)}
              className={cn(
                'absolute top-1/2 w-[2px] -translate-x-1/2 -translate-y-1/2 rounded-full',
                markerHeight(entry.event),
                markerClass(entry.event),
              )}
              style={{ left: `${entry.fraction * 100}%` }}
              aria-hidden
            />
          ))}
        </div>

        <input
          ref={scrubRef}
          type="range"
          min={0}
          max={timeline.durationSeconds}
          step={0.05}
          defaultValue={0}
          onChange={(event) => onSeek(Number(event.target.value))}
          aria-label="Posición de la repetición"
          className={cn(
            'absolute inset-0 h-10 w-full cursor-pointer appearance-none bg-transparent outline-none',
            '[&::-webkit-slider-runnable-track]:h-10 [&::-webkit-slider-runnable-track]:bg-transparent',
            '[&::-webkit-slider-thumb]:h-6 [&::-webkit-slider-thumb]:w-[6px] [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-[2px] [&::-webkit-slider-thumb]:border-0 [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:shadow-[0_0_10px_var(--primary)]',
            '[&::-moz-range-track]:h-10 [&::-moz-range-track]:bg-transparent',
            '[&::-moz-range-thumb]:h-6 [&::-moz-range-thumb]:w-[6px] [&::-moz-range-thumb]:rounded-[2px] [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-primary',
            'focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
          )}
        />
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Button
          type="button"
          size="icon-sm"
          onClick={onTogglePlay}
          aria-label={playing ? 'Pausar (espacio)' : 'Reproducir (espacio)'}
        >
          {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
        </Button>

        <span
          ref={clockRef}
          className="min-w-[86px] font-[family-name:var(--font-mono)] text-sm tabular-nums text-foreground"
        >
          {roundClockLabelFor(timeline, 0)}
        </span>

        <div className="flex items-center gap-1" role="group" aria-label="Velocidad de reproducción">
          {TACTICAL_REPLAY_SPEEDS.map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => onSpeedChange(option)}
              aria-pressed={speed === option}
              className={cn(
                'h-10 min-w-11 rounded-md border px-2 font-[family-name:var(--font-mono)] text-xs tabular-nums outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
                speed === option
                  ? 'border-primary bg-primary font-semibold text-primary-foreground'
                  : 'border-primary/25 bg-background/45 text-muted-foreground hover:border-primary/55 hover:bg-primary/10 hover:text-foreground',
              )}
            >
              {option}×
            </button>
          ))}
        </div>

        <Button
          type="button"
          size="icon-sm"
          variant={loop ? 'default' : 'outline'}
          onClick={onToggleLoop}
          aria-pressed={loop}
          aria-label="Repetir en bucle"
        >
          <Repeat aria-hidden />
        </Button>

        <span className="ml-auto font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
          espacio · ←/→ 1 s · ⇧←/→ evento
        </span>
      </div>
    </div>
  );
}
