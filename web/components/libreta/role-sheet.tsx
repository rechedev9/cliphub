'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { PlugZap } from 'lucide-react';
import { LibraryApiError, isLibraryOffline, type LibraryRosterEntry } from '@/lib/api/library';
import { getTeamRoles, setTeamRoles, type RoleSlot, type RoleSuggestion, type TeamRoles } from '@/lib/api/roles';
import type { LibraryPlay } from '@/lib/api/stratbook';
import type { TacticalSide } from '@/lib/api/tactical';
import { LIBRETA_QUERY } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { busiestMap } from '@/lib/libreta/plan-table';
import {
  busiestTeam,
  cleanRoles,
  fillFromSuggestion,
  rolePlayers,
  sameRoles,
  suggestedRoleLabel,
  type PlaceLabel,
  type RolePlayer,
  type RoleSides,
} from '@/lib/libreta/roles';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { MapField, NO_TEAM, TeamField, mapChoices, useLibraryTeams } from './play-fields';
import { PlayRolesList } from './play-roles-list';
import { RoleRowsEditor, useTeamRoster } from './role-rows-editor';

const MAP_PATTERN = /^[a-z0-9_]+$/;

/** The team and map of the Roles tab, kept in `?equipo=` and `?mapa=` so a reload or a link lands on the same sheet. */
function useRolesScope(): { team: string | undefined; map: string | undefined; set: (key: 'team' | 'map', value: string) => void } {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const query = params.toString();
  const set = useCallback(
    (key: 'team' | 'map', value: string) => {
      const next = new URLSearchParams(query);
      next.set(LIBRETA_QUERY[key], value);
      router.replace(`${pathname}?${next.toString()}`, { scroll: false });
    },
    [router, pathname, query],
  );
  const map = params.get(LIBRETA_QUERY.map) ?? undefined;
  return { team: params.get(LIBRETA_QUERY.team) ?? undefined, map: map !== undefined && MAP_PATTERN.test(map) ? map : undefined, set };
}

/** The Roles tab: who does what for a team on one map, per side, and the roles written in its plays. */
export function RoleSheet({ plays }: { plays: LibraryPlay[] | null }): ReactNode {
  const teams = useLibraryTeams();
  const scope = useRolesScope();
  const team = scope.team ?? (plays ? busiestTeam(plays) : null) ?? teams[0]?.id ?? NO_TEAM;
  const teamId = team === NO_TEAM ? null : team;
  const roster = useTeamRoster(teamId);

  if (plays === null) {
    return (
      <div role="status" aria-label="Cargando roles" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-md" />
        <Skeleton className="h-72 w-full" />
      </div>
    );
  }
  const teamPlays = plays.filter((play) => play.team_id === teamId);
  const map = scope.map ?? busiestMap(teamPlays, 'de_mirage');
  const teamName = teams.find((entry) => entry.id === teamId)?.name ?? plays.find((play) => play.team_id === teamId)?.team_name;
  const names = new Map(roster.map((entry) => [entry.steamid64, entry.nick]));

  return (
    <div className="flex flex-col gap-5">
      <div className="grid max-w-md gap-3 sm:grid-cols-2">
        <TeamField value={team} onChange={(next) => scope.set('team', next)} teams={teams} currentName={teamName} />
        <MapField value={map} onChange={(next) => scope.set('map', next)} maps={mapChoices([map, ...plays.map((play) => play.map)])} />
      </div>
      {teamId === null ? (
        <p className="text-body-sm text-fg-2">Las jugadas sin equipo no tienen hoja de roles. Elige un equipo para escribir la suya.</p>
      ) : (
        <TeamSheet key={`${teamId}:${map}`} teamId={teamId} map={map} roster={roster} />
      )}
      <section aria-label="Roles en las jugadas" className="flex flex-col gap-2">
        <h2 className="text-body font-semibold text-fg-1">Roles en las jugadas</h2>
        <PlayRolesList plays={plays} map={map} team={teamId} teamName={teamName} names={names} />
      </section>
    </div>
  );
}

type SheetLoad = { kind: 'loading' } | { kind: 'ready'; roles: TeamRoles } | { kind: 'failed'; offline: boolean };

function TeamSheet({ teamId, map, roster }: { teamId: string; map: string; roster: readonly LibraryRosterEntry[] }): ReactNode {
  const [load, setLoad] = useState<SheetLoad>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    getTeamRoles(teamId, map)
      .then((roles) => {
        if (active) setLoad({ kind: 'ready', roles });
      })
      .catch((error: unknown) => {
        if (active) setLoad({ kind: 'failed', offline: isLibraryOffline(error) });
      });
    return () => {
      active = false;
    };
  }, [teamId, map, attempt]);

  if (load.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando la hoja de roles" className="grid gap-4 lg:grid-cols-2">
        <Skeleton className="h-72 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    );
  }
  if (load.kind === 'failed') {
    return (
      <EmptyState
        icon={PlugZap}
        title={load.offline ? 'Servicio local sin conexión' : 'No se pudo leer la hoja de roles'}
        description={
          load.offline
            ? 'Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo.'
            : 'Puede que el equipo ya no esté en la biblioteca. Elige otro equipo o vuelve a intentarlo.'
        }
        actions={
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              setLoad({ kind: 'loading' });
              setAttempt((n) => n + 1);
            }}
          >
            Reintentar
          </Button>
        }
      />
    );
  }
  const players = rolePlayers(roster, [...load.roles.suggested.t, ...load.roles.suggested.ct]);
  return <SheetEditor roles={load.roles} players={players} teamId={teamId} map={map} />;
}

type SaveState = { kind: 'idle' } | { kind: 'saving' } | { kind: 'saved' } | { kind: 'error'; message: string };

function suggestionText(roles: TeamRoles, map: string): string {
  if (roles.matches === 0) return `No hay partidas de ${roles.team_name} en ${prettyMapName(map)}, así que no hay sugerencia.`;
  const read = roles.matches === 1 ? 'Sugerencia leída de 1 partida.' : `Sugerencia leída de ${roles.matches} partidas.`;
  const skipped = roles.skipped.length;
  if (skipped === 0) return read;
  return `${read} ${skipped === 1 ? '1 partida no se pudo leer.' : `${skipped} partidas no se pudieron leer.`}`;
}

function SheetEditor({ roles, players, teamId, map }: { roles: TeamRoles; players: RolePlayer[]; teamId: string; map: string }): ReactNode {
  const [saved, setSaved] = useState<RoleSides>({ t: roles.sheet.t, ct: roles.sheet.ct });
  const [sides, setSides] = useState<RoleSides>(saved);
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });
  const calloutLabel = useCalloutNames([map]);
  const place = (token: string): string => calloutLabel(map, token);
  const dirty = !sameRoles(sides.t, saved.t) || !sameRoles(sides.ct, saved.ct);
  const fill = fillFromSuggestion(sides, roles.suggested, place);

  function edit(side: TacticalSide, slots: RoleSlot[]): void {
    setSides((current) => (side === 'T' ? { ...current, t: slots } : { ...current, ct: slots }));
    setSave({ kind: 'idle' });
  }

  async function submit(): Promise<void> {
    const t = cleanRoles(sides.t);
    const ct = cleanRoles(sides.ct);
    if (!t.ok) {
      setSave({ kind: 'error', message: `T: ${t.error}` });
      return;
    }
    if (!ct.ok) {
      setSave({ kind: 'error', message: `CT: ${ct.error}` });
      return;
    }
    setSave({ kind: 'saving' });
    try {
      const sheet = await setTeamRoles(teamId, { map, t: t.slots, ct: ct.slots });
      setSaved({ t: sheet.t, ct: sheet.ct });
      setSides({ t: sheet.t, ct: sheet.ct });
      setSave({ kind: 'saved' });
    } catch (error: unknown) {
      const rejected = error instanceof LibraryApiError && error.status === 400;
      setSave({
        kind: 'error',
        message: rejected
          ? 'Studio no aceptó la hoja. Revisa que cada rol tenga nombre y que las tareas no pasen de 200 caracteres.'
          : 'No se pudo guardar la hoja de roles. Comprueba que ClipHub Studio sigue abierto.',
      });
    }
  }

  return (
    <section aria-label={`Hoja de roles de ${roles.team_name} en ${prettyMapName(map)}`} className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <p className="mr-auto text-body-sm text-fg-2">{suggestionText(roles, map)}</p>
        <span title={fill.filled.length === 0 && roles.matches > 0 ? 'Solo rellena los lados que no tienen roles.' : undefined}>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={fill.filled.length === 0}
            onClick={() => {
              setSides({ t: fill.t, ct: fill.ct });
              setSave({ kind: 'idle' });
            }}
          >
            Usar la sugerencia
          </Button>
        </span>
        <Button type="button" size="sm" disabled={!dirty || save.kind === 'saving'} onClick={() => void submit()}>
          Guardar
        </Button>
      </div>
      {save.kind === 'error' ? (
        <p role="alert" className="text-body-sm text-destructive">
          {save.message}
        </p>
      ) : null}
      {save.kind === 'saved' && !dirty ? (
        <p role="status" className="text-body-sm text-fg-2">
          Hoja de roles guardada.
        </p>
      ) : null}
      <div className="grid gap-4 lg:grid-cols-2">
        <SideColumn side="T" slots={sides.t} suggested={roles.suggested.t} players={players} place={place} onChange={(slots) => edit('T', slots)} />
        <SideColumn side="CT" slots={sides.ct} suggested={roles.suggested.ct} players={players} place={place} onChange={(slots) => edit('CT', slots)} />
      </div>
    </section>
  );
}

function SideColumn({
  side,
  slots,
  suggested,
  players,
  place,
  onChange,
}: {
  side: TacticalSide;
  slots: RoleSlot[];
  suggested: readonly RoleSuggestion[];
  players: readonly RolePlayer[];
  place: PlaceLabel;
  onChange: (slots: RoleSlot[]) => void;
}): ReactNode {
  return (
    <section aria-label={`Roles ${side}`} className="studio-panel @container/roles flex flex-col gap-3 p-3">
      <h3 className="text-body-sm font-semibold text-fg-1">{side}</h3>
      {slots.length === 0 ? (
        <div className="flex flex-col gap-1.5 text-body-sm text-fg-2">
          <p>Sin roles {side} todavía.</p>
          {suggested.length > 0 ? (
            <ul className="flex flex-col gap-1 text-fg-3">
              {suggested.map((entry) => (
                <li key={entry.steamid64}>
                  <span className="text-fg-2">{suggestedRoleLabel(entry.role)}</span>: {entry.player || 'Sin nombre'}
                  {entry.callout ? `, ${place(entry.callout)}` : ''}
                  <span className="tabular-nums"> ({entry.rounds} rondas)</span>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
      <RoleRowsEditor label={`Roles ${side}`} slots={slots} players={players} onChange={onChange} />
    </section>
  );
}
