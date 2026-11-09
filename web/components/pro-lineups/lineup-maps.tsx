'use client';

import Link from 'next/link';
import { useState, type ReactNode } from 'react';
import { CloudOff, MapPinned, PlugZap } from 'lucide-react';
import { prettyMapName, timeAgo } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useProLineupMaps, type ProLoad } from './use-pro-lineups';

/** The states every pro lineup screen shares before its data is there. */
export function ProLoadProblem({ load, onRetry, map }: { load: ProLoad<unknown>; onRetry: () => void; map?: string }): ReactNode {
  const retry = (
    <Button type="button" variant="outline" size="sm" onClick={onRetry}>
      Reintentar
    </Button>
  );
  switch (load.kind) {
    case 'offline':
      return <EmptyState icon={PlugZap} title="Servicio local sin conexión" description="Comprueba que ClipHub Studio sigue abierto." actions={retry} />;
    case 'server_down':
      return (
        <EmptyState
          icon={CloudOff}
          title="No se pudo descargar la biblioteca de lineups"
          description="Los lineups pro vienen del servidor de ClipHub y hace falta conexión la primera vez. Luego Studio guarda una copia."
          actions={retry}
        />
      );
    case 'disabled':
      return (
        <EmptyState
          icon={CloudOff}
          title="El servidor de ClipHub está desactivado"
          description="Este Studio está configurado para no conectarse al servidor de ClipHub, de donde vienen los lineups pro."
        />
      );
    case 'missing':
      return (
        <EmptyState
          icon={MapPinned}
          title="Aún no hay lineups pro de este mapa"
          description={`Se añaden cuando hay suficientes partidas pro de ${map === undefined ? 'este mapa' : prettyMapName(map)}.`}
          actions={
            <Button asChild variant="outline" size="sm">
              <Link href="/lineups">Ver otros mapas</Link>
            </Button>
          }
        />
      );
    case 'failed':
      return <EmptyState icon={PlugZap} title="No se pudieron leer los lineups" description="Vuelve a intentarlo en unos segundos." actions={retry} />;
    default:
      return null;
  }
}

const MAP_GRID = 'grid grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] gap-3';

function MapsSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando mapas" className={MAP_GRID}>
      {Array.from({ length: 7 }, (_, index) => (
        <div key={index} className="flex flex-col overflow-hidden rounded-lg border border-border bg-surface-1">
          <Skeleton className="aspect-[4/3] w-full rounded-none" />
          <div className="flex flex-col gap-1.5 px-3 py-2.5">
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-4 w-36" />
          </div>
        </div>
      ))}
    </div>
  );
}

/** Every map with pro lineups, as cards with CS2's radar that open the map. */
export function LineupMaps(): ReactNode {
  const [attempt, setAttempt] = useState(0);
  const load = useProLineupMaps(attempt);
  if (load.kind === 'loading') return <MapsSkeleton />;
  if (load.kind !== 'ready') return <ProLoadProblem load={load} onRetry={() => setAttempt((n) => n + 1)} />;
  const { maps, built_at: builtAt, fresh } = load.data;
  return (
    <div className="flex flex-col gap-2">
      <ul className={MAP_GRID}>
        {maps.map((entry) => (
          <li key={entry.map}>
            <Link
              href={`/lineups/${entry.map}`}
              className="group flex flex-col overflow-hidden rounded-lg border border-border bg-surface-1 transition-[border-color,background-color] hover:border-border-strong hover:bg-surface-2 focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
            >
              <MapThumb map={entry.map} className="aspect-[4/3] h-auto w-full rounded-none" crop={1.15} />
              <span className="flex flex-col gap-0.5 px-3 py-2.5">
                <span className="text-body font-semibold text-fg-1">{prettyMapName(entry.map)}</span>
                <span className="text-meta text-fg-2 tabular-nums">
                  {entry.lineups} lineups · {entry.combos} {entry.combos === 1 ? 'combo' : 'combos'}
                </span>
                <span className="text-meta text-fg-3 tabular-nums">De {entry.recent_maps} mapas pro</span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
      <p className="text-meta text-fg-3">
        Actualizado {timeAgo(builtAt)}
        {fresh ? null : '. Sin conexión con el servidor: es la última copia que guardó Studio.'}
      </p>
    </div>
  );
}
