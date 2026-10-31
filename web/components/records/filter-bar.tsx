'use client';

import type { ReactNode } from 'react';
import { Search, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { FILTER_ANY, SEARCH_FILTER, activeFilterCount } from '@/lib/records/table-view';

export type FilterOption = { readonly value: string; readonly label: string };
export type FilterDefinition = {
  readonly id: string;
  readonly label: string;
  readonly options: readonly FilterOption[];
};

// Radix Select refuses an empty item value, so "any" travels as this sentinel inside the control only.
const ANY_OPTION = '__any__';

export type FilterBarProps = {
  filters?: readonly FilterDefinition[];
  values: Readonly<Record<string, string>>;
  onChange: (id: string, value: string) => void;
  onReset: () => void;
  searchPlaceholder?: string;
  /** "12 de 40 partidas": what the filters leave on screen. */
  count?: { readonly shown: number; readonly total: number; readonly noun: string };
  /** Puts the selects on a row of their own under the search and the count, for lists with many filters. */
  stacked?: boolean;
  className?: string;
};

/** Search plus one select per filter, a reset and the visible count, in one dense row. */
export function FilterBar({
  filters = [],
  values,
  onChange,
  onReset,
  searchPlaceholder = 'Buscar',
  count,
  stacked = false,
  className,
}: FilterBarProps): ReactNode {
  const active = activeFilterCount(values);
  const selects = filters.map((filter) => {
    const value = values[filter.id] ?? FILTER_ANY;
    return (
      <Select
        key={filter.id}
        value={value === FILTER_ANY ? ANY_OPTION : value}
        onValueChange={(next) => onChange(filter.id, next === ANY_OPTION ? FILTER_ANY : next)}
      >
        <SelectTrigger aria-label={filter.label} className="h-9 min-w-36">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ANY_OPTION}>{`${filter.label}: todos`}</SelectItem>
          {filter.options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  });
  const reset =
    active > 0 ? (
      <Button type="button" variant="ghost" size="sm" className="h-9" onClick={onReset}>
        <X aria-hidden />
        Limpiar filtros
      </Button>
    ) : null;
  const shownCount = count ? (
    <span role="status" className="ml-auto text-body-sm text-fg-3">
      {count.shown === count.total ? `${count.total} ${count.noun}` : `${count.shown} de ${count.total} ${count.noun}`}
    </span>
  ) : null;
  return (
    <div role="search" className={className ?? 'flex flex-wrap items-center gap-2'}>
      <label className="relative min-w-48 flex-1 basis-56 sm:max-w-80">
        <span className="sr-only">{searchPlaceholder}</span>
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-3" aria-hidden />
        <Input
          type="search"
          value={values[SEARCH_FILTER] ?? FILTER_ANY}
          onChange={(event) => onChange(SEARCH_FILTER, event.target.value)}
          placeholder={searchPlaceholder}
          className="h-9 pl-9"
        />
      </label>
      {stacked ? (
        <>
          {shownCount}
          <div className="flex basis-full flex-wrap items-center gap-2">
            {selects}
            {reset}
          </div>
        </>
      ) : (
        <>
          {selects}
          {reset}
          {shownCount}
        </>
      )}
    </div>
  );
}
