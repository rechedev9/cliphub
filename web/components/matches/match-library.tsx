'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Link2, MonitorPlay, PlugZap, Swords, TriangleAlert } from 'lucide-react';
import {
  LIBRARY_PARSE_STATES,
  LIBRARY_SOURCES,
  analyzeLibraryDemo,
  isLibraryOffline,
  linkAllLibraryJobs,
  listLibraryDemos,
  listLibraryMatches,
  listLibrarySeries,
  listUnlinkedLibraryJobs,
  patchLibraryDemo,
  type LibrarySource,
} from '@/lib/api/library';
import type { LibrarySeries } from '@/lib/api/veto';
import { listVoiceSummaries, type VoiceSummary } from '@/lib/api/voice';
import { LONG_VIDEOS_HREF, MATCHES_QUERY, matchHref } from '@/lib/clips/routes';
import { bulkDeleteSummary, deleteErrorMessage } from '@/lib/delete-error';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  eventOptions,
  groupPartidas,
  hasPov,
  libraryTags,
  ownTeamId,
  ownTeamName,
  parseListView,
  rowEvent,
  rowKind,
  seriesIndex,
  withListView,
  withoutListView,
  type LibraryListView,
} from '@/lib/library/partida-groups';
import {
  DATE_RANGES,
  PARSE_STATE_LABEL,
  SOURCE_LABEL,
  buildPartidaRows,
  inDateRange,
  mapChips,
  matchScoreMargin,
  matchTitle,
  partidaDate,
  teamFilterOptions,
  type MapChip,
  type PartidaRow,
} from '@/lib/library/partidas';
import { loadPovJobs } from '@/lib/library/pov-jobs';
import { voiceIndex, type VoiceByMatch } from '@/lib/library/voice-summary';
import type { CountNoun } from '@/lib/records/selection';
import { FILTER_ANY, SEARCH_FILTER, filterRows, type TableViewState } from '@/lib/records/table-view';
import { startPollLoop } from '@/lib/poll-loop';
import { settleEach, withoutKeys } from '@/lib/settle-each';
import { cn } from '@/lib/utils';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { DeleteChosenButton } from '@/components/records/delete-chosen-button';
import { EmptyState } from '@/components/records/empty-state';
import { SelectionBar } from '@/components/records/selection-bar';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { DemoImport } from '@/components/matches/demo-import';
import { LibraryListControls } from '@/components/matches/library-list-controls';
import { PartidaGroupsList } from '@/components/matches/partida-groups-list';
import { PartidaVoice } from '@/components/matches/partida-voice';
import { DemoStatus, RowActions, Scoreline, isPending, removeDemo, type RowHandlers } from '@/components/matches/partida-cells';
import { PartidasFrame } from '@/components/matches/partidas-frame';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

const STATE_OPTIONS = LIBRARY_PARSE_STATES.map((state) => ({ value: state, label: PARSE_STATE_LABEL[state] }));
const SOURCE_OPTIONS = LIBRARY_SOURCES.map((source) => ({ value: source, label: SOURCE_LABEL[source] }));
const DATE_OPTIONS = DATE_RANGES.map((range) => ({ value: range.value, label: range.label }));

const PARTIDA: CountNoun = { one: 'partida', many: 'partidas', feminine: true };

const DELETE_DESCRIPTION =
  'La biblioteca pierde la copia de la demo, el análisis y las correcciones. Los vídeos ya creados y las demos de Crear se quedan. No se puede deshacer.';

function toggled(current: ReadonlySet<string>, key: string): Set<string> {
  const next = new Set(current);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  return next;
}

const FAST_POLL_MS = 3_000;
const IDLE_POLL_MS = 30_000;

// Radix refuses an empty item value, so "every map" travels as this sentinel inside the chips only.
const ALL_MAPS = '__all__';

type VoiceColumn = { byMatch: VoiceByMatch; onResynced: () => void };

function columns(handlers: RowHandlers, voice: VoiceColumn): DataTableColumn<PartidaRow>[] {
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
      id: 'voice',
      header: 'Voz',
      cell: ({ match }) => {
        const summary = match ? voice.byMatch.get(match.id) : undefined;
        return summary ? <PartidaVoice compact summary={summary} onResynced={voice.onResynced} /> : null;
      },
      sortValue: ({ match }) => (match && voice.byMatch.has(match.id) ? 1 : 0),
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
      rival: search.get(MATCHES_QUERY.rival) ?? FILTER_ANY,
      event: search.get(MATCHES_QUERY.event) ?? FILTER_ANY,
      date: FILTER_ANY,
      source: FILTER_ANY,
      state: FILTER_ANY,
    },
  };
}

type Loaded = { rows: PartidaRow[]; series: LibrarySeries[]; unlinkedJobs: number; voices: VoiceSummary[] };

async function loadLibrary(): Promise<Loaded> {
  const [demos, matches, unlinked, series, voices] = await Promise.all([
    listLibraryDemos(),
    listLibraryMatches(),
    // The Crear backlog is a hint; the table stands without it.
    listUnlinkedLibraryJobs().catch(() => []),
    // Without series every partida stands alone, as before.
    listLibrarySeries().catch((): LibrarySeries[] => []),
    // Voice is an icon on the rows; without it they show none.
    listVoiceSummaries().catch((): VoiceSummary[] => []),
  ]);
  return { rows: buildPartidaRows(demos, matches), series, unlinkedJobs: unlinked.length, voices };
}

/** Partidas: every demo in the library with its match, filterable, importable, and live while scans run. */
export function MatchLibrary(): ReactNode {
  const search = useSearchParams();
  // Another page's filter link is a new starting view, not an edit of this one; the list view keys are this page's own.
  return <MatchLibraryTable key={withoutListView(search)} search={search} />;
}

function MatchLibraryTable({ search }: { search: URLSearchParams }): ReactNode {
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  // Why the last library read failed: the service is absent, or it answered with an error.
  const [readError, setReadError] = useState<'offline' | 'error' | null>(null);
  const [linking, setLinking] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [deleting, setDeleting] = useState(false);
  const view = useTableView('partidas', initialView(search));
  const [listView, setListView] = useState<LibraryListView>(() => parseListView(search));
  const [povJobs, setPovJobs] = useState<ReadonlySet<string>>(new Set());
  const pendingRef = useRef(false);
  const pollRef = useRef<() => void>(() => {});
  // Only the newest read may land, so a slow poll that began before an edit cannot undo it.
  const readRef = useRef(0);

  const load = useCallback(async () => {
    const read = ++readRef.current;
    try {
      const next = await loadLibrary();
      if (read !== readRef.current) return;
      setLoaded(next);
      setReadError(null);
      pendingRef.current = next.rows.some(isPending);
    } catch (error) {
      if (read !== readRef.current) return;
      setReadError(isLibraryOffline(error) ? 'offline' : 'error');
      setLoaded((current) => current ?? { rows: [], series: [], unlinkedJobs: 0, voices: [] });
    }
  }, []);

  // The jobs of every demo, joined: the POV read repeats only when the set of jobs changes, not on each poll.
  const jobKey = loaded === null ? '' : loaded.rows.flatMap(({ demo }) => demo.job_ids).sort().join(',');
  useEffect(() => {
    let active = true;
    void loadPovJobs(jobKey === '' ? [] : jobKey.split(',')).then((jobs) => {
      if (active) setPovJobs(jobs);
    });
    return () => {
      active = false;
    };
  }, [jobKey]);

  // The list view lives in the address, so a reload or a shared link keeps it; the history entry is replaced, not added.
  function changeListView(next: LibraryListView): void {
    setListView(next);
    const query = withListView(new URLSearchParams(window.location.search), next);
    window.history.replaceState(null, '', query === '' ? window.location.pathname : `${window.location.pathname}?${query}`);
  }

  const changeTags = useCallback((demoId: string, tags: string[]) => {
    setLoaded((current) =>
      current === null ? current : { ...current, rows: current.rows.map((row) => (row.demo.id === demoId ? { ...row, demo: { ...row.demo, tags } } : row)) },
    );
    pollRef.current();
  }, []);

  async function changeSources(demoIds: readonly string[], source: LibrarySource): Promise<boolean> {
    const { failed } = await settleEach(demoIds, (demoId) => patchLibraryDemo(demoId, { source }));
    setNotice(failed.length > 0 ? 'No se pudo cambiar la fuente de alguna partida.' : null);
    pollRef.current();
    return failed.length === 0;
  }

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

  async function deleteChosen(ids: readonly string[]): Promise<void> {
    setDeleting(true);
    const { done, failed } = await settleEach(ids, removeDemo);
    setNotice(
      bulkDeleteSummary({
        deleted: done.length,
        failed: failed.length,
        noun: PARTIDA,
        reason: failed[0] ? deleteErrorMessage(failed[0].error) : undefined,
      }),
    );
    // Failures and rows picked during the delete stay chosen.
    setSelected((current) => withoutKeys(current, done.map(({ item }) => item)));
    setDeleting(false);
    pollRef.current();
  }

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

  // The frame is the root in every branch, so the importer keeps its open dialog when the first demo fills the table.
  const frame = (children: ReactNode): ReactNode => (
    <PartidasFrame importer={<DemoImport onImported={() => pollRef.current()} />}>{children}</PartidasFrame>
  );

  if (loaded === null) return frame(<LibrarySkeleton />);

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
    return frame(empty);
  }

  const now = Date.now();
  const seriesById = seriesIndex(loaded.series);
  const voices = voiceIndex(loaded.voices);
  // Mi equipo comes from the team profile of #411; until it is read here, the scrims decide.
  const ownTeam = ownTeamId({ rows, explicit: null });
  const selectedMap = view.state.filters.map ?? FILTER_ANY;
  // Every filter but the map: the chips count over these rows, and the map chip then narrows them.
  const facet = filterRows(rows, { ...view.state.filters, map: FILTER_ANY }, {
    fields: {
      team: ({ match }) => match?.teams.map((team) => team.team_id ?? '') ?? [],
      rival: ({ match }) => match?.teams.map((team) => team.team_id ?? '') ?? [],
      event: (row) => rowEvent(row, seriesById),
      source: ({ demo }) => demo.source,
      state: ({ demo }) => demo.state,
    },
    searchText: ({ demo, match }) =>
      [match ? prettyMapName(match.map) : '', match ? matchTitle(match) : '', demo.file_name, ...demo.tags].join(' '),
  })
    .filter((row) => inDateRange(view.state.filters.date ?? FILTER_ANY, partidaDate(row), now))
    .filter((row) => listView.kind === null || rowKind(row, seriesById) === listView.kind)
    .filter((row) => !listView.pov || hasPov(row, povJobs))
    .filter((row) => !listView.voice || (row.match !== null && voices.has(row.match.id)));
  const shown = selectedMap === FILTER_ANY ? facet : facet.filter(({ match }) => match?.map === selectedMap);
  const chips = mapChips({ library: rows.map(({ match }) => match), listed: facet.map(({ match }) => match), selected: selectedMap });
  const pending = rows.filter(isPending).length;
  const handlers: RowHandlers = { onRetry: (demoId) => void retry(demoId), onRemoved: () => pollRef.current() };
  // Only rows the filters show count as chosen, so a bulk delete never reaches a hidden one.
  const chosenIds = shown.map((row) => row.demo.id).filter((demoId) => selected.has(demoId));
  const markScrimButton = (
    <Button type="button" variant="outline" size="xs" disabled={deleting} onClick={() => void changeSources(chosenIds, 'scrim').then((ok) => ok && setSelected(new Set()))}>
      <Swords aria-hidden />
      Marcar como scrim
    </Button>
  );
  const filterBar = (
    <FilterBar
      values={view.state.filters}
      onChange={view.setFilter}
      onReset={view.resetFilters}
      searchPlaceholder="Buscar por mapa, equipo, archivo o etiqueta"
      filters={[
        { id: 'team', label: 'Equipo', options: teamFilterOptions(rows.map(({ match }) => match)) },
        { id: 'rival', label: 'Rival', options: teamFilterOptions(rows.map(({ match }) => match)) },
        { id: 'event', label: 'Evento', options: eventOptions(loaded.series) },
        { id: 'date', label: 'Fecha', options: DATE_OPTIONS },
        { id: 'source', label: 'Fuente', options: SOURCE_OPTIONS },
        { id: 'state', label: 'Estado', options: STATE_OPTIONS },
      ]}
      count={{ shown: shown.length, total: rows.length, noun: rows.length === 1 ? 'partida' : 'partidas' }}
    />
  );

  const body = (
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
        <LibraryListControls view={listView} onChange={changeListView} />
        {listView.mode === 'series' ? (
          <div className="flex flex-col gap-4">
            <div className="flex flex-wrap items-start gap-2">
              {filterBar}
            </div>
            <PartidaGroupsList
              days={groupPartidas({ rows: shown, series: loaded.series, sort: listView.sort })}
              context={{
                ownTeam,
                libraryRows: rows,
                ownName: ownTeam === null ? '' : ownTeamName(rows, ownTeam),
                povJobs,
                voices,
                onVoiceResynced: () => pollRef.current(),
                allTags: libraryTags(rows),
                handlers,
                selected,
                onToggle: (demoId) => setSelected((current) => toggled(current, demoId)),
                onTagsChanged: changeTags,
                onSourceChange: (demoId, source) => void changeSources([demoId], source),
              }}
            />
            {chosenIds.length > 0 ? (
              <SelectionBar
                count={chosenIds.length}
                noun={PARTIDA}
                busy={deleting}
                confirmDescription={DELETE_DESCRIPTION}
                actions={markScrimButton}
                onDelete={() => void deleteChosen(chosenIds)}
                onClear={() => setSelected(new Set())}
              />
            ) : null}
          </div>
        ) : (
        <DataTable
          label="Partidas"
          rows={shown}
          selection={{
            selected,
            onChange: setSelected,
            noun: PARTIDA,
            busy: deleting,
            actions: (
              <>
                {markScrimButton}
                <DeleteChosenButton
                  count={chosenIds.length}
                  noun={PARTIDA}
                  loading={deleting}
                  description={DELETE_DESCRIPTION}
                  onConfirm={() => void deleteChosen(chosenIds)}
                />
              </>
            ),
          }}
          columns={columns(handlers, { byMatch: voices, onResynced: () => pollRef.current() })}
          rowKey={(row) => row.demo.id}
          rowHref={(row) => (row.match ? matchHref(row.match.id) : undefined)}
          view={view}
          toolbar={filterBar}
          empty={<p className="text-body-sm text-fg-2">Ninguna partida cumple estos filtros.</p>}
        />
        )}
      </div>
  );
  return frame(body);
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
    <PartidasFrame importer={<Skeleton className="h-8 w-36" />}>
      <LibrarySkeleton />
    </PartidasFrame>
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
