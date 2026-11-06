'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getLibraryPlayerRole, type LibraryRosterEntry } from '@/lib/api/library';
import type { Baseline, BaselineMetric, BaselineScope } from '@/lib/api/baseline';
import { METRIC_INFO, baselineSampleText, baselineValue, minRoundsNote, proSourceText, referenceLabel } from '@/lib/analysis/reference';
import { playerHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { DeltaValue } from '@/components/ui/delta-value';
import { Skeleton } from '@/components/ui/skeleton';
import { useReferences } from '@/hooks/use-references';

/** The numbers each roster row shows, compared with the reference. */
const ROW_METRICS: readonly BaselineMetric[] = ['kpr', 'dpr', 'adr', 'kast', 'opening_win_pct', 'trade_pct'];
const SHORT_LABEL: Partial<Record<BaselineMetric, string>> = {
  kpr: 'Bajas/r',
  dpr: 'Muertes/r',
  opening_win_pct: 'Aperturas',
  trade_pct: 'Tradeadas',
};
/** The roster as one dense row per player, most recent first: role and key numbers against the chosen reference. */
export function TeamRoster({
  roster,
  map,
  reference,
}: {
  roster: readonly LibraryRosterEntry[];
  /** The scope's map; numbers and reference read the same map. */
  map: string;
  reference: BaselineScope;
}): ReactNode {
  const data = useReferences(map, ROW_METRICS);
  const [roles, setRoles] = useState<ReadonlyMap<string, string>>(new Map());
  const ids = roster.map((entry) => entry.steamid64).join(',');

  useEffect(() => {
    let active = true;
    for (const steamId of ids === '' ? [] : ids.split(',')) {
      getLibraryPlayerRole(steamId)
        .then((role) => {
          if (active) setRoles((current) => new Map(current).set(steamId, ROLE_LABEL[role.role]));
        })
        .catch(() => undefined);
    }
    return () => {
      active = false;
    };
  }, [ids]);

  if (roster.length === 0) return <p className="text-body-sm text-fg-2">Sin jugadores en las partidas de este equipo.</p>;
  const sorted = [...roster].sort((a, b) => Date.parse(b.last_played_at) - Date.parse(a.last_played_at));
  const ready = data !== null && data !== 'failed' ? data : null;
  const scope = reference;
  let baseline: Baseline | null = null;
  if (ready) baseline = scope === 'pro' ? ready.pro : ready.library;
  const label = referenceLabel(scope);

  return (
    <section aria-label="Plantilla" className="flex flex-col gap-2">
      {data === 'failed' ? (
        <p role="alert" className="text-body-sm text-destructive">
          No se pudieron leer las cifras de los jugadores. Comprueba que ClipHub Studio sigue abierto.
        </p>
      ) : null}
      <div className="studio-panel overflow-x-auto">
        <table aria-label="Plantilla" className="w-full border-collapse text-left text-body-sm">
          <thead className="bg-surface-2 text-meta text-fg-3">
            <tr className="border-b border-border">
              <th scope="col" className="sticky left-0 z-10 h-8 bg-surface-2 px-3 font-medium">Jugador</th>
              <th scope="col" className="h-8 px-3 font-medium">Rol</th>
              <th scope="col" className="h-8 px-3 text-right font-medium">Partidas</th>
              {ROW_METRICS.map((metric) => (
                <th key={metric} scope="col" className="h-8 px-3 text-right font-medium whitespace-nowrap" title={METRIC_INFO[metric].label}>
                  {SHORT_LABEL[metric] ?? METRIC_INFO[metric].label}
                </th>
              ))}
              <th scope="col" className="h-8 px-3 text-right font-medium">Última</th>
            </tr>
            <tr className="border-b border-border text-fg-3">
              <th scope="row" colSpan={3} className="sticky left-0 z-10 bg-surface-2 px-3 py-1 text-left font-normal">
                {label}
              </th>
              {ROW_METRICS.map((metric) => {
                const value = baselineValue(baseline, metric);
                return (
                  <td key={metric} className="px-3 py-1 text-right tabular-nums">
                    {value === null ? '' : METRIC_INFO[metric].format(value)}
                  </td>
                );
              })}
              <td />
            </tr>
          </thead>
          <tbody>
            {sorted.map((entry) => {
              const name = entry.nick || entry.steamid64;
              
              return (
                <tr key={entry.steamid64} className="border-b border-border-subtle last:border-b-0 hover:bg-surface-3/50">
                  <th scope="row" className="sticky left-0 z-10 bg-surface-2 px-3 py-1.5 text-left font-medium">
                    <span className="flex items-center gap-2">
                      <PlayerAvatar nickname={name} playerID={entry.steamid64} size={24} />
                      <Link href={playerHref(entry.steamid64)} className="truncate text-fg-1 hover:text-primary hover:underline">
                        {name}
                      </Link>
                    </span>
                  </th>
                  <td className="px-3 py-1.5 whitespace-nowrap text-fg-2">{roles.get(entry.steamid64) ?? ''}</td>
                  <td className="px-3 py-1.5 text-right text-fg-2 tabular-nums">{entry.matches}</td>
                  {ROW_METRICS.map((metric) => {
                    const info = METRIC_INFO[metric];
                    const row = ready?.boards.get(metric)?.players.find((line) => line.steamid64 === entry.steamid64);
                    return (
                      <td key={metric} className="px-3 py-1.5 text-right">
                        {data === null ? (
                          <Skeleton className="ml-auto h-4 w-12" />
                        ) : (
                          <DeltaValue
                            value={row?.value ?? null}
                            format={info.format}
                            reference={baselineValue(baseline, metric)}
                            higherIsBetter={info.higherIsBetter}
                            epsilon={info.epsilon}
                            referenceLabel={label}
                          />
                        )}
                      </td>
                    );
                  })}
                  <td className="px-3 py-1.5 text-right whitespace-nowrap text-fg-3">{formatShortDate(entry.last_played_at)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {ready ? (
        <p className="measure-read text-meta text-fg-3">
          Cifras de cada jugador en todas sus partidas de la biblioteca{map === '' ? '' : ` en ${prettyMapName(map)}`}, no solo con este equipo.{' '}
          {scope === 'pro' ? proSourceText(ready.pro) : `La referencia sale de ${baselineSampleText(ready.library)}.${minRoundsNote(ready.minRounds)}`}
        </p>
      ) : null}
    </section>
  );
}
