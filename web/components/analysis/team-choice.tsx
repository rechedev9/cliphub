'use client';

import type { ReactNode } from 'react';
import type { LibraryTeam } from '@/lib/api/library';
import type { ProTeamCount } from '@/lib/api/pro-matches';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

const PUBLIC_VALUE = 'hltv:';

/** A tool opened without a team: one sentence on what it needs and the team select. */
export function TeamChoice({
  sentence,
  teams,
  onPick,
  publicTeams = [],
  onPickPublic,
  disabled = false,
  className,
}: {
  sentence: string;
  /** Every library team; null while it loads. */
  teams: readonly LibraryTeam[] | null;
  onPick: (teamId: string) => void;
  publicTeams?: readonly ProTeamCount[];
  onPickPublic?: (team: ProTeamCount) => void;
  disabled?: boolean;
  className?: string;
}): ReactNode {
  const withMatches = (teams ?? []).filter((team) => team.match_count > 0);
  const offered = onPickPublic === undefined ? [] : publicTeams;
  const empty = teams !== null && withMatches.length === 0 && offered.length === 0;
  const pick = (value: string): void => {
    const hltv = offered.find((team) => `${PUBLIC_VALUE}${team.id}` === value);
    if (hltv !== undefined) onPickPublic?.(hltv);
    else onPick(value);
  };
  return (
    <section aria-label="Elige un equipo" className={cn('flex flex-col items-start gap-3', className)}>
      <p className="measure-read text-body text-fg-2">
        {empty ? 'Aún no hay equipos con partidas analizadas. Importa una demo en Partidas y su equipo aparecerá aquí.' : sentence}
      </p>
      {empty ? null : (
        <Select value="" onValueChange={pick} disabled={teams === null || disabled}>
          <SelectTrigger aria-label="Equipo" className="h-8 w-64 max-w-full text-body-sm">
            <SelectValue placeholder={teams === null ? 'Cargando equipos' : 'Elige un equipo'} />
          </SelectTrigger>
          <SelectContent>
            {withMatches.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name} ({team.match_count})
              </SelectItem>
            ))}
            {offered.map((team) => (
              <SelectItem key={`${PUBLIC_VALUE}${team.id}`} value={`${PUBLIC_VALUE}${team.id}`}>
                {team.name} ({team.matches} {team.matches === 1 ? 'pública' : 'públicas'})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </section>
  );
}
