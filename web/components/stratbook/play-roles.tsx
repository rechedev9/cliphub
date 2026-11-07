'use client';

import { useState, type ReactNode } from 'react';
import { Pencil } from 'lucide-react';
import { patchLibraryPlay } from '@/lib/api/library';
import type { RoleSlot } from '@/lib/api/roles';
import type { LibraryPlay } from '@/lib/api/stratbook';
import { cleanRoles, roleCalloutLabel, rolePlayers, rolesFromPositions, sameRoles } from '@/lib/libreta/roles';
import { Button } from '@/components/ui/button';
import { RoleRowsEditor, useTeamRoster } from '@/components/libreta/role-rows-editor';

const CELL = 'px-2 py-1.5 text-left font-normal';

function RoleTable({ slots, names }: { slots: readonly RoleSlot[]; names: ReadonlyMap<string, string> }): ReactNode {
  if (slots.length === 0) return <p className="text-body-sm text-fg-2">Esta jugada no tiene roles.</p>;
  return (
    <div className="overflow-x-auto rounded-md border border-border">
      <table className="w-full min-w-[32rem] text-body-sm">
        <caption className="sr-only">Roles de la jugada</caption>
        <thead className="bg-surface-1 text-meta text-fg-3">
          <tr>
            <th scope="col" className={CELL}>Rol</th>
            <th scope="col" className={CELL}>Jugador</th>
            <th scope="col" className={CELL}>Callout</th>
            <th scope="col" className={CELL}>Tarea</th>
          </tr>
        </thead>
        <tbody>
          {slots.map((slot, index) => {
            const player = slot.steamid64 ? (names.get(slot.steamid64) ?? 'Otro jugador') : '';
            return (
              <tr key={`${slot.name}:${index}`} className="border-t border-border-subtle">
                <td className={CELL}>{slot.name}</td>
                <td className={CELL}>{player || <span className="text-fg-3">Sin jugador</span>}</td>
                <td className={CELL}>{roleCalloutLabel(slot.callout) || <span className="text-fg-3">Sin callout</span>}</td>
                <td className={CELL}>{slot.task ?? ''}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/** Who does what in the play; editing a play without roles starts from its saved positions. */
export function PlayRoles({ play, onSaved }: { play: LibraryPlay; onSaved: () => void }): ReactNode {
  const roster = useTeamRoster(play.team_id);
  const players = rolePlayers(roster, play.positions);
  const names = new Map(players.map((player) => [player.steamid64, player.nick]));
  const [draft, setDraft] = useState<RoleSlot[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(slots: readonly RoleSlot[]): Promise<void> {
    const clean = cleanRoles(slots);
    if (!clean.ok) {
      setError(clean.error);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await patchLibraryPlay(play.id, { roles: clean.slots });
      setDraft(null);
      onSaved();
    } catch {
      setError('No se pudieron guardar los roles. Comprueba que ClipHub Studio sigue abierto.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="@container/roles flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-body font-semibold text-fg-1">Roles</h2>
        {draft === null ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setDraft(play.roles.length > 0 ? play.roles : rolesFromPositions(play))}
          >
            <Pencil aria-hidden />
            Editar roles
          </Button>
        ) : null}
      </div>
      {draft === null ? (
        <RoleTable slots={play.roles} names={names} />
      ) : (
        <>
          {play.roles.length === 0 && draft.length > 0 ? (
            <p className="text-body-sm text-fg-2">Una fila por jugador de las posiciones. Cambia cada nombre por su rol.</p>
          ) : null}
          <RoleRowsEditor label="Roles de la jugada" slots={draft} players={players} onChange={setDraft} />
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <div className="flex gap-2">
            <Button type="button" size="sm" disabled={busy || sameRoles(draft, play.roles)} onClick={() => void save(draft)}>
              Guardar roles
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => {
                setDraft(null);
                setError(null);
              }}
            >
              Cancelar
            </Button>
          </div>
        </>
      )}
    </section>
  );
}
