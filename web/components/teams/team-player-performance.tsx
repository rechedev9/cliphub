'use client';

import { useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowDown, ArrowUp } from 'lucide-react';
import type { PlayerLine, SideStatsPlayer } from '@/lib/api/side-stats';
import { RATING_FORMULA_TEXT } from '@/lib/analysis/rating';
import { playerHref } from '@/lib/clips/routes';
import { formatDecimal } from '@/lib/format';
import { nextSort, sortRows, type TableSort } from '@/lib/records/table-view';
import { InfoTip } from '@/components/analysis/info-tip';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { SlashTitle } from '@/components/records/document-page';
import { DeltaValue } from '@/components/ui/delta-value';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { RoleChips } from './player-stat-card';
import { groupText, sideStatsState } from './team-side-stats';
import { useTeamSideStats, type TeamSideStatsQuery } from './use-team-side-stats';

type ColumnKey = keyof PlayerLine;

type Column = {
  key: ColumnKey;
  label: string;
  digits: number;
  pct?: boolean;
  /** DPR reads better when lower. */
  lowerIsBetter?: boolean;
  /** The smallest difference that earns an arrow. */
  epsilon: number;
  hint: string;
};

const COLUMNS: readonly Column[] = [
  { key: 'rating', label: 'Rating', digits: 2, epsilon: 0.005, hint: RATING_FORMULA_TEXT },
  { key: 'kpr', label: 'KPR', digits: 2, epsilon: 0.005, hint: 'Bajas por ronda.' },
  { key: 'kd', label: 'K/D', digits: 2, epsilon: 0.005, hint: 'Bajas entre muertes.' },
  { key: 'dpr', label: 'DPR', digits: 2, lowerIsBetter: true, epsilon: 0.005, hint: 'Muertes por ronda.' },
  { key: 'adr', label: 'ADR', digits: 1, epsilon: 0.05, hint: 'Daño por ronda.' },
  { key: 'kast', label: 'KAST', digits: 0, pct: true, epsilon: 0.5, hint: 'Rondas con baja, asistencia, supervivencia o trade.' },
  { key: 'udr', label: 'UDR', digits: 2, epsilon: 0.005, hint: 'Daño de HE y molotov a rivales por ronda.' },
  { key: 'far', label: 'FAR', digits: 2, epsilon: 0.005, hint: 'Asistencias de flash por ronda.' },
  { key: 'hs', label: 'HS %', digits: 0, pct: true, epsilon: 0.5, hint: 'Bajas con disparo a la cabeza.' },
];

function formatCell(value: number | null, column: Column): string {
  if (value === null) return '-';
  return column.pct ? `${Math.round(value)} %` : formatDecimal(value, column.digits);
}

function Cell({ value, column, group }: { value: number | null; column: Column; group: number | null }): ReactNode {
  return (
    <td className="px-2 py-3 text-right font-num font-bold text-fg-1 tabular-nums">
      {value === null ? (
        '-'
      ) : (
        <DeltaValue
          value={value}
          format={(v) => formatCell(v, column)}
          reference={group}
          higherIsBetter={column.lowerIsBetter !== true}
          epsilon={column.epsilon}
          referenceLabel="Media del grupo"
        />
      )}
    </td>
  );
}

function PlayerCell({ player }: { player: SideStatsPlayer }): ReactNode {
  const name = player.name || player.steamid64;
  return (
    <th scope="row" className="px-3 py-2 text-left font-normal">
      <div className="flex min-w-0 items-center gap-3">
        <PlayerAvatar nickname={name} playerID={player.steamid64} size={32} />
        <div className="flex min-w-0 flex-col gap-1">
          <Link href={playerHref(player.steamid64)} className={cn('truncate text-body font-bold text-fg-1 hover:text-primary hover:underline', FOCUS_RING)}>
            {name}
          </Link>
          <RoleChips player={player} side="" />
        </div>
      </div>
    </th>
  );
}

function ariaSort(sort: TableSort | null, key: ColumnKey): 'ascending' | 'descending' | undefined {
  if (sort?.column !== key) return undefined;
  return sort.direction === 'asc' ? 'ascending' : 'descending';
}

function SortIcon({ sort, column }: { sort: TableSort | null; column: ColumnKey }): ReactNode {
  if (sort?.column !== column) return null;
  const Icon = sort.direction === 'asc' ? ArrowUp : ArrowDown;
  return <Icon aria-hidden className="size-3" />;
}

function isColumnKey(key: string): key is ColumnKey {
  return COLUMNS.some((column) => column.key === key);
}

/** Rendimiento de jugadores, EDGE's Player Performance: a row per player with an arrow against the group, and the team's line under them. */
export function TeamPlayerPerformance(props: TeamSideStatsQuery): ReactNode {
  const data = useTeamSideStats(props);
  const [sort, setSort] = useState<TableSort | null>(null);
  const rows = useMemo(
    () => (data === null || data === 'failed' ? [] : sortRows(data.players, sort, (player, key) => (isColumnKey(key) ? player[key] : null))),
    [data, sort],
  );
  const state = sideStatsState(data);
  if (state !== null || data === null || data === 'failed') return state;
  const group = data.players_group_avg;
  const toggle = (key: ColumnKey): void => setSort((current) => nextSort(current, key));
  return (
    <section aria-label="Rendimiento de jugadores" className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <SlashTitle>Rendimiento de jugadores</SlashTitle>
        <InfoTip label="las flechas" text={`Cada flecha compara con la media de los jugadores del grupo. ${groupText(data)}`} />
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[46rem] border-separate border-spacing-y-1 text-body-sm">
          <thead>
            <tr>
              <th scope="col" className="px-3 pb-2 text-left text-meta font-bold tracking-wider text-fg-3 uppercase">
                Jugador
              </th>
              {COLUMNS.map((column) => (
                <th
                  key={column.key}
                  scope="col"
                  title={column.hint}
                  aria-sort={ariaSort(sort, column.key)}
                  className="px-2 pb-2 text-right"
                >
                  <button
                    type="button"
                    onClick={() => toggle(column.key)}
                    className={cn('inline-flex items-center gap-0.5 text-meta font-bold tracking-wider text-fg-3 uppercase hover:text-fg-1', FOCUS_RING)}
                  >
                    {column.label}
                    <SortIcon sort={sort} column={column.key} />
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((player) => (
              <tr key={player.steamid64} className="bg-surface-2 [&>*:first-child]:rounded-l-sm [&>*:last-child]:rounded-r-sm">
                <PlayerCell player={player} />
                {COLUMNS.map((column) => (
                  <Cell key={column.key} value={player[column.key]} column={column} group={group[column.key]} />
                ))}
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <th scope="row" className="px-3 pt-3 text-left text-meta font-bold tracking-wider text-fg-3 uppercase">
                Media
              </th>
              {COLUMNS.map((column) => (
                <td key={column.key} className="px-2 pt-3 text-right font-num font-bold text-fg-2 tabular-nums">
                  {formatCell(data.players_average[column.key], column)}
                </td>
              ))}
            </tr>
          </tfoot>
        </table>
      </div>
    </section>
  );
}
