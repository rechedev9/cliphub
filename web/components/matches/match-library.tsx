'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Layers, Link2, MonitorPlay, PlugZap, RefreshCw, Swords, TriangleAlert } from 'lucide-react';
import {
  LIBRARY_PARSE_STATES,
  LIBRARY_SOURCES,
  analyzeLibraryDemo,
  isLibraryOffline,
  linkAllLibraryJobs,
  listLibraryDemos,
  listLibraryMatches,
  listUnlinkedLibraryJobs,
  type LibraryParseState,
} from '@/lib/api/library';
import { LIBRARY_SERIES_HREF, LONG_VIDEOS_HREF, MATCHES_QUERY, matchHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  DATE_RANGES,
  PARSE_STATE_LABEL,
  SOURCE_LABEL,
  buildPartidaRows,
  demoErrorLabel,
  inDateRange,
  matchScoreLabel,
  matchScoreMargin,
  matchTitle,
  partidaDate,
  teamFilterOptions,
  type PartidaRow,
} from '@/lib/library/partidas';
import { mapFilterOptions } from '@/lib/records/matches';
import { FILTER_ANY, SEARCH_FILTER, filterRows, type TableViewState } from '@/lib/records/table-view';
import { startPollLoop } from '@/lib/poll-loop';
import { cn } from '@/lib/utils';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { DemoImport } from '@/components/matches/demo-import';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

const STATE_TONE: Record<LibraryParseState, StatusTagTone> = {
  queued: 'neutral',
  parsing: 'primary',
  ready: 'success',
  failed: 'danger',
};

const STATE_OPTIONS = LIBRARY_PARSE_STATES.map((state) => ({ value: state, label: PARSE_STATE_LABEL[state] }));
const SOURCE_OPTIONS = LIBRARY_SOURCES.map((source) => ({ value: source, label: SOURCE_LABEL[source] }));
const DATE_OPTIONS = DATE_RANGES.map((range) => ({ value: range.value, label: range.label }));

const FAST_POLL_MS = 3_000;
const IDLE_POLL_MS = 30_000;

function isPending(row: PartidaRow): boolean {
  return row.demo.state === 'queued' || row.demo.state === 'parsing';
}

function columns(onRetry: (demoId: string) => void): DataTableColumn<PartidaRow>[] {
  return [
    {
      id: 'map',
      header: 'Mapa',
      required: true,
      // Until the scan yields a match, the file name is the only handle the row has.
      cell: ({ match, demo }) =>
        match ? prettyMapName(match.map) : <span className="block max-w-40 truncate text-fg-2" title={demo.file_name}>{demo.file_name}</span>,
      sortValue: ({ match, demo }) => (match ? prettyMapName(match.map) : demo.file_name),
    },
    {
      id: 'date',
      header: 'Fecha',
      cell: (row) => <span className="whitespace-nowrap text-fg-2">{formatShortDate(partidaDate(row))}</span>,
      sortValue: (row) => Date.parse(partidaDate(row)) || null,
    },
    {
      id: 'teams',
      header: 'Equipos',
      // A row without a match has no teams to show, so the cell says why: still pending, or what the scan hit.
      cell: ({ match, demo }) => {
        if (match) {
          const title = matchTitle(match);
          return <span className="block truncate" title={title}>{title}</span>;
        }
        if (demo.state !== 'failed') return <span className="text-fg-3">Pendiente</span>;
        const reason = demoErrorLabel(demo.error_code);
        return (
          <Tooltip>
            <TooltipTrigger asChild>
              {/* Focusable so the full reason also opens from the keyboard. */}
              <span tabIndex={0} className={cn('block truncate rounded-sm text-meta text-destructive', FOCUS_RING)}>
                {reason}
              </span>
            </TooltipTrigger>
            <TooltipContent side="bottom" align="start" className="max-w-sm">
              {reason}
            </TooltipContent>
          </Tooltip>
        );
      },
      sortValue: ({ match }) => (match ? matchTitle(match) : null),
      // The flexible column: it takes the width the others leave and truncates, so the table never outgrows the page.
      className: 'w-full max-w-0',
    },
    {
      id: 'score',
      header: 'Marcador',
      cell: ({ match }) => (match ? <span className="whitespace-nowrap tabular-nums">{matchScoreLabel(match)}</span> : null),
      sortValue: ({ match }) => (match ? matchScoreMargin(match) : null),
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
      id: 'state',
      header: 'Estado',
      cell: ({ demo }) => (
        <span className="flex items-center gap-2">
          <StatusTag tone={STATE_TONE[demo.state]} dot>
            {PARSE_STATE_LABEL[demo.state]}
          </StatusTag>
          {demo.state === 'failed' ? (
            <Button type="button" variant="ghost" size="xs" onClick={() => onRetry(demo.id)}>
              <RefreshCw aria-hidden />
              Reintentar
            </Button>
          ) : null}
        </span>
      ),
      sortValue: ({ demo }) => PARSE_STATE_LABEL[demo.state],
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
  ];
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

/** Title and page actions. The page's Suspense fallback passes a placeholder where the live importer goes. */
export function PartidasHeader({ importer }: { importer: ReactNode }): ReactNode {
  return (
    <StudioPageHeader
      title="Partidas"
      description="La biblioteca común de demos: la misma partida sirve para crear vídeos y para analizarla ronda a ronda."
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
  const shown = filterRows(rows, view.state.filters, {
    fields: {
      map: ({ match }) => match?.map ?? '',
      team: ({ match }) => match?.teams.map((team) => team.team_id ?? '') ?? [],
      source: ({ demo }) => demo.source,
      state: ({ demo }) => demo.state,
    },
    searchText: ({ demo, match }) =>
      [match ? prettyMapName(match.map) : '', match ? matchTitle(match) : '', demo.file_name, ...demo.tags].join(' '),
  }).filter((row) => inDateRange(view.state.filters.date ?? FILTER_ANY, partidaDate(row), now));
  const pending = rows.filter(isPending).length;

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
        <DataTable
          label="Partidas"
          rows={shown}
          columns={columns((demoId) => void retry(demoId))}
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
                { id: 'map', label: 'Mapa', options: mapFilterOptions(rows.flatMap(({ match }) => (match ? [match] : []))) },
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

/** Same blocks and heights as the loaded table, so the first read does not shift the page. */
function LibrarySkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando partidas" className="flex flex-col gap-3">
      <Skeleton className="h-9 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}
