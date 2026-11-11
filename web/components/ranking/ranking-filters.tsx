'use client';

import type { ReactNode } from 'react';
import { X } from 'lucide-react';
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
  parseRankingRole,
  type PinnedPlayer,
  type RankingRole,
  type RankingState,
} from '@/lib/ranking/ranking';
import { scopeCountText, type AnalysisScope } from '@/lib/analysis/scope';
import { prettyMapName } from '@/lib/format';
import { PeriodControl } from '@/components/analysis/scope-bar';
import { FilterChip, FilterRow, MinRoundsControl, PinnedChips, PlayerSearch } from '@/components/ranking/ranking-filter-parts';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every value" travels as this. */
const ALL = '__all__';
const TRIGGER = 'h-7 gap-1.5 px-2.5 text-body-sm';
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

const SIDE_OPTIONS = [
  { value: '', label: 'Ambos' },
  { value: 'CT', label: 'CT' },
  { value: 'T', label: 'T' },
] as const;

export type RankingFiltersProps = {
  state: RankingState;
  /** Map names for the map pills, such as `de_mirage`. */
  maps: readonly string[];
  onScope: (patch: Partial<AnalysisScope>) => void;
  /** Sample of the board on screen; null while it is being counted. */
  counts: { readonly maps: number; readonly rounds: number } | null;
  /** Set when some filter differs from the defaults; resets all of them. */
  onClear: (() => void) | null;
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

/** The filter card, as EDGE's rows: maps, side, roles, scope and pinned players. */
export function RankingFilters({
  state,
  maps,
  onScope,
  counts,
  onClear,
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
  const rolesLocked = pro && needsMap;
  return (
    <div className="studio-panel flex flex-col gap-2.5 p-3.5">
      <div aria-live="polite" className="flex items-center justify-end gap-3">
        <p className="text-meta font-semibold tracking-widest whitespace-nowrap text-fg-3 uppercase tabular-nums">
          {counts === null ? 'Contando' : scopeCountText(counts.maps, counts.rounds)}
        </p>
        {onClear === null ? null : (
          <Button type="button" variant="ghost" size="xs" onClick={onClear} className="hover:text-destructive">
            <X aria-hidden />
            Limpiar
          </Button>
        )}
      </div>
      <FilterRow label="Mapas">
        <FilterChip pressed={state.scope.map === ''} onPick={() => onScope({ map: '' })}>
          Todos
        </FilterChip>
        {maps.map((map) => (
          <FilterChip key={map} pressed={state.scope.map === map} onPick={() => onScope({ map })}>
            {prettyMapName(map)}
          </FilterChip>
        ))}
      </FilterRow>
      <FilterRow label="Lado">
        <Segmented label="Lado" value={state.scope.side} options={SIDE_OPTIONS} onChange={(side) => onScope({ side })} />
      </FilterRow>
      <FilterRow label="Roles">
        <div role="group" aria-label="Rol" className="flex flex-wrap items-center gap-1.5">
          <FilterChip pressed={roleValue === ALL} disabled={rolesLocked} title={roleTitle} onPick={() => pickRole(ALL)}>
            Todos
          </FilterChip>
          {pro
            ? null
            : GENERIC_ROLES.map((role) => (
                <FilterChip key={role} pressed={roleValue === role} onPick={() => pickRole(role)}>
                  {LIBRARY_ROLE_LABEL[role]}
                </FilterChip>
              ))}
          {needsMap ? (
            SIDES.map((side) => (
              <FilterChip key={side} tone={side === 'T' ? 't' : 'ct'} pressed={roleValue === allroundValue(side)} disabled={rolesLocked} title={roleTitle} onPick={() => pickRole(allroundValue(side))}>
                {side} todoterreno
              </FilterChip>
            ))
          ) : (
            <>
              {T_ROLES.map((role) => (
                <FilterChip key={role} tone="t" pressed={roleValue === role} onPick={() => pickRole(role)}>
                  T {ROLE_LABEL[role]}
                </FilterChip>
              ))}
              {CT_ROLES.map((role) => (
                <FilterChip key={role} tone="ct" pressed={roleValue === role} onPick={() => pickRole(role)}>
                  CT {ROLE_LABEL[role]}
                </FilterChip>
              ))}
            </>
          )}
        </div>
      </FilterRow>
      <FilterRow label="Alcance">
        <Segmented
          label="Datos"
          value={state.scope.reference}
          options={[
            { value: 'pro', label: 'Profesionales' },
            { value: 'library', label: 'Tu biblioteca' },
          ]}
          onChange={onReference}
        />
        <PeriodControl scope={state.scope} onChange={onScope} />
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
        <MinRoundsControl value={state.minRounds} onCommit={onMinRounds} />
      </FilterRow>
      <FilterRow label="Fijados">
        <PlayerSearch players={players} pins={pins} onPin={onPin} />
        <PinnedChips pins={pins} onPin={onPin} />
      </FilterRow>
    </div>
  );
}
