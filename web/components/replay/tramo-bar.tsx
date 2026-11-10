'use client';

import { useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react';
import { QUICK_TRAMOS, ROUND_CLOCK_SECONDS, WHOLE_ROUND, isWholeRound, moveTramoEdge, quickTramoOf, roundClock, tramoEnd, tramoLabel } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import { FOCUS_RING } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { cn } from '@/lib/utils';

/** A tramo and how to change it; the Replay keeps it in the URL and hands it to each form. */
export type TramoControl = { tramo: Tramo; onChange: (tramo: Tramo) => void };

/** The owner's tramo when it gives one, else one this view keeps for itself. */
export function useTramo(control: TramoControl | undefined): TramoControl {
  const [own, setOwn] = useState<Tramo>(WHOLE_ROUND);
  return control ?? { tramo: own, onChange: setOwn };
}

const EDGE_LABEL = { from: 'Inicio del tramo', to: 'Final del tramo' } as const;

function Handle({
  edge,
  tramo,
  duration,
  onChange,
}: {
  edge: 'from' | 'to';
  tramo: Tramo;
  duration: number;
  onChange: (tramo: Tramo) => void;
}): ReactNode {
  const dragging = useRef(false);
  const seconds = edge === 'from' ? tramo.from : tramoEnd(tramo);
  const secondsAt = (event: PointerEvent<HTMLButtonElement>): number | null => {
    const track = event.currentTarget.parentElement?.getBoundingClientRect();
    if (track === undefined || track.width === 0) return null;
    return ((event.clientX - track.left) / track.width) * duration;
  };
  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>): void => {
    const step = event.shiftKey ? 5 : 1;
    if (event.key === 'ArrowLeft') onChange(moveTramoEdge(tramo, edge, seconds - step));
    else if (event.key === 'ArrowRight') onChange(moveTramoEdge(tramo, edge, seconds + step));
    else return;
    event.preventDefault();
    event.stopPropagation();
  };
  return (
    <button
      type="button"
      role="slider"
      aria-label={EDGE_LABEL[edge]}
      aria-valuemin={0}
      aria-valuemax={ROUND_CLOCK_SECONDS}
      aria-valuenow={seconds}
      aria-valuetext={roundClock(seconds)}
      onKeyDown={onKeyDown}
      onPointerDown={(event) => {
        dragging.current = true;
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        const at = dragging.current ? secondsAt(event) : null;
        if (at !== null) onChange(moveTramoEdge(tramo, edge, at));
      }}
      onPointerUp={() => {
        dragging.current = false;
      }}
      onPointerCancel={() => {
        dragging.current = false;
      }}
      style={{ left: `${(seconds / duration) * 100}%` }}
      className={cn(
        'pointer-events-auto absolute inset-y-0 z-10 w-3 -translate-x-1/2 cursor-ew-resize touch-none outline-none',
        'after:absolute after:inset-y-0 after:left-1/2 after:w-1 after:-translate-x-1/2 after:rounded-[1px] after:bg-primary',
        FOCUS_RING,
      )}
    />
  );
}

/**
 * The tramo over a track: a translucent violet bracket with a handle at each
 * end. `duration` is the track's length in seconds since freeze end. It lays
 * itself over its positioned parent and lets clicks through, except on the handles.
 */
export function TramoBracket({ tramo, duration, onChange }: { tramo: Tramo; duration: number; onChange: (tramo: Tramo) => void }): ReactNode {
  if (tramo.fromPlant === true || duration <= 0) return null;
  const from = Math.min(tramo.from, duration);
  const to = Math.min(tramoEnd(tramo), duration);
  return (
    <div className="pointer-events-none absolute inset-0">
      <div className="absolute inset-y-0 bg-primary/25" style={{ left: `${(from / duration) * 100}%`, width: `${((to - from) / duration) * 100}%` }} aria-hidden />
      <Handle edge="from" tramo={tramo} duration={duration} onChange={onChange} />
      <Handle edge="to" tramo={tramo} duration={duration} onChange={onChange} />
    </div>
  );
}

/** The quick tramos, the same four wherever a tramo is chosen. */
export function QuickTramos({ tramo, onChange, plant = true }: { tramo: Tramo; onChange: (tramo: Tramo) => void; /** Offer "Desde el plant"; off where every round shares one clock. */ plant?: boolean }): ReactNode {
  const chosen = quickTramoOf(tramo);
  const quicks = QUICK_TRAMOS.filter((quick) => plant || quick.id !== 'plant');
  return (
    <Segmented
      label="Tramo"
      value={chosen ?? 'custom'}
      options={[...quicks.map((quick) => ({ value: quick.id, label: quick.label })), ...(chosen === null ? [{ value: 'custom' as const, label: 'A medida' }] : [])]}
      onChange={(id) => {
        const quick = QUICK_TRAMOS.find((entry) => entry.id === id);
        if (quick !== undefined) onChange(quick.tramo);
      }}
    />
  );
}

/**
 * The tramo of the forms that count rounds instead of playing them: the round
 * clock from 1:55 to 0:00 as a track, the bracket over it and the quick tramos.
 */
export function TramoBar({
  tramo,
  onChange,
  marks,
  compact = false,
}: {
  tramo: Tramo;
  onChange: (tramo: Tramo) => void;
  /** Drawn in the track, such as a histogram. */
  marks?: ReactNode;
  /** One row (label, track, quick tramos) for a panel under the map, where height is scarce. */
  compact?: boolean;
}): ReactNode {
  const shown: Tramo = tramo.fromPlant === true || isWholeRound(tramo) ? { from: 0, to: ROUND_CLOCK_SECONDS } : tramo;
  const label = <span className="shrink-0 text-body-sm font-medium text-fg-1">{tramoLabel(tramo)}</span>;
  const quick = <QuickTramos tramo={tramo} onChange={onChange} />;
  const track = (
    <div className={cn('flex items-center gap-2', compact && 'min-w-[12rem] flex-1')}>
      <span className="font-mono text-meta tabular-nums text-fg-3">{roundClock(0)}</span>
      <div className={cn('relative min-w-0 flex-1 overflow-hidden rounded-sm bg-surface-0 shadow-[inset_0_0_0_1px_var(--border)]', compact ? 'h-6' : 'h-7')}>
        {marks}
        {tramo.fromPlant === true ? null : <TramoBracket tramo={shown} duration={ROUND_CLOCK_SECONDS} onChange={onChange} />}
      </div>
      <span className="font-mono text-meta tabular-nums text-fg-3">{roundClock(ROUND_CLOCK_SECONDS)}</span>
    </div>
  );
  if (compact) {
    return (
      <div role="group" aria-label="Tramo de la ronda" className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        {label}
        {track}
        {quick}
      </div>
    );
  }
  return (
    <div role="group" aria-label="Tramo de la ronda" className="flex flex-col gap-2 rounded-md border border-border bg-surface-2 px-3 py-2.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        {label}
        {quick}
      </div>
      {track}
    </div>
  );
}
