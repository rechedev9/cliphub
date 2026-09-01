'use client';

import { Search } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

/** The three scoreboard views: everything, only wins, or highest-frag first. */
export type MatchFilter = 'all' | 'wins' | 'frags';

const FILTER_ITEMS: Array<{ value: MatchFilter; label: string; description: string }> = [
  { value: 'all', label: 'TODAS', description: 'Todas las demos' },
  { value: 'wins', label: 'VICTORIAS', description: 'Solo victorias' },
  { value: 'frags', label: 'MEJORES FRAGS', description: 'Mejores frags primero' },
];

export type MatchFiltersProps = {
  filter: MatchFilter;
  onFilterChange: (filter: MatchFilter) => void;
  query: string;
  onQueryChange: (query: string) => void;
};

/** Narrow the raw Radix value back onto the filter union without a cast. */
function toMatchFilter(value: string): MatchFilter | null {
  return FILTER_ITEMS.find((item) => item.value === value)?.value ?? null;
}

/**
 * Filter controls for the matches scoreboard: the shared square mono chips
 * (Todas / Victorias / Mejores frags) plus a map search box.
 *
 * `spacing={2}` matters visually: at the default `spacing={0}` the group applies
 * its *joined plate* radii — first chip rounded-left, last rounded-right, middle
 * square — while the call site separated the chips with a gap, so the control
 * rendered as three mismatched corners floating 8px apart.
 */
export function MatchFilters({ filter, onFilterChange, query, onQueryChange }: MatchFiltersProps) {
  return (
    <div className="flex w-full flex-col gap-3 @[48rem]/content:w-auto @[48rem]/content:items-end">
      <div className="-mb-1 w-full overflow-x-auto pb-1 @[48rem]/content:w-auto @[48rem]/content:overflow-visible @[48rem]/content:pb-0">
        <ToggleGroup
          type="single"
          value={filter}
          onValueChange={(value) => {
            const next = toMatchFilter(value);
            if (next !== null) onFilterChange(next);
          }}
          variant="filter"
          spacing={2}
          className="w-max"
          aria-label="Filtrar demos"
        >
          {FILTER_ITEMS.map((item) => (
            <ToggleGroupItem key={item.value} value={item.value} aria-label={item.description}>
              {item.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>

      <div className="relative w-full @[48rem]/content:max-w-[17rem]">
        <Search
          size={16}
          aria-hidden
          className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-fg-3"
        />
        <Input
          type="search"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Buscar mapa…"
          aria-label="Buscar por mapa"
          className="pl-10 font-mono text-body-sm"
        />
      </div>
    </div>
  );
}
