'use client';

import { useEffect, useRef, useState, type ChangeEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { BookMarked, FileDown, FileUp, PlugZap, Swords } from 'lucide-react';
import { exportStratbook, importStratbook, isLibraryOffline, listLibraryPlays, type PlayListQuery } from '@/lib/api/library';
import type { LibraryPlay } from '@/lib/api/stratbook';
import { MATCHES_HREF, STRATBOOK_DETECT_TAB, STRATBOOK_QUERY, playHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { PLAY_ORIGIN_LABEL, parseStratbookFile, playKindLabel, playShapeLabel, stratbookFileName } from '@/lib/stratbook/plays';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { RecordTabs } from '@/components/records/record-tabs';
import { useTableView, type TableViewControl } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PlayDetection } from './play-detection';

const NO_TEAM = 'none';

const COLUMNS: readonly DataTableColumn<LibraryPlay>[] = [
  { id: 'title', header: 'Jugada', required: true, cell: (play) => play.title, sortValue: (play) => play.title },
  {
    id: 'team',
    header: 'Equipo',
    cell: (play) =>
      play.team_id ? (
        <Link href={teamHref(play.team_id)} className="hover:text-primary hover:underline">
          {play.team_name}
        </Link>
      ) : (
        <span className="text-fg-3">Sin equipo</span>
      ),
    sortValue: (play) => play.team_name ?? '',
  },
  { id: 'map', header: 'Mapa', cell: (play) => prettyMapName(play.map), sortValue: (play) => play.map },
  { id: 'side', header: 'Lado', cell: (play) => play.side, sortValue: (play) => play.side },
  { id: 'kind', header: 'Tipo', cell: (play) => playShapeLabel(play), sortValue: (play) => play.kind },
  { id: 'grenades', header: 'Granadas', align: 'end', cell: (play) => play.grenades.length, sortValue: (play) => play.grenades.length },
  { id: 'examples', header: 'Rondas', align: 'end', cell: (play) => play.examples.length, sortValue: (play) => play.examples.length },
  { id: 'origin', header: 'Origen', cell: (play) => PLAY_ORIGIN_LABEL[play.origin], sortValue: (play) => play.origin },
  {
    id: 'updated',
    header: 'Actualizada',
    cell: (play) => <span className="text-fg-2">{formatShortDate(play.updated_at)}</span>,
    sortValue: (play) => Date.parse(play.updated_at) || null,
  },
];

function options(values: Iterable<[string, string]>): { value: string; label: string }[] {
  return [...new Map(values)].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'));
}

/** Downloads a JSON document as a file. */
function downloadJson(value: unknown, fileName: string): void {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json;charset=utf-8' }));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = fileName;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** Stratbook: the saved plays, the detection of repeated plays, and the file export and import. */
export function StratbookLibrary(): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const requestedTeam = search.get(STRATBOOK_QUERY.team) ?? undefined;
  const [tab, setTab] = useState(search.get(STRATBOOK_QUERY.tab) === STRATBOOK_DETECT_TAB ? STRATBOOK_DETECT_TAB : 'jugadas');
  const [plays, setPlays] = useState<LibraryPlay[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const view = useTableView('stratbook', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const { setFilter } = view;

  useEffect(() => {
    let active = true;
    listLibraryPlays()
      .then((list) => {
        if (!active) return;
        setPlays(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setPlays([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  // ?equipo= opens the list on that team's plays once; the filter is not stable, so a ref guards it.
  const appliedTeam = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (requestedTeam === undefined || appliedTeam.current === requestedTeam) return;
    appliedTeam.current = requestedTeam;
    if (tab !== STRATBOOK_DETECT_TAB) setFilter('team', requestedTeam);
  }, [requestedTeam, tab, setFilter]);

  function changeTab(next: string): void {
    setTab(next);
    if (search.size > 0) router.replace(pathname);
  }

  async function exportFile(): Promise<void> {
    const filters = view.state.filters;
    const query: PlayListQuery = {};
    const team = filters.team ?? FILTER_ANY;
    if (team !== FILTER_ANY && team !== NO_TEAM) query.team_id = team;
    if ((filters.map ?? FILTER_ANY) !== FILTER_ANY) query.map = filters.map;
    const side = filters.side ?? FILTER_ANY;
    if (side === 'CT' || side === 'T') query.side = side;
    setBusy(true);
    setMessage(null);
    try {
      const file = await exportStratbook(query);
      const teamName = query.team_id ? plays?.find((play) => play.team_id === query.team_id)?.team_name : undefined;
      downloadJson(file, stratbookFileName(new Date(), teamName));
      setMessage(file.plays.length === 1 ? 'Exportada 1 jugada.' : `Exportadas ${file.plays.length} jugadas.`);
    } catch {
      setMessage('No se pudo exportar el stratbook.');
    } finally {
      setBusy(false);
    }
  }

  async function importFile(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (file === undefined) return;
    setMessage(null);
    const parsed = parseStratbookFile(await file.text());
    if (!parsed.ok) {
      setMessage(parsed.error);
      return;
    }
    setBusy(true);
    try {
      const result = await importStratbook(parsed.file);
      const parts = [result.imported === 1 ? 'Importada 1 jugada.' : `Importadas ${result.imported} jugadas.`];
      if (result.duplicates > 0) parts.push(result.duplicates === 1 ? '1 ya estaba en el stratbook.' : `${result.duplicates} ya estaban en el stratbook.`);
      if (result.rejected.length > 0) {
        parts.push(`${result.rejected.length} no se pudieron leer: ${result.rejected.map((entry) => entry.title || `n.º ${entry.index + 1}`).join(', ')}.`);
      }
      setMessage(parts.join(' '));
      setAttempt((n) => n + 1);
    } catch {
      setMessage('No se pudo importar el fichero. Comprueba que es un stratbook exportado por Studio.');
    } finally {
      setBusy(false);
    }
  }

  const fileActions = (
    <>
      <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => fileRef.current?.click()}>
        <FileUp aria-hidden />
        Importar
      </Button>
      <Button type="button" variant="outline" size="sm" disabled={busy || (plays?.length ?? 0) === 0} onClick={() => void exportFile()}>
        <FileDown aria-hidden />
        Exportar
      </Button>
      <input
        ref={fileRef}
        type="file"
        accept=".json,application/json"
        aria-label="Fichero de stratbook"
        className="sr-only"
        tabIndex={-1}
        onChange={(event) => void importFile(event)}
      />
    </>
  );

  return (
    <div className="flex flex-col gap-3">
      {message ? (
        <p role="status" className="text-body-sm text-fg-2">
          {message}
        </p>
      ) : null}
      <RecordTabs
        value={tab}
        onValueChange={changeTab}
        tabs={[
          {
            id: 'jugadas',
            label: 'Jugadas',
            count: plays?.length,
            content: (
              <PlayList
                plays={plays}
                offline={offline}
                view={view}
                fileActions={fileActions}
                onDetect={() => changeTab(STRATBOOK_DETECT_TAB)}
                onRetry={() => setAttempt((n) => n + 1)}
              />
            ),
          },
          {
            id: STRATBOOK_DETECT_TAB,
            label: 'Detectar repetidas',
            content: <PlayDetection initialTeamId={requestedTeam} onSaved={() => setAttempt((n) => n + 1)} />,
          },
        ]}
      />
    </div>
  );
}

function PlayList({
  plays,
  offline,
  view,
  fileActions,
  onDetect,
  onRetry,
}: {
  plays: LibraryPlay[] | null;
  offline: boolean;
  view: TableViewControl;
  fileActions: ReactNode;
  onDetect: () => void;
  onRetry: () => void;
}): ReactNode {
  if (plays === null) {
    return (
      <div role="status" aria-label="Cargando jugadas" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (plays.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer el stratbook. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={BookMarked}
        title="Tu stratbook está vacío"
        description="Guarda una ronda desde la repetición 2D de una partida, deja que Studio encuentre las jugadas que un equipo repite o importa un stratbook."
        preview={[
          'Mapa, lado y tipo de jugada, con sus granadas y posiciones.',
          'Notas y rondas de ejemplo que abren la repetición 2D en su tick.',
          'Jugadas repetidas detectadas al agrupar rondas parecidas.',
        ]}
        actions={
          <>
            <Button type="button" size="sm" onClick={onDetect}>
              Detectar jugadas repetidas
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link href={MATCHES_HREF}>
                <Swords aria-hidden />
                Ver partidas
              </Link>
            </Button>
            {fileActions}
          </>
        }
      />
    );
  }

  const shown = filterRows(plays, view.state.filters, {
    fields: {
      team: (play) => play.team_id ?? NO_TEAM,
      map: (play) => play.map,
      side: (play) => play.side,
      kind: (play) => play.kind,
      origin: (play) => play.origin,
    },
    searchText: (play) => `${play.title} ${play.team_name ?? ''} ${play.notes} ${prettyMapName(play.map)}`,
  });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterBar
          values={view.state.filters}
          onChange={view.setFilter}
          onReset={view.resetFilters}
          searchPlaceholder="Buscar por título, equipo o notas"
          filters={[
            {
              id: 'team',
              label: 'Equipo',
              options: options(plays.map((play) => [play.team_id ?? NO_TEAM, play.team_name ?? 'Sin equipo'])),
            },
            { id: 'map', label: 'Mapa', options: options(plays.map((play) => [play.map, prettyMapName(play.map)])) },
            { id: 'side', label: 'Lado', options: [{ value: 'T', label: 'T' }, { value: 'CT', label: 'CT' }] },
            { id: 'kind', label: 'Tipo', options: options(plays.map((play) => [play.kind, playKindLabel(play.kind)])) },
            {
              id: 'origin',
              label: 'Origen',
              options: (['round', 'detected', 'import'] as const).map((value) => ({ value, label: PLAY_ORIGIN_LABEL[value] })),
            },
          ]}
          count={{ shown: shown.length, total: plays.length, noun: plays.length === 1 ? 'jugada' : 'jugadas' }}
          className="flex flex-1 flex-wrap items-center gap-2"
        />
        <div className="flex flex-wrap gap-2">{fileActions}</div>
      </div>
      <DataTable
        label="Jugadas"
        rows={shown}
        columns={COLUMNS}
        rowKey={(play) => play.id}
        rowHref={(play) => playHref(play.id)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ninguna jugada cumple estos filtros.</p>}
      />
    </div>
  );
}
