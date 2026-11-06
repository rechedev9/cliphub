'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getGlobalRankings } from '@/lib/api/library';
import type { GlobalMetric, GlobalRankings } from '@/lib/api/community';
import { METRIC_INFO } from '@/lib/analysis/reference';
import { prettyMapName } from '@/lib/format';
import { Skeleton } from '@/components/ui/skeleton';

const SETTINGS_HREF = '/settings#community';
const percent = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 0 });

/** The own player's percentile on each metric among every Studio that shares its rankings. */
export function GlobalRanking({ map }: { map: string }): ReactNode {
  const [data, setData] = useState<GlobalRankings | null | 'failed'>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setData(null);
    getGlobalRankings(map)
      .then((loaded) => {
        if (active) setData(loaded);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [map, attempt]);

  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el ranking global: el servidor de ClipHub no responde.{' '}
        <button type="button" onClick={() => setAttempt((n) => n + 1)} className="underline underline-offset-4">
          Vuelve a intentarlo
        </button>
        .
      </p>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Leyendo el ranking global" className="studio-panel flex flex-col gap-1.5 p-3">
        {Array.from({ length: 10 }, (_, index) => (
          <Skeleton key={index} className="h-7 w-full" />
        ))}
      </div>
    );
  }
  if (!data.rankings_enabled) {
    return (
      <p className="text-body-sm text-fg-2">
        El ranking global está desactivado. Actívalo en{' '}
        <Link href={SETTINGS_HREF} className="text-primary underline underline-offset-4">
          Ajustes
        </Link>{' '}
        para comparar tus cifras con las de otros usuarios de Studio.
      </p>
    );
  }
  if (!data.steam_id_set) {
    return (
      <p className="text-body-sm text-fg-2">
        Indica tu SteamID en{' '}
        <Link href="/settings" className="text-primary underline underline-offset-4">
          Ajustes
        </Link>{' '}
        para que Studio sepa qué jugador de tus demos eres.
      </p>
    );
  }
  const where = map === '' ? 'en todos los mapas' : `en ${prettyMapName(map)}`;
  if (data.rounds < data.min_rounds) {
    return (
      <p className="text-body-sm text-fg-2">
        Tienes {data.rounds} rondas {where}. El ranking global necesita {data.min_rounds} o más: importa más demos tuyas o elige otro mapa.
      </p>
    );
  }
  const installs = Math.max(0, ...data.metrics.map((row) => row.installs));
  const ready = data.metrics.some((row) => row.percentile !== null);
  return (
    <section aria-label="Ranking global" className="studio-panel flex flex-col">
      {ready ? null : (
        <p className="border-b border-border-subtle px-3 py-2 text-body-sm text-fg-2">
          Aún hay {installs} {installs === 1 ? 'Studio' : 'Studios'} con datos {where}. Los percentiles aparecen cuando lo comparten {data.min_installs} o más.
        </p>
      )}
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-left text-body-sm">
          <thead className="bg-surface-2 text-meta text-fg-3">
            <tr className="border-b border-border">
              <th scope="col" className="h-8 px-2.5 font-medium">Cifra</th>
              <th scope="col" className="h-8 px-2.5 text-right font-medium">Tú</th>
              <th scope="col" className="h-8 px-2.5 text-right font-medium">Mediana</th>
              <th scope="col" className="h-8 w-[35%] px-2.5 font-medium">Percentil</th>
              <th scope="col" className="h-8 px-2.5 text-right font-medium">Studios</th>
            </tr>
          </thead>
          <tbody>
            {data.metrics.map((row) => (
              <GlobalRow key={row.metric} row={row} />
            ))}
          </tbody>
        </table>
      </div>
      <p className="border-t border-border-subtle px-3 py-1.5 text-meta text-fg-3">
        Tus {data.rounds} rondas en {data.matches} {data.matches === 1 ? 'partida' : 'partidas'} {where}, frente a cada Studio que comparte sus cifras con {data.min_rounds} rondas o más. El percentil dice a cuántos superas; en muertes, morir menos es mejor.
      </p>
    </section>
  );
}

function GlobalRow({ row }: { row: GlobalMetric }): ReactNode {
  const info = METRIC_INFO[row.metric];
  return (
    <tr className="border-b border-border-subtle last:border-b-0">
      <th scope="row" className="px-2.5 py-1.5 text-left font-medium text-fg-1">{info.label}</th>
      <td className="px-2.5 py-1.5 text-right text-fg-1 tabular-nums">{row.value === null ? 'Sin dato' : info.format(row.value)}</td>
      <td className="px-2.5 py-1.5 text-right text-fg-2 tabular-nums">{row.median === null ? '' : info.format(row.median)}</td>
      <td className="px-2.5 py-1.5">
        {row.percentile === null ? (
          <span className="text-meta text-fg-3">Faltan Studios</span>
        ) : (
          <span className="flex items-center gap-2">
            <span aria-hidden className="block h-1.5 flex-1 rounded-sm bg-surface-3">
              <span className="block h-full rounded-sm bg-primary/60" style={{ width: `${row.percentile}%` }} />
            </span>
            <span className="w-24 text-right text-fg-1 tabular-nums">Superas al {percent.format(row.percentile)} %</span>
          </span>
        )}
      </td>
      <td className="px-2.5 py-1.5 text-right text-fg-2 tabular-nums">{row.installs}</td>
    </tr>
  );
}
