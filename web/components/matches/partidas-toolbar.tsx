'use client';

import { useId, type ReactNode } from 'react';
import { ListFilter, Search, X } from 'lucide-react';
import { currentEdition } from '@/lib/edition';
import {
  KIND_LABEL,
  LIBRARY_VIEWS,
  MATCH_KINDS,
  SORTS,
  SORT_LABEL,
  type LibraryListView,
  type LibraryViewMode,
  type MatchKind,
  type PartidaSort,
} from '@/lib/library/partida-groups';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import type { FilterDefinition } from '@/components/records/filter-bar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';

const ALL_KINDS = 'todas';
type KindChoice = MatchKind | typeof ALL_KINDS;
const KIND_OPTIONS: readonly { value: KindChoice; label: string }[] = [
  { value: ALL_KINDS, label: 'Todas' },
  ...MATCH_KINDS.map((kind) => ({ value: kind, label: KIND_LABEL[kind] })),
];

const VIEW_LABEL: Record<LibraryViewMode, string> = { series: 'Por día', tabla: 'Tabla' };
const VIEW_OPTIONS = LIBRARY_VIEWS.map((mode) => ({ value: mode, label: VIEW_LABEL[mode] }));

// Radix Select refuses an empty item value, so "any" travels as this sentinel inside the control only.
const ANY_OPTION = '__any__';

type ActiveChip = { key: string; label: string; value: string; clear: () => void };

export type PartidasToolbarProps = {
  filters: readonly FilterDefinition[];
  values: Readonly<Record<string, string>>;
  onFilterChange: (id: string, value: string) => void;
  onFiltersReset: () => void;
  view: LibraryListView;
  onViewChange: (next: LibraryListView) => void;
  count: { readonly shown: number; readonly total: number };
};

/** Search, one Filtros panel, the view and the sort on one line; the filters in force below as chips that clear one by one. */
export function PartidasToolbar({ filters, values, onFilterChange, onFiltersReset, view, onViewChange, count }: PartidasToolbarProps): ReactNode {
  const desktop = currentEdition() === 'desktop';
  const chips: ActiveChip[] = [];
  for (const filter of filters) {
    const value = values[filter.id] ?? FILTER_ANY;
    if (value === FILTER_ANY) continue;
    const label = filter.options.find((option) => option.value === value)?.label ?? value;
    chips.push({ key: filter.id, label: filter.label, value: label, clear: () => onFilterChange(filter.id, FILTER_ANY) });
  }
  if (view.kind !== null) chips.push({ key: 'kind', label: 'Tipo', value: KIND_LABEL[view.kind], clear: () => onViewChange({ ...view, kind: null }) });
  if (desktop && view.pov) chips.push({ key: 'pov', label: 'Con', value: 'POV', clear: () => onViewChange({ ...view, pov: false }) });
  if (view.voice) chips.push({ key: 'voice', label: 'Con', value: 'voz', clear: () => onViewChange({ ...view, voice: false }) });
  const noun = count.total === 1 ? 'partida' : 'partidas';

  return (
    <div role="search" className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <label className="relative min-w-48 flex-1 basis-64">
          <span className="sr-only">Buscar por mapa, equipo, archivo o etiqueta</span>
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-3" aria-hidden />
          <Input
            type="search"
            value={values[SEARCH_FILTER] ?? FILTER_ANY}
            onChange={(event) => onFilterChange(SEARCH_FILTER, event.target.value)}
            placeholder="Buscar por mapa, equipo, archivo o etiqueta"
            className="h-9 pl-9"
          />
        </label>
        <FiltersPanel filters={filters} values={values} onFilterChange={onFilterChange} view={view} onViewChange={onViewChange} active={chips.length} />
        <Segmented label="Vista" value={view.mode} options={VIEW_OPTIONS} onChange={(mode) => onViewChange({ ...view, mode })} />
        {view.mode === 'series' ? (
          <Select value={view.sort} onValueChange={(sort) => onViewChange({ ...view, sort: SORTS.find((entry) => entry === sort) ?? view.sort })}>
            <SelectTrigger aria-label="Ordenar por" className="h-9 w-auto">
              <SelectValue />
            </SelectTrigger>
            <SelectContent align="end">
              {SORTS.map((sort: PartidaSort) => (
                <SelectItem key={sort} value={sort}>
                  {SORT_LABEL[sort]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : null}
      </div>
      <div className="flex min-h-7 flex-wrap items-center gap-2">
        {chips.map((chip) => (
          <span key={chip.key} className="inline-flex h-7 items-center gap-1.5 border border-border-strong bg-surface-3 pr-0.5 pl-2 text-body-sm">
            <span className="text-fg-3">{chip.label}</span>
            <span className="font-semibold text-fg-1">{chip.value}</span>
            <Button type="button" variant="ghost" size="icon-xs" aria-label={`Quitar el filtro ${chip.label} ${chip.value}`} onClick={chip.clear}>
              <X aria-hidden />
            </Button>
          </span>
        ))}
        {chips.length > 1 ? (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            onClick={() => {
              onFiltersReset();
              onViewChange({ ...view, kind: null, pov: false, voice: false });
            }}
          >
            Quitar filtros
          </Button>
        ) : null}
        <span role="status" className="ml-auto text-body-sm text-fg-3">
          {count.shown === count.total ? `${count.total} ${noun}` : `${count.shown} de ${count.total} ${noun}`}
        </span>
      </div>
    </div>
  );
}

function FiltersPanel({
  filters,
  values,
  onFilterChange,
  view,
  onViewChange,
  active,
}: Omit<PartidasToolbarProps, 'onFiltersReset' | 'count'> & { active: number }): ReactNode {
  const povId = useId();
  const voiceId = useId();
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="h-9" aria-label={active > 0 ? `Filtros, ${active} activos` : 'Filtros'}>
          <ListFilter aria-hidden />
          Filtros
          {active > 0 ? <span className="min-w-5 rounded-sm bg-primary px-1 text-meta font-bold text-primary-foreground">{active}</span> : null}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="grid w-[min(30rem,calc(100vw-2rem))] grid-cols-[5.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-2 p-3">
        <span className="text-meta text-fg-3">Tipo</span>
        <Segmented
          label="Tipo de partida"
          value={view.kind ?? ALL_KINDS}
          options={KIND_OPTIONS}
          onChange={(choice) => onViewChange({ ...view, kind: choice === ALL_KINDS ? null : choice })}
        />
        {filters.map((filter) => {
          const value = values[filter.id] ?? FILTER_ANY;
          return (
            <FilterRow key={filter.id} label={filter.label}>
              <Select value={value === FILTER_ANY ? ANY_OPTION : value} onValueChange={(next) => onFilterChange(filter.id, next === ANY_OPTION ? FILTER_ANY : next)}>
                <SelectTrigger aria-label={filter.label} className="h-8 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ANY_OPTION}>Todos</SelectItem>
                  {filter.options.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FilterRow>
          );
        })}
        <span className="text-meta text-fg-3">Contenido</span>
        <span className="flex flex-wrap items-center gap-4">
          {currentEdition() === 'desktop' ? (
            <span className="flex items-center gap-2">
              <Switch aria-labelledby={povId} checked={view.pov} onCheckedChange={(pov) => onViewChange({ ...view, pov })} />
              <span id={povId} className="text-body-sm text-fg-2">Con POV</span>
            </span>
          ) : null}
          <span className="flex items-center gap-2">
            <Switch aria-labelledby={voiceId} checked={view.voice} onCheckedChange={(voice) => onViewChange({ ...view, voice })} />
            <span id={voiceId} className="text-body-sm text-fg-2">Con voz</span>
          </span>
        </span>
      </PopoverContent>
    </Popover>
  );
}

function FilterRow({ label, children }: { label: string; children: ReactNode }): ReactNode {
  return (
    <>
      <span className="text-meta text-fg-3">{label}</span>
      {children}
    </>
  );
}
