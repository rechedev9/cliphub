'use client';

import type { ReactNode } from 'react';
import { TACTICAL_REPLAY_SPEEDS } from '@/lib/tactical-timeline';
import { formatRate } from '@/lib/format';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** The clock box and the strip every time control shares, as Skybox EDGE draws them. */
export const CLOCK_BOX = 'flex h-10 shrink-0 items-center gap-1.5 rounded-sm border border-border bg-surface-1 px-2 font-mono text-body-sm tabular-nums text-fg-1';
export const STRIP = 'relative h-10 min-w-0 flex-1 overflow-hidden rounded-sm bg-surface-0 shadow-[inset_0_0_0_1px_var(--border)]';
/** A playing strip carries the green frame of the tramo window. */
export const PLAYING_STRIP = cn(STRIP, 'shadow-[inset_0_0_0_2px_var(--primary)]');

/** A range input laid over a strip: transparent track, a thin green bar as the playhead. */
export const SCRUB_INPUT = cn(
  'absolute inset-0 h-10 w-full cursor-pointer appearance-none bg-transparent outline-none',
  '[&::-webkit-slider-runnable-track]:h-10 [&::-webkit-slider-runnable-track]:bg-transparent',
  '[&::-webkit-slider-thumb]:h-10 [&::-webkit-slider-thumb]:w-1.5 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-[2px] [&::-webkit-slider-thumb]:bg-primary',
  '[&::-moz-range-track]:h-10 [&::-moz-range-track]:bg-transparent',
  '[&::-moz-range-thumb]:h-10 [&::-moz-range-thumb]:w-1.5 [&::-moz-range-thumb]:rounded-[2px] [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-primary',
  FOCUS_RING,
);

/** The playback speeds as one pressed-button group. */
export function SpeedPicker({ speed, onChange, min = 0 }: { speed: number; onChange: (speed: number) => void; min?: number }): ReactNode {
  return (
    <div className="flex items-center rounded-md border border-border-strong bg-surface-1 p-0.5" role="group" aria-label="Velocidad">
      {TACTICAL_REPLAY_SPEEDS.filter((option) => option >= min).map((option) => (
        <button
          key={option}
          type="button"
          onClick={() => onChange(option)}
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
  );
}
