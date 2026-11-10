'use client';

import type { ReactNode } from 'react';
import type { ClockCount } from '@/lib/analysis/grenade-patterns';

type Stack = readonly { steamid64: string; token: string }[];

/** "Tras la plant": the throws after the bomb went down, which the round clock no longer times. */
export function GrenadeAfterPlant({ bin, stack, names }: { bin: ClockCount; stack: Stack; names: ReadonlyMap<string, string> }): ReactNode {
  if (bin.count === 0) return null;
  return (
    <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-2">
      <span>
        Tras la plant: <span className="font-medium text-fg-1 tabular-nums">{bin.count}</span> {bin.count === 1 ? 'granada' : 'granadas'}
      </span>
      {stack.map((player) => {
        const count = bin.byPlayer.get(player.steamid64) ?? 0;
        return count === 0 ? null : (
          <span key={player.steamid64} className="inline-flex items-center gap-1 tabular-nums">
            <span aria-hidden className="size-2 rounded-full" style={{ background: `var(${player.token})` }} />
            {names.get(player.steamid64) ?? ''} {count}
          </span>
        );
      })}
    </p>
  );
}
