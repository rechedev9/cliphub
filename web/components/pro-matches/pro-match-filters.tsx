'use client';

import { useState, type ReactNode } from 'react';
import { Check, ChevronDown, Clock, Layers, ListFilter, Plus, Users } from 'lucide-react';
import { PRO_PERIODS, DEFAULT_PRO_QUERY, type ProMatchQuery, type ProMatchesPage, type ProTeamCount } from '@/lib/api/pro-matches';
import { prettyMapName } from '@/lib/format';
import { cn } from '@/lib/utils';
import { MapThumb } from '@/components/analysis/map-thumb';
import { FOCUS_RING } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Segmented } from '@/components/ui/segmented';

const TRIGGER = cn(
  'inline-flex h-8 items-center gap-2 rounded-md border border-border-strong bg-transparent px-2.5 text-body-sm font-medium text-fg-1',
  'transition-colors hover:bg-surface-3 [&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-fg-3',
  FOCUS_RING,
);
const CHOSEN = 'border-primary';
const ROW = cn(
  'flex min-h-8 w-full items-center gap-2 rounded-sm px-2 py-1 text-left text-body-sm text-popover-foreground transition-colors hover:bg-surface-5',
  FOCUS_RING,
);

function PopoverHeader({ children }: { children: ReactNode }): ReactNode {
  return <p className="flex items-center gap-1.5 px-2 pt-2 pb-1 text-meta font-medium tracking-wider text-fg-3 uppercase [&_svg]:size-3">{children}</p>;
}

type TeamPickerProps = {
  label: string;
  any: string;
  value: string;
  teams: readonly ProTeamCount[];
  onChange: (team: string) => void;
};

/** A team dropdown with a search box and the teams that played in the period; red-tinted once one is chosen. */
function TeamPicker({ label, any, value, teams, onChange }: TeamPickerProps): ReactNode {
  const [open, setOpen] = useState(false);
  const [term, setTerm] = useState('');
  const chosen = teams.find((team) => String(team.id) === value);
  const shown = teams.filter((team) => team.name.toLocaleLowerCase('es').includes(term.trim().toLocaleLowerCase('es')));
  const pick = (team: string): void => {
    onChange(team);
    setOpen(false);
    setTerm('');
  };
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button type="button" aria-label={label} className={cn(TRIGGER, 'w-48 justify-between', value !== '' && 'border-destructive/60 bg-destructive/10')}>
          <Users aria-hidden />
          <span className="min-w-0 flex-1 truncate text-left">{value === '' ? any : (chosen?.name ?? value)}</span>
          <ChevronDown aria-hidden />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0">
        <div className="border-b border-border p-2">
          <Input type="search" aria-label={`Buscar ${label.toLocaleLowerCase('es')}`} placeholder="Buscar equipo" value={term} onChange={(event) => setTerm(event.target.value)} />
        </div>
        <div className="max-h-72 overflow-y-auto p-1">
          <button type="button" className={ROW} onClick={() => pick('')}>
            <span className="flex-1">{any}</span>
            {value === '' ? <Check aria-hidden className="size-4 text-primary" /> : null}
          </button>
          <PopoverHeader>
            <Clock aria-hidden />
            Recientes
          </PopoverHeader>
          {shown.length === 0 ? <p className="px-2 py-2 text-body-sm text-fg-3">Ningún equipo se llama así.</p> : null}
          {shown.map((team) => (
            <button key={team.id} type="button" className={cn(ROW, String(team.id) === value && 'text-primary')} onClick={() => pick(String(team.id))}>
              <span className="min-w-0 flex-1 truncate">{team.name}</span>
              {team.rank ? <span className="text-meta text-fg-3 tabular-nums">#{team.rank}</span> : null}
              <span className="w-6 text-right text-meta text-fg-3 tabular-nums">{team.matches}</span>
              {String(team.id) === value ? <Check aria-hidden className="size-4 text-primary" /> : null}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

function MapPicker({ value, maps, onChange }: { value: string; maps: ProMatchesPage['facets']['maps']; onChange: (map: string) => void }): ReactNode {
  const [open, setOpen] = useState(false);
  // A chosen map stays listed even when the period no longer has it.
  const listed = value !== '' && !maps.some((map) => map.name === value) ? [{ name: value, matches: 0 }, ...maps] : maps;
  const pick = (map: string): void => {
    onChange(map);
    setOpen(false);
  };
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button type="button" aria-label="Mapa" className={cn(TRIGGER, 'w-44 justify-between', value !== '' && CHOSEN)}>
          <Layers aria-hidden />
          <span className="min-w-0 flex-1 truncate text-left">{value === '' ? 'Todos los mapas' : prettyMapName(value)}</span>
          <ChevronDown aria-hidden />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-60 p-1">
        <button type="button" className={ROW} onClick={() => pick('')}>
          <span className="flex-1">Todos los mapas</span>
          {value === '' ? <Check aria-hidden className="size-4 text-primary" /> : null}
        </button>
        {listed.map((map) => (
          <button key={map.name} type="button" className={cn(ROW, 'relative mt-0.5 h-9 overflow-hidden p-0')} onClick={() => pick(map.name)}>
            <MapThumb map={map.name} className="absolute inset-0 size-full rounded-none" crop={1.5} />
            <span aria-hidden className="absolute inset-0 bg-linear-to-r from-black/75 via-black/40 to-black/60" />
            <span className="relative flex w-full items-center gap-2 px-2 text-white">
              <span className={cn('flex-1 font-semibold', value === map.name && 'text-primary')}>{prettyMapName(map.name)}</span>
              <span className="text-meta text-white/70 tabular-nums">{map.matches}</span>
              {value === map.name ? <Check aria-hidden className="size-4 text-primary" /> : null}
            </span>
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}

/** The period choices, plus the one an address or an assistant link asked for. */
function periodOptions(days: number): { value: string; label: string }[] {
  const options = PRO_PERIODS.map((period) => ({ value: String(period.days), label: period.label }));
  return options.some((option) => option.value === String(days)) ? options : [...options, { value: String(days), label: `${days} días` }];
}

function filtersLabel(active: number): string {
  if (active === 0) return 'Filtros';
  return active === 1 ? '1 filtro' : `${active} filtros`;
}

/** The period and the event, the two filters the catalog has besides team and map. */
function FiltersMenu({ page, query, onChange }: { page: ProMatchesPage; query: ProMatchQuery; onChange: (patch: Partial<ProMatchQuery>) => void }): ReactNode {
  const active = (query.event !== '' ? 1 : 0) + (query.days !== DEFAULT_PRO_QUERY.days ? 1 : 0);
  const periods = periodOptions(query.days);
  const events = query.event !== '' && !page.facets.events.some((event) => event.name === query.event) ? [{ name: query.event, matches: 0 }, ...page.facets.events] : page.facets.events;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button type="button" className={cn(TRIGGER, active > 0 && CHOSEN)}>
          {active > 0 ? <ListFilter aria-hidden className="text-primary!" /> : <Plus aria-hidden />}
          {filtersLabel(active)}
          <ChevronDown aria-hidden />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0">
        <PopoverHeader>Periodo</PopoverHeader>
        <div className="px-2 pb-2">
          <Segmented label="Periodo" value={String(query.days)} onChange={(days) => onChange({ days: Number(days) })} options={periods} />
        </div>
        <div className="border-t border-border">
          <PopoverHeader>Evento</PopoverHeader>
          <div className="max-h-60 overflow-y-auto p-1 pt-0">
            <button type="button" className={ROW} onClick={() => onChange({ event: '' })}>
              <span className="flex-1">Todos los eventos</span>
              {query.event === '' ? <Check aria-hidden className="size-4 text-primary" /> : null}
            </button>
            {events.map((event) => (
              <button key={event.name} type="button" className={cn(ROW, query.event === event.name && 'text-primary')} onClick={() => onChange({ event: event.name })}>
                <span className="min-w-0 flex-1 truncate">{event.name}</span>
                <span className="text-meta text-fg-3 tabular-nums">{event.matches}</span>
                {query.event === event.name ? <Check aria-hidden className="size-4 text-primary" /> : null}
              </button>
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}

export type ProFiltersProps = {
  page: ProMatchesPage;
  query: ProMatchQuery;
  /** The second team of "VS."; empty when none. It narrows the page already read, as the catalog filters one team only. */
  rival: string;
  onChange: (patch: Partial<ProMatchQuery>) => void;
  onRivalChange: (team: string) => void;
};

/** The Públicas toolbar: team, "VS." and a second team, map and the Filtros menu. */
export function ProFilters({ page, query, rival, onChange, onRivalChange }: ProFiltersProps): ReactNode {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <TeamPicker
        label="Equipo"
        any="Cualquier equipo"
        value={query.team}
        teams={page.facets.teams}
        onChange={(team) => {
          onChange({ team });
          if (team === '') onRivalChange('');
        }}
      />
      {query.team === '' ? null : (
        <>
          <span className="text-meta font-semibold tracking-wider text-fg-3">VS.</span>
          <TeamPicker label="Rival" any="Cualquier equipo" value={rival} teams={page.facets.teams.filter((team) => String(team.id) !== query.team)} onChange={onRivalChange} />
        </>
      )}
      <MapPicker value={query.map} maps={page.facets.maps} onChange={(map) => onChange({ map })} />
      <FiltersMenu page={page} query={query} onChange={onChange} />
    </div>
  );
}
