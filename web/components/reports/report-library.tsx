'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { FileText, Loader2, Plus, PlugZap, Shield } from 'lucide-react';
import { createLibraryReport, deleteLibraryReport, isLibraryNotFound, isLibraryOffline, listLibraryMatches, listLibraryReports } from '@/lib/api/library';
import type { LibraryReport } from '@/lib/api/scout';
import { REPORTS_QUERY, TEAMS_HREF, reportHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { REPORT_SIDE_LABEL } from '@/lib/reports/scout-sections';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { DocumentPage } from '@/components/records/document-page';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { useTableView, type TableViewControl } from '@/components/records/use-table-view';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { Button } from '@/components/ui/button';
import { NewReportDialog, type ReportDraft } from './new-report-dialog';

/** A report being written: the row stays on the list, reading "Generando", until the server answers. */
type PendingReport = { id: string; teamName: string };
type ReportRow = { kind: 'report'; report: LibraryReport } | { kind: 'pending'; pending: PendingReport };

function rowKey(row: ReportRow): string {
  return row.kind === 'report' ? row.report.id : row.pending.id;
}

function mapsText(report: LibraryReport, mapOfMatch: ReadonlyMap<string, string>): string {
  const maps = [...new Set(report.match_ids.flatMap((id) => mapOfMatch.get(id) ?? []))];
  return maps.map(prettyMapName).join(', ');
}

function buildColumns(mapOfMatch: ReadonlyMap<string, string>): readonly DataTableColumn<ReportRow>[] {
  return [
    {
      id: 'team',
      header: 'Equipo',
      required: true,
      cell: (row) =>
        row.kind === 'pending' ? (
          <span className="font-medium">{row.pending.teamName}</span>
        ) : (
          <span className="flex min-w-0 flex-col">
            <Link href={teamHref(row.report.team_id)} className="font-medium text-fg-1 hover:text-primary hover:underline">
              {row.report.team_name}
            </Link>
            <span className="truncate text-meta text-fg-3">{row.report.title}</span>
          </span>
        ),
      sortValue: (row) => (row.kind === 'report' ? row.report.team_name : row.pending.teamName),
    },
    {
      id: 'maps',
      header: 'Mapas',
      cell: (row) => (row.kind === 'report' ? <span className="text-fg-2">{mapsText(row.report, mapOfMatch)}</span> : null),
    },
    {
      id: 'matches',
      header: 'Partidas',
      align: 'end',
      cell: (row) => (row.kind === 'report' ? row.report.match_ids.length : null),
      sortValue: (row) => (row.kind === 'report' ? row.report.match_ids.length : null),
    },
    {
      id: 'side',
      header: 'Lado',
      cell: (row) => (row.kind === 'report' ? REPORT_SIDE_LABEL[row.report.side] : null),
      sortValue: (row) => (row.kind === 'report' ? row.report.side : null),
    },
    {
      id: 'updated',
      header: 'Generado',
      cell: (row) =>
        row.kind === 'report' ? (
          <span className="text-fg-2">{formatShortDate(row.report.updated_at)}</span>
        ) : (
          <span role="status" className="inline-flex items-center gap-1.5 text-fg-2">
            <Loader2 aria-hidden className="size-3.5 animate-spin motion-reduce:animate-none" />
            Generando...
          </span>
        ),
      sortValue: (row) => (row.kind === 'report' ? Date.parse(row.report.updated_at) || null : Number.MAX_SAFE_INTEGER),
    },
    {
      id: 'open',
      header: 'Ver',
      required: true,
      cell: (row) =>
        row.kind === 'report' ? (
          <Button asChild variant="success" size="xs">
            <Link href={reportHref(row.report.id)}>Abrir</Link>
          </Button>
        ) : null,
    },
  ];
}

/** A report already gone counts as deleted: the row only has to leave the list. */
async function deleteReport(reportId: string): Promise<void> {
  try {
    await deleteLibraryReport(reportId);
  } catch (error) {
    if (!isLibraryNotFound(error)) throw error;
  }
}

function deleteColumn(onDeleted: () => void): DataTableColumn<ReportRow> {
  return {
    id: 'delete',
    header: '',
    required: true,
    className: 'w-px',
    cell: (row) =>
      row.kind === 'report' ? (
        <DeleteMatchButton label={row.report.title} onConfirm={() => deleteReport(row.report.id)} onDeleted={onDeleted} />
      ) : null,
  };
}

/** Informes: the saved scouting reports as the Match Prep list, and the door to a new one. */
export function ReportLibrary(): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const requestedTeam = search.get(REPORTS_QUERY.team) ?? undefined;
  const [reports, setReports] = useState<LibraryReport[] | null>(null);
  const [mapOfMatch, setMapOfMatch] = useState<ReadonlyMap<string, string>>(new Map());
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [creating, setCreating] = useState(requestedTeam !== undefined);
  const [pending, setPending] = useState<PendingReport[]>([]);
  const [failure, setFailure] = useState<string | null>(null);
  const view = useTableView('informes', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: ['matches', 'side'],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });

  useEffect(() => {
    if (requestedTeam !== undefined) setCreating(true);
  }, [requestedTeam]);

  useEffect(() => {
    let active = true;
    listLibraryReports()
      .then((list) => {
        if (!active) return;
        setReports(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setReports([]);
        setOffline(isLibraryOffline(error));
      });
    listLibraryMatches()
      .then((matches) => {
        if (active) setMapOfMatch(new Map(matches.map((match) => [match.id, match.map])));
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  function setDialog(open: boolean): void {
    setCreating(open);
    // The team in the URL only opens the dialog once; closing it clears the link.
    if (!open && requestedTeam !== undefined) router.replace(pathname);
  }

  function generate(draft: ReportDraft): void {
    const row: PendingReport = { id: `pending-${crypto.randomUUID()}`, teamName: draft.team_name };
    setFailure(null);
    setPending((rows) => [...rows, row]);
    createLibraryReport({ title: draft.title, team_id: draft.team_id, match_ids: draft.match_ids, side: draft.side })
      .catch(() => setFailure('No se pudo crear el informe. Comprueba que las partidas siguen en la biblioteca.'))
      .finally(() => {
        setPending((rows) => rows.filter((entry) => entry.id !== row.id));
        reload();
      });
  }

  const newButton = (
    <span className="flex flex-wrap items-center gap-2">
      <Button asChild variant="outline" size="sm">
        <Link href={TEAMS_HREF} title="La preparación de un rival está en su ficha de Equipos, pestaña Preparar">
          Preparar rival
        </Link>
      </Button>
      <Button type="button" variant="success" size="icon-sm" aria-label="Nuevo informe" title="Nuevo informe" onClick={() => setCreating(true)}>
        <Plus aria-hidden />
      </Button>
    </span>
  );

  // The dialog stays at one place in the tree, so loading the list never remounts it mid-animation.
  return (
    <DocumentPage title="Informes" description="Estos son los informes que has generado. Se calculan en este PC a partir de las partidas que elijas." actions={newButton}>
      {failure ? (
        <p role="alert" className="text-body-sm text-destructive">
          {failure}
        </p>
      ) : null}
      <ReportListBody reports={reports} pending={pending} offline={offline} view={view} mapOfMatch={mapOfMatch} onNew={() => setCreating(true)} onReload={reload} />
      <NewReportDialog open={creating} onOpenChange={setDialog} initialTeamId={requestedTeam} onGenerate={generate} />
    </DocumentPage>
  );
}

function ReportListBody({
  reports,
  pending,
  offline,
  view,
  mapOfMatch,
  onNew,
  onReload,
}: {
  reports: LibraryReport[] | null;
  pending: readonly PendingReport[];
  offline: boolean;
  view: TableViewControl;
  mapOfMatch: ReadonlyMap<string, string>;
  onNew: () => void;
  onReload: () => void;
}): ReactNode {
  if (reports === null) {
    return <TableSkeleton label="Cargando informes" columns={4} />;
  }

  if (reports.length === 0 && pending.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer los informes. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={onReload}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={FileText}
        title="Aún no hay informes"
        description="Un informe reúne lo que hace un rival en las partidas que elijas y se exporta para el coach y el IGL."
        preview={[
          'Pistolas, defaults de T, setups de CT y ejecuciones por sitio con sus granadas.',
          'Economía tras perder, posiciones del AWP y aperturas por jugador.',
          'Cada cifra enlaza a su partida, su ronda y su tick.',
        ]}
        actions={
          <>
            <Button type="button" variant="success" size="sm" onClick={onNew}>
              <Plus aria-hidden />
              Nuevo informe
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link href={TEAMS_HREF}>
                <Shield aria-hidden />
                Ver equipos
              </Link>
            </Button>
          </>
        }
      />
    );
  }

  const shown = filterRows(reports, view.state.filters, {
    fields: {},
    searchText: (report) => `${report.title} ${report.team_name}`,
  });
  const rows: ReportRow[] = [
    ...pending.map((entry): ReportRow => ({ kind: 'pending', pending: entry })),
    ...shown.map((report): ReportRow => ({ kind: 'report', report })),
  ];

  return (
    <DataTable
      toolbar={
        <FilterBar
          values={view.state.filters}
          onChange={view.setFilter}
          onReset={view.resetFilters}
          searchPlaceholder="Buscar por título o rival"
          count={{ shown: shown.length, total: reports.length, noun: reports.length === 1 ? 'informe' : 'informes' }}
        />
      }
      label="Informes"
      rows={rows}
      columns={[...buildColumns(mapOfMatch), deleteColumn(onReload)]}
      rowKey={rowKey}
      view={view}
      empty={<p className="text-body-sm text-fg-2">Ningún informe cumple esta búsqueda.</p>}
    />
  );
}
