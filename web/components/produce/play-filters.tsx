'use client';

import type { ReactNode } from 'react';
import type { PlaySide } from '@/lib/api/types';
import { prettifyWeapon } from '@/lib/api/map';
import { isFilterActive, NO_PLAY_FILTER, type PlayFilter, type PlayFilterOptions, type PlayKindFilter } from '@/lib/produce/play-filter';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

const ANY_WEAPON = 'any';
const ANY_PLAYER = 'any';
const KIND_LABELS: Record<PlayKindFilter, string> = { clutch: 'Clutch', ace: 'Ace' };

export type PlayFiltersProps = {
  options: PlayFilterOptions;
  value: PlayFilter;
  disabled?: boolean;
  onChange: (next: PlayFilter) => void;
};

/** Player, weapon, kind and side filters over the play list; shows only the choices this match has. */
export function PlayFilters({ options, value, disabled = false, onChange }: PlayFiltersProps): ReactNode {
  const { weapons, kinds, sides, players } = options;
  if (players.length < 2 && weapons.length < 2 && kinds.length === 0 && sides.length < 2) return null;
  return (
    <div className="flex w-full flex-wrap items-center gap-2">
      {players.length >= 2 ? (
        <Select value={value.player ?? ANY_PLAYER} disabled={disabled}
          onValueChange={(next) => onChange({ ...value, player: next === ANY_PLAYER ? null : next })}>
          <SelectTrigger aria-label="Jugador" className="h-11 w-auto min-w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY_PLAYER}>Todos los jugadores</SelectItem>
            {players.map((player) => (
              <SelectItem key={player.steamId} value={player.steamId}>{player.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}
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
        <Button type="button" variant="ghost" size="xs" disabled={disabled} onClick={() => onChange(NO_PLAY_FILTER)}>
          Quitar filtros
        </Button>
      ) : null}
    </div>
  );
}
