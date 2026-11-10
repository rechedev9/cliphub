'use client';

import type { MouseEvent, ReactNode } from 'react';
import { pickThrower, type GrenadeThrower } from '@/lib/analysis/grenade-patterns';
import { PLAYER_COLORS } from '@/components/prep/ct-positions-map';
import { Toggle } from '@/components/ui/toggle';

/** A team has five players; a sixth thrower (a stand-in) gets a neutral colour instead of repeating one. */
const PLAYER_COLOR_SLOTS = 5;
const NEUTRAL_TOKEN = '--fg-3';

/** The colour token of the player at `index` of the picker, the same on the radar and the histogram. */
export function playerToken(index: number): string {
  return index < PLAYER_COLOR_SLOTS ? (PLAYER_COLORS[index] ?? NEUTRAL_TOKEN) : NEUTRAL_TOKEN;
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
