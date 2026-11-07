'use client';

import type { ReactNode } from 'react';
import {
  ROUND_KIND_FILTERS,
  ROUND_WINNER_FILTERS,
  type PerTeam,
  type RoundFilter,
  type RoundKindFilter,
  type RoundWinnerFilter,
} from '@/lib/library/rounds';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { cn } from '@/lib/utils';

const KIND_LABEL: Record<RoundKindFilter, string> = {
  any: 'Cualquier ronda',
  pistol: 'Pistolas',
  eco: 'Con eco',
  force: 'Con force',
  upset: 'Contra pronóstico',
};

/** The Rondas chips: who won the round, and what kind of round it was. */
export function RoundFilters({
  value,
  onChange,
  teamNames,
  counts,
  upsets,
}: {
  value: RoundFilter;
  onChange: (next: RoundFilter) => void;
  teamNames: PerTeam<string>;
  counts: Readonly<Record<RoundWinnerFilter, number>>;
  /** Whether the win chance is known, which the upset chip needs. */
  upsets: boolean;
}): ReactNode {
  const kinds = ROUND_KIND_FILTERS.filter((kind) => kind !== 'upset' || upsets);
  return (
    <div className="flex flex-col gap-2">
      <ToggleGroup
        type="single"
        variant="filter"
        spacing={2}
        aria-label="Quién ganó"
        value={value.winner}
        onValueChange={(next: string) => onChange({ ...value, winner: ROUND_WINNER_FILTERS.find((key) => key === next) ?? 'all' })}
        className="flex-wrap"
      >
        {ROUND_WINNER_FILTERS.map((key) => (
          <ToggleGroupItem key={key} value={key}>
            {key === 'all' ? 'Todas' : <span className="max-w-48 truncate">Gana {teamNames[key]}</span>}
            <span className={cn('font-normal', value.winner === key ? 'text-primary-foreground' : 'text-fg-3')}>{counts[key]}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <ToggleGroup
        type="single"
        variant="filter"
        spacing={2}
        aria-label="Tipo de ronda"
        value={value.kind}
        onValueChange={(next: string) => onChange({ ...value, kind: kinds.find((kind) => kind === next) ?? 'any' })}
        className="flex-wrap"
      >
        {kinds.map((kind) => (
          <ToggleGroupItem key={kind} value={kind}>
            {KIND_LABEL[kind]}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}
