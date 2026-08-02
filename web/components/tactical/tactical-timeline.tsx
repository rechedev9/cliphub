'use client';

import type { ReactNode, RefObject } from 'react';
import { Pause, Play, Repeat } from 'lucide-react';
import { TACTICAL_EVENT_KINDS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalEvent } from '@/lib/api/tactical';
import { TACTICAL_REPLAY_SPEEDS, roundClockLabelFor } from '@/lib/tactical-timeline';
import type { RoundTimeline, TimelineEvent } from '@/lib/tactical-timeline';
import { eventKindLabel } from '@/lib/tactical-labels';
import { Button, FOCUS_RING } from '@/components/ui/button';
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

/** Seated on the rail's baseline, so the three steps read as one ranked family. */
function markerHeight(event: TacticalEvent): string {
  if (event.kind === TACTICAL_EVENT_KINDS.plant) return 'h-full';
  if (event.kind === TACTICAL_EVENT_KINDS.kill) return 'h-[62%]';
  return 'h-[38%]';
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
        {/* A well, so it takes --surface-0 and the WCAG control edge; inset by
            the thumb's half-width so 0-100% matches the playhead's travel. */}
        <div className="studio-rim pointer-events-none absolute inset-x-[3px] top-1/2 h-4 -translate-y-1/2 overflow-hidden rounded-sm border border-border-strong bg-surface-0">
          <div
            className="absolute inset-y-0 left-0 border-r border-primary/40 bg-surface-3"
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
                'absolute bottom-0 w-[3px] -translate-x-1/2 rounded-t-[1px]',
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
            '[&::-webkit-slider-thumb]:h-6 [&::-webkit-slider-thumb]:w-[6px] [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-[2px] [&::-webkit-slider-thumb]:border-0 [&::-webkit-slider-thumb]:bg-primary [&::-webkit-slider-thumb]:shadow-[var(--glow-primary-sm)]',
            '[&::-moz-range-track]:h-10 [&::-moz-range-track]:bg-transparent',
            '[&::-moz-range-thumb]:h-6 [&::-moz-range-thumb]:w-[6px] [&::-moz-range-thumb]:rounded-[2px] [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-primary',
            FOCUS_RING,
          )}
        />
      </div>

      <div className="flex flex-wrap items-center gap-3">
        {/* The only solid cyan fill in the row, and the largest control in it. */}
        <Button
          type="button"
          size="icon"
          onClick={onTogglePlay}
          aria-label={playing ? 'Pausar (espacio)' : 'Reproducir (espacio)'}
        >
          {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
        </Button>

        {/* Right-aligned so the leading sign grows leftward instead of shifting
            every digit; 8ch in a mono face fits the widest label. */}
        <span ref={clockRef} className="w-[8ch] text-right font-mono text-body tabular-nums text-fg-1">
          {roundClockLabelFor(timeline, 0)}
        </span>

        <div className="flex items-center gap-1" role="group" aria-label="Velocidad de reproducción">
          {TACTICAL_REPLAY_SPEEDS.map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => onSpeedChange(option)}
              aria-pressed={speed === option}
              // Same resting/active contract as the square filter chips one
              // panel above (STUDIO_FILTER_CHIP_CLASS), at the same 44px.
              className={cn(
                'h-11 min-w-11 border px-3 font-mono text-meta tabular-nums outline-none transition-colors duration-(--dur-fast) ease-standard',
                FOCUS_RING,
                speed === option
                  ? 'border-primary bg-primary font-semibold text-primary-foreground shadow-[var(--elev-1),var(--glow-primary-sm)]'
                  : 'border-border-strong bg-surface-2 text-fg-2 hover:border-border-accent hover:bg-surface-3 hover:text-fg-1',
              )}
            >
              {option}×
            </button>
          ))}
        </div>

        {/* An engaged toggle, not a second CTA. */}
        <Button
          type="button"
          size="icon-sm"
          variant={loop ? 'outline-primary' : 'outline'}
          onClick={onToggleLoop}
          aria-pressed={loop}
          aria-label="Repetir en bucle"
        >
          <Repeat aria-hidden />
        </Button>

        <span className="ml-auto font-mono text-meta uppercase tracking-wider text-fg-3">
          espacio · ←/→ 1 s · ⇧←/→ evento
        </span>
      </div>
    </div>
  );
}
