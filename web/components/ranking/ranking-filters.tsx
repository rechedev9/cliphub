'use client';

import type { ReactNode } from 'react';
import { CT_ROLES, LEADERBOARD_SOURCES, PRO_TOPS, T_ROLES, type LeaderboardMatchSource, type LeaderboardRole, type ProTop } from '@/lib/api/baseline';
import { SOURCE_LABEL } from '@/lib/library/partidas';
import { ROLE_LABEL } from '@/lib/ranking/columns';
import { RANKING_MIN_ROUNDS, type RankingState } from '@/lib/ranking/ranking';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every value" travels as this. */
const ALL = '__all__';
const TRIGGER = 'h-8 gap-1.5 px-2.5 text-body-sm';

function parseRole(value: string): LeaderboardRole | '' {
  return [...CT_ROLES, ...T_ROLES].find((role) => role === value) ?? '';
}

function parseTop(value: string): ProTop | null {
  return PRO_TOPS.find((top) => String(top) === value) ?? null;
}

function parseLevel(value: string): LeaderboardMatchSource | '' {
  return LEADERBOARD_SOURCES.find((source) => source === value) ?? '';
}

export type RankingFiltersProps = {
  state: RankingState;
  onRole: (role: LeaderboardRole | '') => void;
  onLevel: (level: LeaderboardMatchSource | '') => void;
  onTop: (top: ProTop | null) => void;
  onMinRounds: (rounds: number) => void;
  onReference: (reference: 'library' | 'pro') => void;
};

/** The ranking's own filters; map, side and period come from the shared scope bar. */
export function RankingFilters({ state, onRole, onLevel, onTop, onMinRounds, onReference }: RankingFiltersProps): ReactNode {
  const pro = state.scope.reference === 'pro';
  const needsMap = state.scope.map === '';
  const roleTitle = needsMap ? 'Elige un mapa: el rol se lee en un mapa' : undefined;
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
      <Select value={state.role === '' ? ALL : state.role} onValueChange={(next) => onRole(next === ALL ? '' : parseRole(next))} disabled={needsMap}>
        <SelectTrigger
          aria-label="Rol"
          title={roleTitle}
          className={cn(TRIGGER, state.role !== '' && 'border-fg-3 text-fg-1')}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>Todos los roles</SelectItem>
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
        </SelectContent>
      </Select>
      {pro ? (
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
      <Select value={String(state.minRounds)} onValueChange={(next) => onMinRounds(Number(next))}>
        <SelectTrigger aria-label="Rondas mínimas" className={TRIGGER}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {RANKING_MIN_ROUNDS.map((rounds) => (
            <SelectItem key={rounds} value={String(rounds)}>
              Al menos {rounds} rondas
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
