'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Plus, X } from 'lucide-react';
import { getLibraryTeam, type LibraryRosterEntry } from '@/lib/api/library';
import { MAX_ROLE_CALLOUT, MAX_ROLE_NAME, MAX_ROLE_SLOTS, MAX_ROLE_TASK, type RoleSlot } from '@/lib/api/roles';
import type { RolePlayer } from '@/lib/libreta/roles';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

/** Radix Select refuses an empty value, so "no player" travels as this sentinel. */
const NO_PLAYER = '__none__';

/** The team's roster; empty without a team, while loading or offline. */
export function useTeamRoster(teamId: string | null): LibraryRosterEntry[] {
  const [roster, setRoster] = useState<{ teamId: string; entries: LibraryRosterEntry[] } | null>(null);
  useEffect(() => {
    if (teamId === null) return;
    let active = true;
    getLibraryTeam(teamId)
      .then((team) => {
        if (active) setRoster({ teamId, entries: team.roster ?? [] });
      })
      .catch(() => {
        if (active) setRoster({ teamId, entries: [] });
      });
    return () => {
      active = false;
    };
  }, [teamId]);
  return roster !== null && roster.teamId === teamId ? roster.entries : [];
}

function PlayerSelect({
  value,
  players,
  label,
  onChange,
}: {
  value: string;
  players: readonly RolePlayer[];
  label: string;
  onChange: (steamid64: string) => void;
}): ReactNode {
  const known = value === '' || players.some((player) => player.steamid64 === value);
  return (
    <Select value={value === '' ? NO_PLAYER : value} onValueChange={(next) => onChange(next === NO_PLAYER ? '' : next)}>
      <SelectTrigger aria-label={label} className="h-9 w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={NO_PLAYER}>Sin jugador</SelectItem>
        {players.map((player) => (
          <SelectItem key={player.steamid64} value={player.steamid64}>
            {player.nick}
          </SelectItem>
        ))}
        {known ? null : <SelectItem value={value}>Otro jugador</SelectItem>}
      </SelectContent>
    </Select>
  );
}

const ROW_GRID = '@[40rem]/roles:grid-cols-[minmax(0,9rem)_minmax(0,10rem)_minmax(0,9rem)_minmax(0,1fr)_auto]';

/**
 * Up to five role rows, each with its name, player, callout and task. The
 * parent owns `@container/roles`, so the rows lay out by the space they get.
 */
export function RoleRowsEditor({
  label,
  slots,
  players,
  onChange,
}: {
  label: string;
  slots: readonly RoleSlot[];
  players: readonly RolePlayer[];
  onChange: (slots: RoleSlot[]) => void;
}): ReactNode {
  function update(index: number, patch: Partial<RoleSlot>): void {
    onChange(slots.map((slot, i) => (i === index ? { ...slot, ...patch } : slot)));
  }

  return (
    <div role="group" aria-label={label} className="flex flex-col gap-2">
      {slots.length > 0 ? (
        <div className="flex flex-col gap-1">
          <div aria-hidden className={`hidden gap-2 px-2 text-meta text-fg-3 @[40rem]/roles:grid ${ROW_GRID}`}>
            <span>Rol</span>
            <span>Jugador</span>
            <span>Callout</span>
            <span>Tarea</span>
            <span className="w-8" />
          </div>
          <ul className="flex flex-col divide-y divide-border-subtle rounded-md border border-border">
            {slots.map((slot, index) => {
              const n = index + 1;
              return (
                <li key={index} className={`grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-center gap-2 p-2 ${ROW_GRID}`}>
                  <Input
                    aria-label={`Rol ${n}`}
                    placeholder="Rol"
                    value={slot.name}
                    maxLength={MAX_ROLE_NAME}
                    onChange={(event) => update(index, { name: event.target.value })}
                    className="h-9"
                  />
                  <PlayerSelect
                    label={`Jugador del rol ${n}`}
                    value={slot.steamid64 ?? ''}
                    players={players}
                    onChange={(steamid64) => update(index, { steamid64 })}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={`Quitar el rol ${n}`}
                    onClick={() => onChange(slots.filter((_, i) => i !== index))}
                    className="col-start-3 row-span-2 row-start-1 @[40rem]/roles:col-start-5 @[40rem]/roles:row-span-1"
                  >
                    <X aria-hidden />
                  </Button>
                  <Input
                    aria-label={`Callout del rol ${n}`}
                    placeholder="Callout"
                    value={slot.callout ?? ''}
                    maxLength={MAX_ROLE_CALLOUT}
                    onChange={(event) => update(index, { callout: event.target.value })}
                    className="h-9"
                  />
                  <Input
                    aria-label={`Tarea del rol ${n}`}
                    placeholder="Tarea"
                    value={slot.task ?? ''}
                    maxLength={MAX_ROLE_TASK}
                    onChange={(event) => update(index, { task: event.target.value })}
                    className="h-9"
                  />
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
      <div>
        <Button
          type="button"
          variant="outline"
          size="xs"
          disabled={slots.length >= MAX_ROLE_SLOTS}
          onClick={() => onChange([...slots, { name: '' }])}
        >
          <Plus aria-hidden />
          Añadir rol
        </Button>
      </div>
    </div>
  );
}
