'use client';

import { useState, type ReactNode } from 'react';
import { useSearchParams } from 'next/navigation';
import { Copy, Crosshair, RefreshCw } from 'lucide-react';
import type { LibraryLineup } from '@/lib/api/lineups';
import { GRENADE_KIND_LABEL } from '@/lib/analysis/grenade-patterns';
import { writeClipboardText } from '@/lib/clipboard-write';
import { LIBRETA_QUERY } from '@/lib/clips/routes';
import { setposCommand, techniqueLabel } from '@/lib/libreta/lineups';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { EmptyState } from '@/components/records/empty-state';
import { useLibretaLineups } from './lineup-list';
import { PlanTable } from './plan-table';
import { useLibretaPlays } from './play-list';

type View = 'table' | 'lineups';
type Side = 'T' | 'CT';

const VIEWS = [
  { value: 'table', label: 'Tabla' },
  { value: 'lineups', label: 'Lineups' },
] as const;

const SIDES = [
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
] as const;

/** The Modo partida window: the Tabla and the lineups of one map, small enough for a second monitor. */
export function MatchMode(): ReactNode {
  const params = useSearchParams();
  const map = params.get(LIBRETA_QUERY.map) ?? '';
  const [view, setView] = useState<View>('table');
  const [side, setSide] = useState<Side>(params.get(LIBRETA_QUERY.side) === 'CT' ? 'CT' : 'T');
  const { plays, reload: reloadPlays } = useLibretaPlays();
  const { lineups, offline, reload: reloadLineups } = useLibretaLineups();

  return (
    <div className="flex flex-col gap-3 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <Segmented label="Vista" value={view} options={VIEWS} onChange={setView} />
        {view === 'lineups' ? <Segmented label="Lado" value={side} options={SIDES} onChange={setSide} /> : null}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="ml-auto"
          title="Volver a leer la Libreta"
          onClick={() => {
            reloadPlays();
            reloadLineups();
          }}
        >
          <RefreshCw aria-hidden />
          Actualizar
        </Button>
      </div>
      {offline ? <p className="text-meta text-fg-2">Studio no responde. Prueba Actualizar en un momento.</p> : null}
      {view === 'table' ? (
        <PlanTable
          plays={plays}
          onReload={reloadPlays}
          compact
          initialMap={map === '' ? undefined : map}
          initialSide={side}
          initialTeam={params.get(LIBRETA_QUERY.team) ?? undefined}
        />
      ) : (
        <LineupCards lineups={(lineups ?? []).filter((l) => l.map === map && l.side === side && l.status !== 'archived')} loading={lineups === null} />
      )}
    </div>
  );
}

function LineupCards({ lineups, loading }: { lineups: LibraryLineup[]; loading: boolean }): ReactNode {
  const [copied, setCopied] = useState<string | null>(null);
  if (loading) return <p className="text-meta text-fg-3">Leyendo la Libreta…</p>;
  if (lineups.length === 0) {
    return <EmptyState icon={Crosshair} title="Sin lineups de este mapa y lado" description="Guárdalos desde la pestaña Granadas de un equipo o con Nuevo lineup en la Libreta." />;
  }
  return (
    <ul className="flex flex-col gap-2">
      {lineups.map((lineup) => (
        <li key={lineup.id} className="flex items-start gap-2 rounded-md border border-border-subtle bg-surface-1 px-3 py-2">
          <div className="min-w-0 flex-1">
            <p className="truncate text-body-sm font-medium text-fg-1">{lineup.title}</p>
            <p className="text-meta text-fg-3">
              {GRENADE_KIND_LABEL[lineup.grenade]} · {techniqueLabel(lineup)}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            title="Copiar el setpos para la consola de CS2"
            onClick={() => {
              writeClipboardText(setposCommand(lineup))
                .then(() => setCopied(lineup.id))
                .catch(() => setCopied(null));
            }}
          >
            <Copy aria-hidden />
            {copied === lineup.id ? 'Copiado' : 'setpos'}
          </Button>
        </li>
      ))}
    </ul>
  );
}
