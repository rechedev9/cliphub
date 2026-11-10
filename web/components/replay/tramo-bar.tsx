'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react';
import { Check, SlidersHorizontal, Timer } from 'lucide-react';
import { QUICK_TRAMOS, ROUND_CLOCK_SECONDS, WHOLE_ROUND, clockPercent, dragSeconds, edgeBounds, isWholeRound, keyedEdge, moveTramoEdge, nearestEdge, quickTramoOf, roundClock, tramoEnd } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import { FOCUS_RING } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
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
  // How far inside the handle the press landed, so a drag does not jump to the pointer.
  const grab = useRef(0);
  // The owner keeps the tramo in the URL, which answers a beat later: a held key or a fast drag must build on the last move, not on the stale prop.
  const latest = useRef(tramo);
  useEffect(() => {
    latest.current = tramo;
  }, [tramo]);
  const move = (target: number): void => {
    const next = moveTramoEdge(latest.current, edge, target);
    latest.current = next;
    onChange(next);
  };
  const seconds = edge === 'from' ? tramo.from : tramoEnd(tramo);
  const trackOf = (event: PointerEvent<HTMLButtonElement>): DOMRect | undefined => event.currentTarget.parentElement?.getBoundingClientRect();
  const secondsAt = (event: PointerEvent<HTMLButtonElement>): number | null => {
    const track = trackOf(event);
    if (track === undefined) return null;
    const at = dragSeconds({ clientX: event.clientX, left: track.left, width: track.width, grab: grab.current });
    return at === null ? null : (at / ROUND_CLOCK_SECONDS) * duration;
  };
  const bounds = edgeBounds(tramo, edge);
  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>): void => {
    const target = keyedEdge({ tramo: latest.current, edge, key: event.key, shift: event.shiftKey });
    if (target === null) return;
    move(target);
    event.preventDefault();
    event.stopPropagation();
  };
  return (
    <button
      type="button"
      role="slider"
      aria-label={EDGE_LABEL[edge]}
      aria-valuemin={bounds.min}
      aria-valuemax={bounds.max}
      aria-valuenow={seconds}
      aria-valuetext={`${roundClock(seconds)} en el reloj de la ronda`}
      onKeyDown={onKeyDown}
      onPointerDown={(event) => {
        dragging.current = true;
        const track = trackOf(event);
        grab.current = track === undefined ? 0 : event.clientX - (track.left + (seconds / duration) * track.width);
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        const at = dragging.current ? secondsAt(event) : null;
        if (at !== null) move(at);
      }}
      onPointerUp={() => {
        dragging.current = false;
      }}
      onPointerCancel={() => {
        dragging.current = false;
      }}
      style={{ left: `${(seconds / duration) * 100}%` }}
      className={cn(
        'pointer-events-auto absolute inset-y-0 z-10 flex w-3 cursor-ew-resize touch-none items-center justify-center bg-primary outline-none',
        edge === 'from' ? 'rounded-l-sm' : '-translate-x-full rounded-r-sm',
        FOCUS_RING,
      )}
    >
      <span aria-hidden className="h-4 w-0.5 rounded-full bg-primary-foreground/70" />
    </button>
  );
}

/** A moment of the matched rounds on the strip: its second on the round clock and the token of its player's colour. */
export type TramoEvent = { seconds: number; token: string };

/** The clock box and the strip every time control shares: the Tramo bar and the Replay's transport. */
export const CLOCK_BOX = 'flex h-10 shrink-0 items-center gap-1.5 rounded-sm border border-border bg-surface-1 px-2 font-mono text-body-sm tabular-nums text-fg-1';
export const STRIP = 'relative h-10 min-w-0 flex-1 overflow-hidden rounded-sm bg-surface-0 shadow-[inset_0_0_0_1px_var(--border)]';

/** The quick tramos in a small menu beside the start clock, so the strip carries no buttons. */
function TramoPresets({ tramo, onChange, plant }: { tramo: Tramo; onChange: (tramo: Tramo) => void; plant: boolean }): ReactNode {
  const chosen = quickTramoOf(tramo);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Tramos rápidos"
        className={cn('-mr-1 flex size-7 shrink-0 items-center justify-center rounded-xs text-fg-3 outline-none hover:bg-surface-3 hover:text-fg-1', FOCUS_RING)}
      >
        <SlidersHorizontal aria-hidden className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" side="top">
        {QUICK_TRAMOS.filter((quick) => plant || quick.id !== 'plant').map((quick) => (
          <DropdownMenuItem key={quick.id} onSelect={() => onChange(quick.tramo)}>
            <Check aria-hidden className={cn('size-3.5 text-fg-1', chosen !== quick.id && 'invisible')} />
            {quick.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * The time window of the finders and exports, as Skybox EDGE draws it: a clock box with
 * the start, one strip with a line per event in its player's colour and a grip at each
 * end of the window (the strip dims outside it), and a box with the end. The quick
 * tramos sit in the menu beside the start box.
 */
export function TramoBar({
  tramo,
  onChange,
  events = [],
  plant = true,
}: {
  tramo: Tramo;
  onChange: (tramo: Tramo) => void;
  events?: readonly TramoEvent[];
  /** Offer "Desde el plant"; off where every round shares one clock. */
  plant?: boolean;
}): ReactNode {
  const fromPlant = tramo.fromPlant === true;
  const shown: Tramo = fromPlant || isWholeRound(tramo) ? { from: 0, to: ROUND_CLOCK_SECONDS } : tramo;
  const start = clockPercent(shown.from);
  const end = clockPercent(tramoEnd(shown));
  return (
    <div role="group" aria-label="Tramo de la ronda" className="flex w-full items-center gap-1.5">
      <div className={CLOCK_BOX}>
        <Timer aria-hidden className="size-4 text-fg-3" />
        <span aria-label="Inicio del tramo en el reloj">{fromPlant ? 'Plant' : roundClock(shown.from)}</span>
        <TramoPresets tramo={tramo} onChange={onChange} plant={plant} />
      </div>
      <div
        onPointerDown={(event) => {
          // A press on the bare strip pulls the closer grip there; the grips take their own presses.
          if (fromPlant || (event.target instanceof Element && event.target.closest('[role="slider"]') !== null)) return;
          const box = event.currentTarget.getBoundingClientRect();
          const seconds = dragSeconds({ clientX: event.clientX, left: box.left, width: box.width, grab: 0 });
          if (seconds !== null) onChange(moveTramoEdge(shown, nearestEdge(shown, seconds), seconds));
        }}
        className={cn(STRIP, !fromPlant && 'cursor-pointer')}
      >
        {fromPlant ? null : (
          <>
            <div aria-hidden className="pointer-events-none absolute inset-0">
              {events.map((event, index) => (
                <span key={index} className="absolute inset-y-0 w-px opacity-80" style={{ left: `${clockPercent(event.seconds)}%`, background: `var(${event.token})` }} />
              ))}
            </div>
            <div className="pointer-events-none absolute inset-0">
              <div aria-hidden className="absolute inset-y-0 left-0 bg-surface-0/75" style={{ width: `${start}%` }} />
              <div aria-hidden className="absolute inset-y-0 right-0 bg-surface-0/75" style={{ left: `${end}%` }} />
              <div aria-hidden className="absolute inset-y-0 border-y-2 border-primary" style={{ left: `${start}%`, width: `${end - start}%` }} />
              <Handle edge="from" tramo={shown} duration={ROUND_CLOCK_SECONDS} onChange={onChange} />
              <Handle edge="to" tramo={shown} duration={ROUND_CLOCK_SECONDS} onChange={onChange} />
            </div>
          </>
        )}
      </div>
      <div className={CLOCK_BOX}>
        <Timer aria-hidden className="size-4 text-fg-3" />
        <span aria-label="Final del tramo en el reloj">{roundClock(tramoEnd(shown))}</span>
      </div>
    </div>
  );
}
