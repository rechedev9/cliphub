'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import type { LibraryMatch } from '@/lib/api/library';
import type { MetricsReport } from '@/lib/api/metrics';
import { playerHref } from '@/lib/clips/routes';
import { teamByKey, teamLabel } from '@/lib/library/partidas';
import { ppText, roleText } from '@/lib/metrics/advanced';
import { GENERAL_COLUMNS, bestValues, generalText, isBest, playerRows, teamGroups, type GeneralColumn } from '@/lib/metrics/match-view';
import { cn } from '@/lib/utils';

const COLUMN_LABEL: Record<GeneralColumn, string> = {
  kast: 'KAST',
  adr: 'ADR',
  kpr: 'Bajas/r',
  opening: 'Apertura',
  trades: 'Trades',
  utility: 'Utilidad/r',
};

const COLUMN_HINT: Record<GeneralColumn, string> = {
  kast: 'Rondas con baja, asistencia, supervivencia o trade',
  adr: 'Daño medio por ronda',
  kpr: 'Bajas por ronda',
  opening: 'Duelos de apertura ganados',
  trades: 'Bajas al rival que acababa de matar a un compañero',
  utility: 'Daño de HE y molotovs por ronda',
};

function impactClass(value: number): string {
  if (value > 0.0005) return 'text-outcome-win';
  if (value < -0.0005) return 'text-outcome-loss';
  return 'text-fg-2';
}

/** Both teams of one match in one table, with the best figure of each column marked. */
export function MatchPlayersTable({ match, report }: { match: LibraryMatch; report: MetricsReport }): ReactNode {
  const rows = playerRows(report, match.players ?? []);
  const best = bestValues(rows);
  // One match rarely gives a role; a column of "Sin muestra suficiente" says nothing.
  const showRole = rows.some((row) => row.player.role !== 'unknown');
  if (rows.length === 0) {
    return <p className="text-body-sm text-fg-2">Ningún jugador de la partida jugó estas rondas.</p>;
  }
  return (
    <div className="studio-panel overflow-hidden">
      <div className="overflow-x-auto">
        <table aria-label="Métricas de cada jugador" className="w-full border-collapse text-left text-body-sm">
          <thead className="bg-surface-2 text-meta text-fg-3">
            <tr className="border-b border-border">
              <th scope="col" className="h-9 px-4 font-medium">Jugador</th>
              {showRole ? <th scope="col" className="h-9 px-2.5 font-medium">Rol</th> : null}
              <th scope="col" className="h-9 px-2.5 text-right font-medium" title="Puntos de probabilidad de ganar la ronda que aporta de media">
                Impacto
              </th>
              {GENERAL_COLUMNS.map((column) => (
                <th key={column} scope="col" className="h-9 px-2.5 text-right font-medium whitespace-nowrap last:pr-4" title={COLUMN_HINT[column]}>
                  {COLUMN_LABEL[column]}
                </th>
              ))}
            </tr>
          </thead>
          {teamGroups(rows).map((group, index) => {
            const team = group.teamKey === undefined ? undefined : teamByKey(match, group.teamKey);
            return (
              <tbody key={group.teamKey ?? 'sin-equipo'}>
                <tr className={cn(index > 0 && 'border-t border-border')}>
                  <th scope="colgroup" colSpan={(showRole ? 3 : 2) + GENERAL_COLUMNS.length} className="px-4 pt-2.5 pb-1.5 text-meta font-semibold text-fg-2">
                    {team ? `${teamLabel(team)} · ${team.score} ${team.score === 1 ? 'ronda ganada' : 'rondas ganadas'}` : 'Sin equipo'}
                  </th>
                </tr>
                {group.rows.map((row) => (
                  <tr key={row.player.steamid64} className="border-t border-border-subtle hover:bg-surface-3">
                    <th scope="row" className="h-10 px-4 text-left font-medium">
                      <Link href={playerHref(row.player.steamid64)} className="block max-w-48 truncate text-fg-1 hover:text-primary hover:underline">
                        {row.player.name || row.player.steamid64}
                      </Link>
                    </th>
                    {showRole ? (
                      <td className="px-2.5 whitespace-nowrap text-fg-2">{row.player.role === 'unknown' ? 'Sin dato' : roleText(row.player.role)}</td>
                    ) : null}
                    <td className={cn('px-2.5 text-right font-semibold whitespace-nowrap', impactClass(row.player.swing.per_round))}>
                      {ppText(row.player.swing.per_round)}
                    </td>
                    {GENERAL_COLUMNS.map((column) => {
                      const value = row.values[column];
                      const top = isBest(value, best[column]);
                      return (
                        <td
                          key={column}
                          className={cn(
                            'px-2.5 text-right whitespace-nowrap last:pr-4',
                            top ? 'font-semibold text-outcome-win' : 'text-fg-1',
                            value === null && 'text-fg-3',
                          )}
                        >
                          {generalText(column, value)}
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            );
          })}
        </table>
      </div>
      <p className="border-t border-border-subtle px-4 pt-3 pb-4 text-meta text-fg-3">
        En verde, la mejor cifra de la partida en cada columna. Impacto: puntos de probabilidad de ganar la ronda que aporta de media cada jugador.
      </p>
    </div>
  );
}
