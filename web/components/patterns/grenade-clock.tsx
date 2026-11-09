'use client';

import type { ReactNode } from 'react';
import type { ClockBin } from '@/lib/analysis/grenade-patterns';
import { roundClock } from '@/lib/replay/tramo';

/**
 * When the grenades go off over the round clock, as bars under the tramo; with
 * players chosen each bar stacks them in their colours.
 */
export function GrenadeClockMarks({ bins, stack }: { bins: readonly ClockBin[]; stack: readonly { steamid64: string; token: string }[] }): ReactNode {
  const tallest = bins.reduce((top, bin) => Math.max(top, bin.count), 0);
  return (
    <div className="absolute inset-0 flex items-end gap-px" role="img" aria-label="Cuándo explotan las granadas a lo largo de la ronda">
      {bins.map((bin) => (
        <span
          key={bin.start}
          title={`${roundClock(bin.start)} a ${roundClock(bin.end)}: ${bin.count} ${bin.count === 1 ? 'granada' : 'granadas'}`}
          className="flex flex-1 flex-col-reverse overflow-hidden rounded-t-xs"
          style={{ height: tallest === 0 ? 0 : `${(bin.count / tallest) * 100}%` }}
        >
          {stack.length === 0 ? (
            <span className="h-full bg-fg-3" />
          ) : (
            stack.map((player) => {
              const count = bin.byPlayer.get(player.steamid64) ?? 0;
              return count === 0 ? null : <span key={player.steamid64} style={{ height: `${(count / bin.count) * 100}%`, background: `var(${player.token})` }} />;
            })
          )}
        </span>
      ))}
    </div>
  );
}
