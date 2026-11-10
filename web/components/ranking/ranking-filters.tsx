'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Pin, Search } from 'lucide-react';
import {
  CT_ROLES,
  GENERIC_ROLES,
  LEADERBOARD_SOURCES,
  PRO_TOPS,
  T_ROLES,
  type LeaderboardMatchSource,
  type LeaderboardRow,
  type MapRoleOption,
  type ProTop,
} from '@/lib/api/baseline';
import { ROLE_LABEL as LIBRARY_ROLE_LABEL, SOURCE_LABEL } from '@/lib/library/partidas';
import { ROLE_LABEL } from '@/lib/ranking/columns';
import {
  ALLROUND,
  DEFAULT_MIN_ROUNDS,
  MAX_MIN_ROUNDS,
  parseRankingRole,
  searchPlayers,
  type PinnedPlayer,
  type RankingRole,
  type RankingState,
} from '@/lib/ranking/ranking';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every value" travels as this. */
const ALL = '__all__';
const TRIGGER = 'h-8 gap-1.5 px-2.5 text-body-sm';
const SIDES = ['T', 'CT'] as const;

function allroundValue(side: 'CT' | 'T'): string {
  return `${ALLROUND}:${side}`;
}

function parseTop(value: string): ProTop | null {
  return PRO_TOPS.find((top) => String(top) === value) ?? null;
}

function parseLevel(value: string): LeaderboardMatchSource | '' {
  return LEADERBOARD_SOURCES.find((source) => source === value) ?? '';
}

export type RankingFiltersProps = {
  state: RankingState;
  onRole: (role: RankingRole | '') => void;
  onLevel: (level: LeaderboardMatchSource | '') => void;
  onTop: (top: ProTop | null) => void;
  onTeamTop: (top: ProTop | null) => void;
  onEvent: (event: string) => void;
  onMapRole: (mapRole: string) => void;
  /** Todoterreno on a side over every map, or null to clear it. */
  onAllround: (side: 'CT' | 'T' | null) => void;
  onMinRounds: (rounds: number) => void;
  onReference: (reference: 'library' | 'pro') => void;
  /** Positions players hold on the map and side, and the pro events, as the board lists them. */
  mapRoles: readonly MapRoleOption[];
  events: readonly string[];
  /** The ranked players, to search among when pinning. */
  players: readonly LeaderboardRow[];
  pins: readonly PinnedPlayer[];
  onPin: (pin: PinnedPlayer) => void;
};

/** A whole number committed on blur or Enter, so typing "120" does not ask for 1, 12 and 120. */
function MinRoundsInput({ value, onCommit }: { value: number; onCommit: (rounds: number) => void }): ReactNode {
  const [text, setText] = useState(String(value));
  useEffect(() => setText(String(value)), [value]);
  function commit(): void {
    const n = Number(text);
    if (Number.isInteger(n) && n >= 1 && n <= MAX_MIN_ROUNDS) {
      if (n !== value) onCommit(n);
    } else {
      setText(String(value));
    }
  }
  return (
    <label className="flex items-center gap-1.5 text-body-sm text-fg-2">
      Rondas mínimas
      <Input
        type="number"
        inputMode="numeric"
        min={1}
        max={MAX_MIN_ROUNDS}
        value={text}
        aria-label="Rondas mínimas"
        title={`Por debajo de ${DEFAULT_MIN_ROUNDS} rondas las cifras se mueven mucho`}
        onChange={(event) => setText(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key === 'Enter') commit();
        }}
        className="h-8 w-20 px-2 text-body-sm tabular-nums"
      />
    </label>
  );
}

/** Search among the ranked players and pin one, so a player can be followed through any filter. */
function PlayerSearch({ players, pins, onPin }: { players: readonly LeaderboardRow[]; pins: readonly PinnedPlayer[]; onPin: (pin: PinnedPlayer) => void }): ReactNode {
  const [term, setTerm] = useState('');
  const found = searchPlayers({ rows: players, term, pins });
  return (
    <div className="relative">
      <Search aria-hidden className="pointer-events-none absolute top-2 left-2 size-3.5 text-fg-3" />
      <Input
        type="search"
        value={term}
        placeholder="Buscar jugadores…"
        aria-label="Buscar jugadores para fijar"
        onChange={(event) => setTerm(event.target.value)}
        className="h-8 w-48 pl-7 text-body-sm"
      />
      {found.length > 0 ? (
        <ul className="studio-panel absolute z-10 mt-1 w-64 p-1" aria-label="Jugadores encontrados">
          {found.map((row) => (
            <li key={row.steamid64}>
              <button
                type="button"
                onClick={() => {
                  onPin({ steamid64: row.steamid64, name: row.name });
                  setTerm('');
                }}
                className="flex h-8 w-full items-center gap-2 rounded-sm px-2 text-left text-body-sm text-fg-1 outline-none hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50"
              >
                <Pin aria-hidden className="size-3 shrink-0 text-fg-3" />
                <span className="truncate">{row.name}</span>
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** The ranking's own filters; map, side and period come from the shared scope bar. */
export function RankingFilters({
  state,
  onRole,
  onLevel,
  onTop,
  onTeamTop,
  onEvent,
  onMapRole,
  onAllround,
  onMinRounds,
  onReference,
  mapRoles,
  events,
  players,
  pins,
  onPin,
}: RankingFiltersProps): ReactNode {
  const pro = state.scope.reference === 'pro';
  const needsMap = state.scope.map === '';
  const needsSide = state.scope.side === '';
  // The options arrive with the board; the chosen value from the URL must show while it loads.
  const chosen = { key: state.mapRole, label: state.mapRole === ALLROUND ? 'Todoterreno' : state.mapRole, players: 0 };
  const positions = state.mapRole === '' || mapRoles.some((option) => option.key === state.mapRole) ? mapRoles : [chosen, ...mapRoles];
  const eventNames = state.event === '' || events.includes(state.event) ? events : [state.event, ...events];
  const roleTitle = pro && needsMap ? 'Elige un mapa: con datos profesionales el rol se lee en un mapa' : undefined;
  // Without a map, Todoterreno on a side is the Rol filter's own choice; with a map it is a position.
  const anyMapAllround = needsMap && state.mapRole === ALLROUND && state.scope.side !== '';
  const pickedRole = state.role === '' ? ALL : state.role;
  const roleValue = anyMapAllround ? allroundValue(state.scope.side) : pickedRole;
  function pickRole(next: string): void {
    const side = SIDES.find((entry) => allroundValue(entry) === next);
    if (side !== undefined) onAllround(side);
    else if (next === ALL && anyMapAllround) onAllround(null);
    else onRole(next === ALL ? '' : parseRankingRole(next, state.scope));
  }
  const positionTitle = needsMap || needsSide ? 'Elige un mapa y un lado: la posición es una zona del mapa en un lado' : undefined;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Segmented
        label="Datos"
        value={state.scope.reference}
        options={[
          { value: 'library', label: 'Tu biblioteca' },
          { value: 'pro', label: 'Profesionales' },
        ]}
        onChange={onReference}
      />
      <Select value={roleValue} onValueChange={pickRole} disabled={pro && needsMap}>
        <SelectTrigger aria-label="Rol" title={roleTitle} className={cn(TRIGGER, roleValue !== ALL && 'border-fg-3 text-fg-1')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>Todos los roles</SelectItem>
          {pro
            ? null
            : GENERIC_ROLES.map((role) => (
                <SelectItem key={role} value={role}>
                  {LIBRARY_ROLE_LABEL[role]}
                </SelectItem>
              ))}
          {needsMap ? (
            SIDES.map((side) => (
              <SelectItem key={side} value={allroundValue(side)}>
                {side} todoterreno
              </SelectItem>
            ))
          ) : (
            <>
              {CT_ROLES.map((role) => (
                <SelectItem key={role} value={role}>
                  CT, {ROLE_LABEL[role].toLowerCase()}
                </SelectItem>
              ))}
              {T_ROLES.map((role) => (
                <SelectItem key={role} value={role}>
                  T, {ROLE_LABEL[role].toLowerCase()}
                </SelectItem>
              ))}
            </>
          )}
        </SelectContent>
      </Select>
      <Select value={state.mapRole === '' ? ALL : state.mapRole} onValueChange={(next) => onMapRole(next === ALL ? '' : next)} disabled={needsMap || needsSide}>
        <SelectTrigger aria-label="Posición" title={positionTitle} className={cn(TRIGGER, state.mapRole !== '' && 'border-fg-3 text-fg-1')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>Todas las posiciones</SelectItem>
          {positions.map((option) => (
            <SelectItem key={option.key} value={option.key}>
              {option.players > 0 ? `${option.label} (${option.players})` : option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {pro ? (
        <>
          <Select value={state.top === null ? ALL : String(state.top)} onValueChange={(next) => onTop(parseTop(next))}>
            <SelectTrigger aria-label="Nivel del rival" className={cn(TRIGGER, state.top !== null && 'border-fg-3 text-fg-1')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Cualquier rival</SelectItem>
              {PRO_TOPS.map((top) => (
                <SelectItem key={top} value={String(top)}>
                  Rival top {top} de HLTV
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={state.teamTop === null ? ALL : String(state.teamTop)} onValueChange={(next) => onTeamTop(parseTop(next))}>
            <SelectTrigger aria-label="Nivel del equipo" className={cn(TRIGGER, state.teamTop !== null && 'border-fg-3 text-fg-1')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Cualquier equipo</SelectItem>
              {PRO_TOPS.map((top) => (
                <SelectItem key={top} value={String(top)}>
                  Equipo top {top} de HLTV
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={state.event === '' ? ALL : state.event} onValueChange={(next) => onEvent(next === ALL ? '' : next)}>
            <SelectTrigger aria-label="Evento" className={cn(TRIGGER, 'max-w-56', state.event !== '' && 'border-fg-3 text-fg-1')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Todos los eventos</SelectItem>
              {eventNames.map((event) => (
                <SelectItem key={event} value={event}>
                  {event}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </>
      ) : (
        <Select value={state.level === '' ? ALL : state.level} onValueChange={(next) => onLevel(next === ALL ? '' : parseLevel(next))}>
          <SelectTrigger aria-label="Nivel de las demos" className={cn(TRIGGER, state.level !== '' && 'border-fg-3 text-fg-1')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Todas las demos</SelectItem>
            {LEADERBOARD_SOURCES.map((source) => (
              <SelectItem key={source} value={source}>
                {SOURCE_LABEL[source]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
      <MinRoundsInput value={state.minRounds} onCommit={onMinRounds} />
      <PlayerSearch players={players} pins={pins} onPin={onPin} />
    </div>
  );
}
