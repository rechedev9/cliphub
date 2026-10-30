'use client';

import type { ReactNode } from 'react';
import type { LibraryMatch } from '@/lib/api/library';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { DATE_RANGES, inDateRange, rivalOf, teamLabel } from '@/lib/library/partidas';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

/** The picker's filters: map, rival team and date range. ANY keeps every match. */
export type PickerFilters = { map: string; rival: string; range: string };

export const ANY = '__any__';
export const EMPTY_PICKER_FILTERS: PickerFilters = { map: ANY, rival: ANY, range: ANY };

/** The matches the filters keep, newest first. */
export function filterPickerMatches(matches: readonly LibraryMatch[], teamId: string, filters: PickerFilters, now: number): LibraryMatch[] {
  return matches
    .filter((match) => filters.map === ANY || match.map === filters.map)
    .filter((match) => filters.rival === ANY || (rivalOf(match, teamId)?.team_id ?? '') === filters.rival)
    .filter((match) => filters.range === ANY || inDateRange(filters.range, match.played_at, now))
    .sort((a, b) => Date.parse(b.played_at) - Date.parse(a.played_at) || a.id.localeCompare(b.id));
}

function FilterSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
}): ReactNode {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger aria-label={label} className="h-8 min-w-36">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ANY}>{label}: todos</SelectItem>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * Picks the matches a report reads from a team's matches, filtered by map,
 * rival and date. Only analysed matches can feed a report.
 */
export function ReportMatchPicker({
  teamId,
  matches,
  selected,
  onSelectedChange,
  filters,
  onFiltersChange,
}: {
  teamId: string;
  matches: readonly LibraryMatch[];
  selected: ReadonlySet<string>;
  onSelectedChange: (next: Set<string>) => void;
  filters: PickerFilters;
  onFiltersChange: (next: PickerFilters) => void;
}): ReactNode {
  const shown = filterPickerMatches(matches, teamId, filters, Date.now());
  const maps = [...new Set(matches.map((match) => match.map))]
    .sort()
    .map((map) => ({ value: map, label: prettyMapName(map) }));
  const rivals = new Map<string, string>();
  for (const match of matches) {
    const rival = rivalOf(match, teamId);
    if (rival?.team_id) rivals.set(rival.team_id, teamLabel(rival));
  }
  const rivalOptions = [...rivals].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'));

  function toggle(id: string): void {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onSelectedChange(next);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <FilterSelect label="Mapa" value={filters.map} options={maps} onChange={(map) => onFiltersChange({ ...filters, map })} />
        <FilterSelect label="Rival" value={filters.rival} options={rivalOptions} onChange={(rival) => onFiltersChange({ ...filters, rival })} />
        <FilterSelect
          label="Fecha"
          value={filters.range}
          options={DATE_RANGES.map((range) => ({ value: range.value, label: range.label }))}
          onChange={(range) => onFiltersChange({ ...filters, range })}
        />
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => onSelectedChange(new Set([...selected, ...shown.map((match) => match.id)]))}
        >
          Marcar las visibles
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => onSelectedChange(new Set())}>
          Quitar todas
        </Button>
      </div>
      <p className="text-meta text-fg-3">
        {selected.size} {selected.size === 1 ? 'partida elegida' : 'partidas elegidas'} de {matches.length}
      </p>
      <div className="max-h-72 overflow-y-auto rounded-md border border-border">
        {shown.length === 0 ? (
          <p className="px-3 py-3 text-body-sm text-fg-2">Ninguna partida cumple estos filtros.</p>
        ) : (
          <ul aria-label="Partidas del equipo">
            {shown.map((match) => {
              const own = match.teams.find((team) => team.team_id === teamId);
              const rival = rivalOf(match, teamId);
              return (
                <li key={match.id} className="border-b border-border-subtle last:border-b-0">
                  <label className="flex cursor-pointer items-center gap-3 px-3 py-2 text-body-sm hover:bg-surface-2">
                    <input
                      type="checkbox"
                      className="size-4 accent-primary"
                      checked={selected.has(match.id)}
                      onChange={() => toggle(match.id)}
                    />
                    <span className="w-20 shrink-0 text-fg-2">{formatShortDate(match.played_at)}</span>
                    <span className="w-20 shrink-0">{prettyMapName(match.map)}</span>
                    <span className="min-w-0 flex-1 truncate">contra {teamLabel(rival)}</span>
                    <span className="font-mono text-fg-2">{`${own?.score ?? 0}-${rival?.score ?? 0}`}</span>
                  </label>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}
