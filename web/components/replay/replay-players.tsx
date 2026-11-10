'use client';

import type { MouseEvent, ReactNode } from 'react';
import { Users } from 'lucide-react';
import type { TacticalDocument } from '@/lib/api/tactical';
import { pickPlayer } from '@/lib/replay/player-focus';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** The players in the foreground and how to change them; the Replay keeps them in the URL. */
export type FocusControl = { focus: PlayerFocus; onChange: (focus: number[]) => void };

export const FOCUS_HINT = 'Clic deja solo a ese jugador. Mayús y clic suma otro.';
/** The hint of a player's own card, where F also centres the map on the chosen player. */
export const FOLLOW_HINT = `${FOCUS_HINT} F centra el mapa en él.`;

/** The first row of every player list: nobody chosen, so the whole team counts. */
export function AllTeamRow({ focus, onChange, className }: FocusControl & { className?: string }): ReactNode {
  const all = focus.length === 0;
  return (
    <button
      type="button"
      aria-pressed={all}
      title={FOCUS_HINT}
      onClick={() => onChange([])}
      className={cn(
        'flex h-7 items-center gap-1.5 rounded-sm px-2 text-body-sm outline-none transition-colors duration-(--dur-fast)',
        all ? 'bg-surface-4 font-medium text-fg-1' : 'text-fg-2 hover:bg-surface-3 hover:text-fg-1',
        FOCUS_RING,
        className,
      )}
    >
      <Users className="size-3.5" aria-hidden />
      Todo el equipo
    </button>
  );
}

/**
 * The player list of the forms that count rounds (Mapa de calor, Recorridos,
 * Granadas): the same rule as the Replay's panel, so the choice carries over.
 * With a team in scope it lists that team; without, both.
 */
export function ReplayPlayers({ doc, teamKey, focus, onChange }: FocusControl & { doc: TacticalDocument; teamKey: string | undefined }): ReactNode {
  const teams = doc.teams.filter((team) => teamKey === undefined || team.key === teamKey);
  const names = new Map(doc.players.map((player) => [player.slot, player.name]));
  const pick = (slot: number) => (event: MouseEvent) => onChange(pickPlayer(focus, slot, event.shiftKey));
  return (
    <section aria-label="Jugadores" className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-lg border border-border bg-surface-1 px-2 py-1.5">
      <AllTeamRow focus={focus} onChange={onChange} />
      {teams.map((team) => (
        <div key={team.key} role="group" aria-label={team.name || team.key} className="flex flex-wrap items-center gap-1">
          {teams.length > 1 ? <span className="px-1 text-meta text-fg-3">{team.name || team.key}</span> : null}
          {team.slots.map((slot) => {
            const chosen = focus.includes(slot);
            return (
              <button
                key={slot}
                type="button"
                aria-pressed={chosen}
                title={FOLLOW_HINT}
                onClick={pick(slot)}
                className={cn(
                  'h-7 max-w-40 truncate rounded-sm border px-2 text-body-sm outline-none transition-[background-color,border-color,opacity] duration-(--dur-fast)',
                  chosen ? 'border-fg-2 bg-surface-4 font-medium text-fg-1' : 'border-border-strong text-fg-2 hover:bg-surface-3 hover:text-fg-1',
                  focus.length > 0 && !chosen && 'opacity-45',
                  FOCUS_RING,
                )}
              >
                {names.get(slot) ?? `Jugador ${slot}`}
              </button>
            );
          })}
        </div>
      ))}
    </section>
  );
}
