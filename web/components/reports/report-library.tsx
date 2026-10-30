'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { FilePlus2, FileText, PlugZap, Shield } from 'lucide-react';
import { isLibraryOffline, listLibraryReports } from '@/lib/api/library';
import type { LibraryReport } from '@/lib/api/scout';
import { REPORTS_QUERY, TEAMS_HREF, reportHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { REPORT_SIDE_LABEL } from '@/lib/reports/scout-sections';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView, type TableViewControl } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { NewReportDialog } from './new-report-dialog';

const COLUMNS: readonly DataTableColumn<LibraryReport>[] = [
  { id: 'title', header: 'Informe', required: true, cell: (report) => report.title, sortValue: (report) => report.title },
  {
    id: 'team',
    header: 'Rival',
    cell: (report) => (
      <Link href={teamHref(report.team_id)} className="hover:text-primary hover:underline">
        {report.team_name}
      </Link>
    ),
    sortValue: (report) => report.team_name,
  },
  { id: 'matches', header: 'Partidas', align: 'end', cell: (report) => report.match_ids.length, sortValue: (report) => report.match_ids.length },
  { id: 'side', header: 'Lado', cell: (report) => REPORT_SIDE_LABEL[report.side], sortValue: (report) => report.side },
  {
    id: 'updated',
    header: 'Actualizado',
    cell: (report) => <span className="text-fg-2">{formatShortDate(report.updated_at)}</span>,
    sortValue: (report) => Date.parse(report.updated_at) || null,
  },
];

/** Informes: the saved scouting reports, and the door to a new one. */
export function ReportLibrary(): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const requestedTeam = search.get(REPORTS_QUERY.team) ?? undefined;
  const [reports, setReports] = useState<LibraryReport[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [creating, setCreating] = useState(requestedTeam !== undefined);
  const view = useTableView('informes', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: [],
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
    return () => {
      active = false;
    };
  }, [attempt]);

  function setDialog(open: boolean): void {
    setCreating(open);
    // The team in the URL only opens the dialog once; closing it clears the link.
    if (!open && requestedTeam !== undefined) router.replace(pathname);
  }

  const dialog = <NewReportDialog open={creating} onOpenChange={setDialog} initialTeamId={requestedTeam} />;
  const newButton = (
    <Button type="button" size="sm" onClick={() => setCreating(true)}>
      <FilePlus2 aria-hidden />
      Nuevo informe
    </Button>
  );

  // The dialog stays at one place in the tree, so loading the list never remounts it mid-animation.
  return (
    <>
      <ReportListBody reports={reports} offline={offline} view={view} newButton={newButton} onRetry={() => setAttempt((n) => n + 1)} />
      {dialog}
    </>
  );
}

function ReportListBody({
  reports,
  offline,
  view,
  newButton,
  onRetry,
}: {
  reports: LibraryReport[] | null;
  offline: boolean;
  view: TableViewControl;
  newButton: ReactNode;
  onRetry: () => void;
}): ReactNode {
  if (reports === null) {
    return (
      <div role="status" aria-label="Cargando informes" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (reports.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer los informes. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
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
            {newButton}
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

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterBar
          values={view.state.filters}
          onChange={view.setFilter}
          onReset={view.resetFilters}
          searchPlaceholder="Buscar por título o rival"
          count={{ shown: shown.length, total: reports.length, noun: reports.length === 1 ? 'informe' : 'informes' }}
          className="flex flex-1 flex-wrap items-center gap-2"
        />
        {newButton}
      </div>
      <DataTable
        label="Informes"
        rows={shown}
        columns={COLUMNS}
        rowKey={(report) => report.id}
        rowHref={(report) => reportHref(report.id)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ningún informe cumple esta búsqueda.</p>}
      />
    </div>
  );
}
