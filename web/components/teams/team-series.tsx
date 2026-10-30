'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Plus } from 'lucide-react';
import { listLibrarySeries, type LibraryTeam } from '@/lib/api/library';
import type { LibrarySeries } from '@/lib/api/veto';
import { librarySeriesHref } from '@/lib/clips/routes';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import { DataTable } from '@/components/records/data-table';
import { useTableView } from '@/components/records/use-table-view';
import { NewSeriesDialog } from '@/components/series/new-series-dialog';
import { SERIES_COLUMNS } from '@/components/series/series-library';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

/** The series a team holds a slot in, the source of its picks and bans. */
export function TeamSeries({ team }: { team: LibraryTeam }): ReactNode {
  const [series, setSeries] = useState<LibrarySeries[] | null | 'failed'>(null);
  const [creating, setCreating] = useState(false);
  const view = useTableView('equipo-series', { sort: { column: 'created', direction: 'desc' }, hidden: [], filters: { [SEARCH_FILTER]: FILTER_ANY } });

  useEffect(() => {
    let active = true;
    listLibrarySeries({ team_id: team.id })
      .then((list) => {
        if (active) setSeries(list);
      })
      .catch(() => {
        if (active) setSeries('failed');
      });
    return () => {
      active = false;
    };
  }, [team.id]);

  if (series === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron leer las series. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (series === null) return <Skeleton className="h-48 w-full" />;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-body-sm text-fg-2">Las series con su veto alimentan los picks y bans del historial de mapas y del simulador.</p>
        <Button type="button" size="sm" onClick={() => setCreating(true)}>
          <Plus aria-hidden />
          Nueva serie
        </Button>
      </div>
      <DataTable
        label={`Series de ${team.name}`}
        rows={series}
        columns={SERIES_COLUMNS}
        rowKey={(s) => s.id}
        rowHref={(s) => librarySeriesHref(s.id)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Este equipo aún no tiene series apuntadas.</p>}
      />
      {creating ? <NewSeriesDialog teamId={team.id} onClose={() => setCreating(false)} /> : null}
    </div>
  );
}
