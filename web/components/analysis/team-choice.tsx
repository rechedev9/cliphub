'use client';

import type { ReactNode } from 'react';
import type { LibraryTeam } from '@/lib/api/library';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** A tool opened without a team: one sentence on what it needs and the team select. */
export function TeamChoice({
  sentence,
  teams,
  onPick,
  className,
}: {
  sentence: string;
  /** Every library team; null while it loads. */
  teams: readonly LibraryTeam[] | null;
  onPick: (teamId: string) => void;
  className?: string;
}): ReactNode {
  const withMatches = (teams ?? []).filter((team) => team.match_count > 0);
  return (
    <section aria-label="Elige un equipo" className={cn('flex flex-col items-start gap-3', className)}>
      <p className="measure-read text-body text-fg-2">
        {teams !== null && withMatches.length === 0 ? 'Aún no hay equipos con partidas analizadas. Importa una demo en Partidas y su equipo aparecerá aquí.' : sentence}
      </p>
      {teams !== null && withMatches.length === 0 ? null : (
        <Select value="" onValueChange={onPick} disabled={teams === null}>
          <SelectTrigger aria-label="Equipo" className="h-8 w-64 max-w-full text-body-sm">
            <SelectValue placeholder={teams === null ? 'Cargando equipos' : 'Elige un equipo'} />
          </SelectTrigger>
          <SelectContent>
            {withMatches.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name} ({team.match_count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </section>
  );
}
