'use client';

import type { ReactNode } from 'react';
import type { ClockBin, ClockCount } from '@/lib/analysis/grenade-patterns';
import { roundClock } from '@/lib/replay/tramo';

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

/**
 * When the grenades go off over the round clock, as bars under the tramo; with
 * players chosen each bar stacks them in their colours.
 */
export function GrenadeClockMarks({ bins, stack }: { bins: readonly ClockBin[]; stack: Stack }): ReactNode {
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
