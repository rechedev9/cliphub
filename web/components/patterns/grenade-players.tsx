'use client';

import type { MouseEvent, ReactNode } from 'react';
import { pickThrower, type GrenadeThrower } from '@/lib/analysis/grenade-patterns';
import { PLAYER_COLORS } from '@/components/prep/ct-positions-map';
import { Toggle } from '@/components/ui/toggle';

/** The colour token of the player at `index` of the picker, the same on the radar and the histogram. */
export function playerToken(index: number): string {
  return PLAYER_COLORS[index % PLAYER_COLORS.length] ?? '--fg-1';
}

/** Whose grenades count: everyone, one player with a click, or several with Shift + click. */
export function GrenadePlayerPicker({
  players,
  picked,
  onChange,
}: {
  players: readonly GrenadeThrower[];
  picked: readonly string[];
  onChange: (next: string[]) => void;
}): ReactNode {
  if (players.length === 0) return null;
  return (
    <div role="group" aria-label="Jugadores" className="flex flex-wrap items-center gap-1.5">
      <Toggle variant="filter" size="sm" pressed={picked.length === 0} onPressedChange={() => onChange([])}>
        Todo el equipo
      </Toggle>
      {players.map((player, index) => (
        <Toggle
          key={player.steamid64}
          variant="filter"
          size="sm"
          pressed={picked.includes(player.steamid64)}
          title="Mayús + clic para sumar jugadores"
          onClick={(event: MouseEvent<HTMLButtonElement>) => onChange(pickThrower(picked, player.steamid64, event.shiftKey))}
        >
          <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ background: `var(${playerToken(index)})` }} />
          {player.name}
          <span className="tabular-nums opacity-80">{player.throws}</span>
        </Toggle>
      ))}
    </div>
  );
}
