'use client';

import type { ReactNode } from 'react';
import type { PlaySide } from '@/lib/api/types';
import { prettifyWeapon } from '@/lib/api/map';
import { isFilterActive, type PlayFilter, type PlayFilterOptions, type PlayKindFilter } from '@/lib/produce/play-filter';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

const ANY_WEAPON = 'any';
const KIND_LABELS: Record<PlayKindFilter, string> = { clutch: 'Clutch', ace: 'Ace' };

export type PlayFiltersProps = {
  options: PlayFilterOptions;
  value: PlayFilter;
  disabled?: boolean;
  onChange: (next: PlayFilter) => void;
};

/** Weapon, kind and side filters over the play list; shows only the choices this match has. */
export function PlayFilters({ options, value, disabled = false, onChange }: PlayFiltersProps): ReactNode {
  const { weapons, kinds, sides } = options;
  if (weapons.length < 2 && kinds.length === 0 && sides.length < 2) return null;
  return (
    <div className="flex w-full flex-wrap items-center gap-2">
      {weapons.length >= 2 ? (
        <Select value={value.weapon ?? ANY_WEAPON} disabled={disabled}
          onValueChange={(next) => onChange({ ...value, weapon: next === ANY_WEAPON ? null : next })}>
          <SelectTrigger aria-label="Arma" className="h-11 w-auto min-w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY_WEAPON}>Todas las armas</SelectItem>
            {weapons.map((weapon) => (
              <SelectItem key={weapon} value={weapon}>Solo {prettifyWeapon(weapon)}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}
      {kinds.length > 0 ? (
        <ToggleGroup type="single" variant="filter" spacing={2} aria-label="Tipo de jugada" disabled={disabled}
          value={value.kind ?? ''}
          onValueChange={(next: string) => onChange({ ...value, kind: kinds.find((kind) => kind === next) ?? null })}>
          {kinds.map((kind) => (
            <ToggleGroupItem key={kind} value={kind}>{KIND_LABELS[kind]}</ToggleGroupItem>
          ))}
        </ToggleGroup>
      ) : null}
      {sides.length >= 2 ? (
        <ToggleGroup type="single" variant="filter" spacing={2} aria-label="Lado" disabled={disabled}
          value={value.side ?? ''}
          onValueChange={(next: string) => onChange({ ...value, side: sides.find((side): side is PlaySide => side === next) ?? null })}>
          {sides.map((side) => (
            <ToggleGroupItem key={side} value={side} aria-label={`Solo lado ${side}`}>{side}</ToggleGroupItem>
          ))}
        </ToggleGroup>
      ) : null}
      {isFilterActive(value) ? (
        <Button type="button" variant="ghost" size="xs" disabled={disabled} onClick={() => onChange({ weapon: null, kind: null, side: null })}>
          Quitar filtros
        </Button>
      ) : null}
    </div>
  );
}
