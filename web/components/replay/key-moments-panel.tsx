'use client';

import type { ReactNode } from 'react';
import { ArrowLeftRight, Bomb, Crosshair, Swords, Wrench } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import { KEY_MOMENT_LABELS } from '@/lib/replay/key-moments';
import type { KeyMomentKind } from '@/lib/replay/key-moments';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export const MOMENT_ICONS: Record<KeyMomentKind, ReactNode> = {
  opening: <Crosshair aria-hidden />,
  trade: <ArrowLeftRight aria-hidden />,
  plant: <Bomb aria-hidden />,
  defuse: <Wrench aria-hidden />,
  clutch: <Swords aria-hidden />,
};

export function momentSideClass(side: TacticalSide | null): string {
  if (side === TACTICAL_SIDES.ct) return 'text-side-ct';
  if (side === TACTICAL_SIDES.t) return 'text-side-t';
  return 'text-fg-2';
}

/** A moment the timeline marks and the list jumps to; `at` is its fraction of the track. */
export type ShownMoment = { key: string; kind: KeyMomentKind; label: string; side: TacticalSide | null; clock: string; at: number; onJump: () => void };

/** The round's key moments in order, each a button that jumps the replay to just before it. */
export function KeyMomentList({ moments }: { moments: readonly ShownMoment[] }): ReactNode {
  if (moments.length === 0) return <p className="text-meta text-fg-3">Sin primera baja, plant ni clutch en esta ronda.</p>;
  return (
    <ul className="flex flex-col">
      {moments.map((moment) => (
        <li key={moment.key}>
          <button
            type="button"
            onClick={moment.onJump}
            className={cn(
              'flex h-7 w-full items-center gap-2 rounded-sm px-1.5 text-left text-body-sm text-fg-2 outline-none hover:bg-surface-3 hover:text-fg-1 [&_svg]:size-3.5 [&_svg]:shrink-0',
              FOCUS_RING,
            )}
          >
            <span className={momentSideClass(moment.side)}>{MOMENT_ICONS[moment.kind]}</span>
            <span className="min-w-0 flex-1 truncate">
              <span className="text-fg-1">{KEY_MOMENT_LABELS[moment.kind]}</span>
              <span className="text-fg-3"> · {moment.label}</span>
            </span>
            <span className="text-meta tabular-nums text-fg-3">{moment.clock}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}

/** The same moments as small marks under the track, on its scale. */
export function KeyMomentMarks({ moments }: { moments: readonly ShownMoment[] }): ReactNode {
  if (moments.length === 0) return null;
  return (
    <div className="relative mx-[3px] h-5" role="group" aria-label="Momentos clave">
      {moments.map((moment) => (
        <button
          key={moment.key}
          type="button"
          onClick={moment.onJump}
          title={`${KEY_MOMENT_LABELS[moment.kind]} · ${moment.label} · ${moment.clock}`}
          aria-label={`${KEY_MOMENT_LABELS[moment.kind]}, ${moment.label}, ${moment.clock}`}
          style={{ left: `${moment.at * 100}%` }}
          className={cn(
            'absolute top-0 grid size-5 -translate-x-1/2 place-items-center rounded-sm border border-border-strong bg-surface-2 outline-none hover:bg-surface-4 [&_svg]:size-3',
            FOCUS_RING,
            momentSideClass(moment.side),
          )}
        >
          {MOMENT_ICONS[moment.kind]}
        </button>
      ))}
    </div>
  );
}
