'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Clapperboard, Layers, Link2, MonitorPlay, PlugZap, RefreshCw, Swords, TriangleAlert } from 'lucide-react';
import {
  LIBRARY_PARSE_STATES,
  LIBRARY_SOURCES,
  analyzeLibraryDemo,
  deleteLibraryDemo,
  isLibraryNotFound,
  isLibraryOffline,
  linkAllLibraryJobs,
  listLibraryDemos,
  listLibraryMatches,
  listUnlinkedLibraryJobs,
  type LibraryMatch,
} from '@/lib/api/library';
import { LIBRARY_SERIES_HREF, LONG_VIDEOS_HREF, MATCHES_QUERY, PRODUCE_FORMAT, matchHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  DATE_RANGES,
  PARSE_STATE_LABEL,
  SOURCE_LABEL,
  buildPartidaRows,
  demoErrorLabel,
  inDateRange,
  mapChips,
  matchScoreMargin,
  matchTitle,
  partidaDate,
  scoreline,
  teamFilterOptions,
  type MapChip,
  type Outcome,
  type PartidaRow,
} from '@/lib/library/partidas';
import { FILTER_ANY, SEARCH_FILTER, filterRows, type TableViewState } from '@/lib/records/table-view';
import { startPollLoop } from '@/lib/poll-loop';
import { cn } from '@/lib/utils';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { DemoImport } from '@/components/matches/demo-import';
import { OpenInCrearButton } from '@/components/matches/open-in-crear-button';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

const STATE_OPTIONS = LIBRARY_PARSE_STATES.map((state) => ({ value: state, label: PARSE_STATE_LABEL[state] }));
const SOURCE_OPTIONS = LIBRARY_SOURCES.map((source) => ({ value: source, label: SOURCE_LABEL[source] }));
const DATE_OPTIONS = DATE_RANGES.map((range) => ({ value: range.value, label: range.label }));

const FAST_POLL_MS = 3_000;
const IDLE_POLL_MS = 30_000;

// Radix refuses an empty item value, so "every map" travels as this sentinel inside the chips only.
const ALL_MAPS = '__all__';

const OUTCOME_CLASS: Record<Outcome, string> = {
  win: 'font-semibold text-fg-1',
  loss: 'text-fg-3',
  draw: 'text-fg-2',
};

function isPending(row: PartidaRow): boolean {
  return row.demo.state === 'queued' || row.demo.state === 'parsing';
}

/** "Vitality 13 : 7 MOUZ", the winner in full ink and the loser dimmed. */
function Scoreline({ match }: { match: LibraryMatch }): ReactNode {
  const [left, right] = scoreline(match);
  return (
    <span title={matchTitle(match)} className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3">
      <span className={cn('truncate text-right', OUTCOME_CLASS[left.outcome])}>{left.name}</span>
      <span className="text-body whitespace-nowrap tabular-nums">
        <span className={OUTCOME_CLASS[left.outcome]}>{left.score}</span>
        <span aria-hidden className="px-1.5 text-fg-3">:</span>
        <span className="sr-only"> a </span>
        <span className={OUTCOME_CLASS[right.outcome]}>{right.score}</span>
      </span>
      <span className={cn('truncate', OUTCOME_CLASS[right.outcome])}>{right.name}</span>
    </span>
  );
}

/** Why a row has no score yet: still in the queue, being analysed, or the reason the scan failed. */
function DemoStatus({ row }: { row: PartidaRow }): ReactNode {
  const { demo } = row;
  if (demo.state !== 'failed') return <span className="text-fg-3">{PARSE_STATE_LABEL[demo.state]}</span>;
  const reason = demoErrorLabel(demo.error_code);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {/* Focusable so the full reason also opens from the keyboard. */}
        <span tabIndex={0} className={cn('block truncate rounded-sm text-destructive', FOCUS_RING)}>
          {reason}
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="start" className="max-w-sm">
        {reason}
      </TooltipContent>
    </Tooltip>
  );
}

type RowHandlers = { onRetry: (demoId: string) => void; onRemoved: () => void };

/** A demo already gone counts as removed: the row only has to leave the list. */
async function removeDemo(demoId: string): Promise<void> {
  try {
    await deleteLibraryDemo(demoId);
  } catch (error) {
    if (!isLibraryNotFound(error)) throw error;
  }
}

/** A ready row offers its Short; a failed one, a new try or its removal; a match still re-analysing says so. */
function RowActions({ row, handlers }: { row: PartidaRow; handlers: RowHandlers }): ReactNode {
  const { demo, match } = row;
  if (demo.state === 'failed') {
    return (
      <span className="flex items-center justify-end gap-2">
        <Button type="button" variant="outline" size="xs" onClick={() => handlers.onRetry(demo.id)}>
          <RefreshCw aria-hidden />
          Reintentar
        </Button>
        <DeleteMatchButton label={match ? prettyMapName(match.map) : demo.file_name} onConfirm={() => removeDemo(demo.id)} onDeleted={handlers.onRemoved} />
      </span>
    );
  }
  if (match === null || demo.source === 'shared') return null;
  if (demo.state !== 'ready') return <span className="block text-right whitespace-nowrap text-fg-3">{PARSE_STATE_LABEL[demo.state]}</span>;
  return (
    <span className="flex justify-end">
      <OpenInCrearButton request={{ demo, format: PRODUCE_FORMAT.short }} variant="ghost" size="xs">
        <Clapperboard aria-hidden />
        Crear Short
      </OpenInCrearButton>
    </span>
  );
}

function columns(handlers: RowHandlers): DataTableColumn<PartidaRow>[] {
  return [
    {
      id: 'map',
      header: 'Mapa',
      required: true,
      // Until the scan yields a match, the file name is the only handle the row has.
      cell: ({ match, demo }) => {
        const failed = demo.state === 'failed' ? <TriangleAlert aria-hidden className="size-4 shrink-0 text-destructive" /> : null;
        if (match) {
          return (
            <span className="flex items-center gap-2 font-semibold">
              {failed}
              {prettyMapName(match.map)}
            </span>
          );
        }
        return (
          <span className="flex max-w-48 items-center gap-2 text-fg-2" title={demo.file_name}>
            {failed}
            <span className="truncate">{demo.file_name}</span>
          </span>
        );
      },
      sortValue: ({ match, demo }) => (match ? prettyMapName(match.map) : demo.file_name),
    },
    {
      id: 'date',
      header: 'Fecha',
      cell: (row) => <span className="whitespace-nowrap text-fg-2">{formatShortDate(partidaDate(row))}</span>,
      sortValue: (row) => Date.parse(partidaDate(row)) || null,
    },
    {
      id: 'score',
      header: 'Marcador',
      // A failed scan says why even over the match of an earlier analysis; a row without a match says why it has none.
      cell: (row) => (row.match && row.demo.state !== 'failed' ? <Scoreline match={row.match} /> : <DemoStatus row={row} />),
      sortValue: ({ match }) => (match ? matchScoreMargin(match) : null),
      // The flexible column: it takes the width the others leave and truncates, so the table never outgrows the page.
      className: 'w-full max-w-0',
    },
    {
      id: 'rounds',
      header: 'Rondas',
      cell: ({ match }) => (match ? <span className="tabular-nums">{match.round_count}</span> : null),
      sortValue: ({ match }) => match?.round_count ?? null,
    },
    {
      id: 'source',
      header: 'Fuente',
      cell: ({ demo }) => (
        <span className={cn('whitespace-nowrap', demo.source === 'unknown' ? 'text-fg-3' : 'text-fg-2')}>{SOURCE_LABEL[demo.source]}</span>
      ),
      sortValue: ({ demo }) => SOURCE_LABEL[demo.source],
    },
    {
      id: 'tags',
      header: 'Etiquetas',
      cell: ({ demo }) => (
        <span className="block max-w-40 truncate text-fg-2" title={demo.tags.join(', ')}>
          {demo.tags.join(', ')}
        </span>
      ),
    },
    {
      id: 'file',
      header: 'Archivo',
      cell: ({ demo }) => (
        <span className="block max-w-64 truncate font-mono text-meta text-fg-3" title={demo.file_name}>
          {demo.file_name}
        </span>
      ),
      sortValue: ({ demo }) => demo.file_name,
    },
    {
      id: 'actions',
      header: '',
      required: true,
      className: 'w-px',
      cell: (row) => <RowActions row={row} handlers={handlers} />,
    },
  ];
}

/** "Todos los mapas 7", then one chip per map with the partidas the other filters leave on it. */
function MapChips({ chips, total, value, onChange }: { chips: readonly MapChip[]; total: number; value: string; onChange: (map: string) => void }): ReactNode {
  return (
    <ToggleGroup
      type="single"
      variant="filter"
      spacing={2}
      aria-label="Mapa"
      className="flex-wrap"
      value={value === FILTER_ANY ? ALL_MAPS : value}
      // Pressing the chosen chip again deselects it, which reads as going back to every map.
      onValueChange={(next: string) => onChange(next === ALL_MAPS ? FILTER_ANY : next)}
    >
      <ToggleGroupItem value={ALL_MAPS}>
        Todos los mapas <span className="font-normal text-fg-3">{total}</span>
      </ToggleGroupItem>
      {chips.map((chip) => (
        <ToggleGroupItem key={chip.value} value={chip.value}>
          {chip.label} <span className="font-normal text-fg-3">{chip.count}</span>
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}

function initialView(search: URLSearchParams): TableViewState {
  return {
    sort: { column: 'date', direction: 'desc' },
    // Tags stay searchable; the column comes back from Columnas once demos carry them.
    hidden: ['file', 'tags'],
    filters: {
      [SEARCH_FILTER]: FILTER_ANY,
      map: search.get(MATCHES_QUERY.map) ?? FILTER_ANY,
      team: search.get(MATCHES_QUERY.team) ?? FILTER_ANY,
      date: FILTER_ANY,
      source: FILTER_ANY,
      state: FILTER_ANY,
    },
  };
}

type Loaded = { rows: PartidaRow[]; unlinkedJobs: number };

async function loadLibrary(): Promise<Loaded> {
  const [demos, matches, unlinked] = await Promise.all([
    listLibraryDemos(),
    listLibraryMatches(),
    // The Crear backlog is a hint; the table stands without it.
    listUnlinkedLibraryJobs().catch(() => []),
  ]);
  return { rows: buildPartidaRows(demos, matches), unlinkedJobs: unlinked.length };
}

/** Title and page actions. `PartidasSkeleton` passes a placeholder where the live importer goes. */
function PartidasHeader({ importer }: { importer: ReactNode }): ReactNode {
  return (
    <StudioPageHeader
      title="Partidas"
      description="Las demos que has importado. Abre una para ver cómo fue, ronda a ronda, y sacar sus vídeos."
      actions={
        <div className="flex items-center gap-2">
          <Button asChild variant="outline" size="sm">
            <Link href={LIBRARY_SERIES_HREF}>
              <Layers aria-hidden />
              Series y veto
            </Link>
          </Button>
          {importer}
        </div>
      }
    />
  );
}

/** Partidas: every demo in the library with its match, filterable, importable, and live while scans run. */
export function MatchLibrary(): ReactNode {
  const search = useSearchParams();
  // Another page's filter link is a new starting view, not an edit of this one.
  return <MatchLibraryTable key={search.toString()} search={search} />;
}

function MatchLibraryTable({ search }: { search: URLSearchParams }): ReactNode {
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  // Why the last library read failed: the service is absent, or it answered with an error.
  const [readError, setReadError] = useState<'offline' | 'error' | null>(null);
  const [linking, setLinking] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const view = useTableView('partidas', initialView(search));
  const pendingRef = useRef(false);
  const pollRef = useRef<() => void>(() => {});

  const load = useCallback(async () => {
    try {
      const next = await loadLibrary();
      setLoaded(next);
      setReadError(null);
      pendingRef.current = next.rows.some(isPending);
    } catch (error) {
      setReadError(isLibraryOffline(error) ? 'offline' : 'error');
      setLoaded((current) => current ?? { rows: [], unlinkedJobs: 0 });
    }
  }, []);

  useEffect(() => {
    const stop = startPollLoop({
      tick: async () => {
        await load();
        return pendingRef.current ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    pollRef.current = () => void load();
    return stop;
  }, [load]);

  const retry = useCallback(async (demoId: string) => {
    try {
      await analyzeLibraryDemo(demoId);
    } catch {
      setNotice('No se pudo volver a poner la demo en cola.');
    }
    pollRef.current();
  }, []);

  async function linkCrearDemos(): Promise<void> {
    setLinking(true);
    try {
      const demos = await linkAllLibraryJobs();
      setNotice(demos.length === 1 ? 'Se añadió 1 demo de Crear a la biblioteca.' : `Se añadieron ${demos.length} demos de Crear a la biblioteca.`);
    } catch {
      setNotice('No se pudieron añadir las demos de Crear. Vuelve a intentarlo.');
    } finally {
      setLinking(false);
      pollRef.current();
    }
  }

  // The header sits first in every branch, so the importer keeps its open dialog when the first demo fills the table.
  const header = <PartidasHeader importer={<DemoImport onImported={() => pollRef.current()} />} />;

  if (loaded === null) {
    return (
      <>
        {header}
        <LibrarySkeleton />
      </>
    );
  }

  const { rows, unlinkedJobs } = loaded;
  const backlog = unlinkedJobs > 0 ? <CrearBacklog count={unlinkedJobs} busy={linking} onLink={() => void linkCrearDemos()} /> : null;
  const noticeLine = notice ? <p role="status" className="text-body-sm text-fg-2">{notice}</p> : null;

  if (rows.length === 0) {
    let empty: ReactNode;
    if (readError === 'offline') {
      empty = (
        <EmptyState
          icon={PlugZap}
          title="Servicio local sin conexión"
          description="No se pudo leer la biblioteca de partidas. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
          actions={
            <Button type="button" variant="outline" size="sm" onClick={() => pollRef.current()}>
              Reintentar
            </Button>
          }
        />
      );
    } else if (readError === 'error') {
      empty = (
        <EmptyState
          icon={TriangleAlert}
          title="No se pudo leer la biblioteca"
          description="El servicio local respondió con un error al listar las partidas. Vuelve a intentarlo."
          actions={
            <Button type="button" variant="outline" size="sm" onClick={() => pollRef.current()}>
              Reintentar
            </Button>
          }
        />
      );
    } else {
      empty = (
        <div className="flex flex-col gap-3">
          {backlog}
          <EmptyState
            icon={Swords}
            title="Aún no hay partidas"
            description="Importa demos de CS2 (.dem o .dem.zst) de FACEIT, de matchmaking o de tus scrims. Las que cargues en Crear también entran aquí."
            preview={[
              'Varias demos a la vez; una demo repetida se detecta por su huella y no se duplica.',
              'Cada demo se analiza en este PC: equipos, jugadores, rondas y táctica.',
              'Desde la ficha de una partida puedes crear un Short o una demo completa.',
            ]}
            actions={
              <Button asChild variant="outline" size="sm">
                <Link href={LONG_VIDEOS_HREF}>
                  <MonitorPlay aria-hidden />
                  Ir a Crear
                </Link>
              </Button>
            }
          />
          {noticeLine}
        </div>
      );
    }
    return (
      <>
        {header}
        {empty}
      </>
    );
  }

  const now = Date.now();
  const selectedMap = view.state.filters.map ?? FILTER_ANY;
  // Every filter but the map: the chips count over these rows, and the map chip then narrows them.
  const facet = filterRows(rows, { ...view.state.filters, map: FILTER_ANY }, {
    fields: {
      team: ({ match }) => match?.teams.map((team) => team.team_id ?? '') ?? [],
      source: ({ demo }) => demo.source,
      state: ({ demo }) => demo.state,
    },
    searchText: ({ demo, match }) =>
      [match ? prettyMapName(match.map) : '', match ? matchTitle(match) : '', demo.file_name, ...demo.tags].join(' '),
  }).filter((row) => inDateRange(view.state.filters.date ?? FILTER_ANY, partidaDate(row), now));
  const shown = selectedMap === FILTER_ANY ? facet : facet.filter(({ match }) => match?.map === selectedMap);
  const chips = mapChips({ library: rows.map(({ match }) => match), listed: facet.map(({ match }) => match), selected: selectedMap });
  const pending = rows.filter(isPending).length;
  const handlers: RowHandlers = { onRetry: (demoId) => void retry(demoId), onRemoved: () => pollRef.current() };

  return (
    <>
      {header}
      <div className="flex flex-col gap-3">
        {pending > 0 || noticeLine !== null || backlog !== null ? (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            {pending > 0 ? (
              <p role="status" className="text-body-sm text-fg-2">
                {pending === 1 ? 'Analizando 1 demo' : `Analizando ${pending} demos`}
              </p>
            ) : null}
            {noticeLine}
            {backlog}
          </div>
        ) : null}
        {readError !== null ? (
          <p role="alert" className="text-body-sm text-destructive">
            {readError === 'offline'
              ? 'Se perdió la conexión con el servicio local; la tabla muestra la última lectura.'
              : 'El servicio local respondió con un error; la tabla muestra la última lectura.'}
          </p>
        ) : null}
        {chips.length > 1 || selectedMap !== FILTER_ANY ? (
          <MapChips chips={chips} total={facet.length} value={selectedMap} onChange={(map) => view.setFilter('map', map)} />
        ) : null}
        <DataTable
          label="Partidas"
          rows={shown}
          columns={columns(handlers)}
          rowKey={(row) => row.demo.id}
          rowHref={(row) => (row.match ? matchHref(row.match.id) : undefined)}
          view={view}
          toolbar={
            <FilterBar
              className="flex min-w-0 flex-1 flex-wrap items-center gap-2"
              stacked
              values={view.state.filters}
              onChange={view.setFilter}
              onReset={view.resetFilters}
              searchPlaceholder="Buscar por mapa, equipo, archivo o etiqueta"
              filters={[
                { id: 'team', label: 'Equipo', options: teamFilterOptions(rows.map(({ match }) => match)) },
                { id: 'date', label: 'Fecha', options: DATE_OPTIONS },
                { id: 'source', label: 'Fuente', options: SOURCE_OPTIONS },
                { id: 'state', label: 'Estado', options: STATE_OPTIONS },
              ]}
              count={{ shown: shown.length, total: rows.length, noun: rows.length === 1 ? 'partida' : 'partidas' }}
            />
          }
          empty={<p className="text-body-sm text-fg-2">Ninguna partida cumple estos filtros.</p>}
        />
      </div>
    </>
  );
}

function CrearBacklog({ count, busy, onLink }: { count: number; busy: boolean; onLink: () => void }): ReactNode {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <p className="text-body-sm text-fg-2">
        {count === 1
          ? 'Una demo que cargaste en Crear aún no está en la biblioteca.'
          : `${count} demos que cargaste en Crear aún no están en la biblioteca.`}{' '}
        Al añadirlas se analizan en este PC, una tras otra.
      </p>
      <Button type="button" variant="outline" size="xs" disabled={busy} onClick={onLink}>
        <Link2 aria-hidden />
        {busy ? 'Añadiendo' : 'Añadir a la biblioteca'}
      </Button>
    </div>
  );
}

const SKELETON_CHIPS = ['w-36', 'w-24', 'w-24', 'w-20'] as const;
const SKELETON_FILTERS = ['team', 'date', 'source', 'state'] as const;
const SKELETON_ROWS = 3;
// One width per column of the loaded table: Mapa, Fecha, Marcador, Rondas, Fuente and the row action.
const SKELETON_CELLS = ['w-14', 'w-10', 'w-72', 'w-8', 'w-16', 'w-20'] as const;

/** The page while the route or its first read loads: the real header, the importer slot and the table skeleton. */
export function PartidasSkeleton(): ReactNode {
  return (
    <>
      <PartidasHeader importer={<Skeleton className="h-10 w-40" />} />
      <LibrarySkeleton />
    </>
  );
}

/** The loaded toolbar and table drawn at their own heights, so the first read does not shift the page. */
function LibrarySkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando partidas" className="flex flex-col gap-2">
      <div className="mb-1 flex flex-wrap gap-2">
        {SKELETON_CHIPS.map((width, index) => (
          <Skeleton key={index} className={cn('h-11', width)} />
        ))}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Skeleton className="h-9 w-80 max-w-full" />
        <div className="flex gap-2">
          <Skeleton className="h-9 w-20" />
          <Skeleton className="h-9 w-24" />
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        {SKELETON_FILTERS.map((id) => (
          <Skeleton key={id} className="h-9 w-36" />
        ))}
      </div>
      <div className="studio-panel overflow-hidden">
        <div className="flex h-9 items-center gap-6 border-b border-border bg-surface-2 px-3">
          {SKELETON_CELLS.map((width, index) => (
            <Skeleton key={index} className={cn('h-2.5 shrink-0 opacity-60 shadow-none', width)} />
          ))}
        </div>
        {Array.from({ length: SKELETON_ROWS }, (_, row) => (
          <div key={row} className="flex h-10 items-center gap-6 border-b border-border-subtle px-3 last:border-b-0">
            {SKELETON_CELLS.map((width, index) => (
              <Skeleton key={index} className={cn('h-3 shrink-0 shadow-none', width)} />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
