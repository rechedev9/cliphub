'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Clapperboard, Radar } from 'lucide-react';
import type { TacticalBuyType, TacticalSide } from '@/lib/api/tactical';
import { endReasonLabel } from '@/lib/library/partidas';
import { halfScoreText, isWeakBuy, roundEvidenceHref, type PerTeam, type RoundHalf, type RoundRow, type RoundSwing } from '@/lib/library/rounds';
import { percentText } from '@/lib/metrics/advanced';
import { secondsText } from '@/lib/reports/scout-sections';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { RoundResultTile } from '@/components/matches/round-result-tile';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

/** Whether the win chance per round is still loading, known, or not available for this match. */
export type ChanceStatus = 'loading' | 'ready' | 'none';

/** How many of the visible rounds are picked, which sets the header checkbox. */
export type VisiblePick = 'none' | 'some' | 'all';

export type RoundTableProps = {
  halves: readonly RoundHalf[];
  teamNames: PerTeam<string>;
  /** Player names by tactical slot. */
  playerNames: ReadonlyMap<number, string>;
  chance: ChanceStatus;
  picked: ReadonlySet<number>;
  visiblePick: VisiblePick;
  onTogglePick: (round: number) => void;
  onToggleVisible: () => void;
  pathname: string;
  onOpenRound: () => void;
  /** Absent where no video can be made (the web edition): no pick column and no video button. */
  onVideo?: (round: number) => void;
};

const HEAD = 'h-9 px-2.5 font-medium whitespace-nowrap';
const CELL = 'px-2.5 py-2 align-middle';

function sideText(side: TacticalSide): string {
  return side === 'CT' ? 'text-side-ct' : 'text-side-t';
}

function buyClass(buy: TacticalBuyType): string {
  return isWeakBuy(buy) || buy === 'unknown' ? 'text-fg-3' : 'text-fg-1';
}

/** Every round of a partida grouped by half, with a checkbox to pick several for one video. */
export function RoundTable(props: RoundTableProps): ReactNode {
  const { halves, teamNames, chance, visiblePick, onToggleVisible, onVideo } = props;
  const showSwing = chance !== 'none';
  const canPick = onVideo !== undefined;
  return (
    <div className="overflow-x-auto">
      <table aria-label="Rondas" className="w-full border-collapse text-left text-body-sm tabular-nums">
        <thead className="text-meta text-fg-3">
          <tr>
            {canPick ? (
              <th scope="col" className="w-10 py-1 pr-1 pl-4">
                <label className="grid size-8 cursor-pointer place-items-center">
                  <input
                    type="checkbox"
                    className="size-4 cursor-pointer accent-primary"
                    checked={visiblePick === 'all'}
                    ref={(input) => {
                      if (input) input.indeterminate = visiblePick === 'some';
                    }}
                    onChange={onToggleVisible}
                    aria-label="Elegir todas las rondas que se ven"
                  />
                </label>
              </th>
            ) : null}
            <th scope="col" className={cn(HEAD, !canPick && 'pl-4')}>Ronda</th>
            <th scope="col" className={HEAD}>Marcador</th>
            <th scope="col" className={HEAD}>Gana</th>
            <th scope="col" className={HEAD}>
              <span className="block">Compra</span>
              <span className="block max-w-24 truncate">{teamNames['ct-start']}</span>
            </th>
            <th scope="col" className={HEAD}>
              <span className="block">Compra</span>
              <span className="block max-w-24 truncate">{teamNames['t-start']}</span>
            </th>
            <th scope="col" className={HEAD}>Sitio</th>
            <th scope="col" className={HEAD}>Forma T · CT</th>
            {showSwing ? <th scope="col" className={HEAD}>La baja que más pesó</th> : null}
            <th scope="col" className={cn(HEAD, 'pr-4 text-right')}>Abrir</th>
          </tr>
        </thead>
        {halves.map((half) => (
          <tbody key={`${half.half}:${half.title}`}>
            <tr className="border-t border-border bg-surface-1">
              <th scope="colgroup" colSpan={(showSwing ? 10 : 9) - (canPick ? 0 : 1)} className="px-4 pt-3 pb-2 text-left font-normal">
                <span className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1">
                  <span className="text-body-sm font-semibold text-fg-1">{half.title}</span>
                  <span className="text-meta text-fg-2">
                    {teamNames['ct-start']} <span className={cn('font-semibold', sideText(half.sides['ct-start']))}>{half.sides['ct-start']}</span>
                    {' · '}
                    {teamNames['t-start']} <span className={cn('font-semibold', sideText(half.sides['t-start']))}>{half.sides['t-start']}</span>
                  </span>
                  <span className="text-meta text-fg-3">{halfScoreText(half)}</span>
                </span>
              </th>
            </tr>
            {half.rows.map((row) => (
              <RoundTableRow key={row.number} row={row} table={props} />
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}

function RoundTableRow({ row, table }: { row: RoundRow; table: RoundTableProps }): ReactNode {
  const { onVideo } = table;
  const picked = onVideo !== undefined && table.picked.has(row.number);
  return (
    <tr className={cn('border-t border-border-subtle hover:bg-surface-3', picked && 'bg-primary/8')}>
      {onVideo === undefined ? null : (
        <td className="py-1 pr-1 pl-4">
          <label className="grid size-8 cursor-pointer place-items-center">
            <input
              type="checkbox"
              className="size-4 cursor-pointer accent-primary"
              checked={picked}
              onChange={() => table.onTogglePick(row.number)}
              aria-label={`Elegir la ronda ${row.number} para un vídeo`}
            />
          </label>
        </td>
      )}
      <td className={cn(CELL, 'font-semibold text-fg-1', onVideo === undefined && 'pl-4')}>{row.number}</td>
      <td className={cn(CELL, 'whitespace-nowrap text-fg-2')}>
        <span className={row.winnerTeamKey === 'ct-start' ? 'font-semibold text-fg-1' : 'text-fg-3'}>{row.scoreAfter['ct-start']}</span>
        <span className="text-fg-4">-</span>
        <span className={row.winnerTeamKey === 't-start' ? 'font-semibold text-fg-1' : 'text-fg-3'}>{row.scoreAfter['t-start']}</span>
      </td>
      <td className={CELL}>
        <WinnerCell row={row} teamNames={table.teamNames} />
      </td>
      <td className={cn(CELL, 'whitespace-nowrap', buyClass(row.buys['ct-start']))}>{buyLabel(row.buys['ct-start'])}</td>
      <td className={cn(CELL, 'whitespace-nowrap', buyClass(row.buys['t-start']))}>{buyLabel(row.buys['t-start'])}</td>
      <td className={cn(CELL, 'whitespace-nowrap text-fg-2')}>{siteLabel(row.round.class.site)}</td>
      <td className={cn(CELL, 'whitespace-nowrap text-fg-2')}>
        <span className="flex flex-col">
          <span>{tPatternLabel(row.round.class.t_side)}</span>
          <span>{ctPatternLabel(row.round.class.ct_side)}</span>
        </span>
      </td>
      {table.chance === 'none' ? null : (
        <td className={CELL}>
          <SwingCell row={row} table={table} />
        </td>
      )}
      <td className="py-2 pr-4 pl-3">
        <div className="flex justify-end gap-1">
          <Button asChild size="xs" variant="ghost">
            <Link
              href={roundEvidenceHref(table.pathname, row.number, row.tick)}
              onClick={table.onOpenRound}
              aria-label={`Abrir la ronda ${row.number} en el Replay`}
            >
              <Radar aria-hidden />
              Táctica
            </Link>
          </Button>
          {onVideo === undefined ? null : (
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              onClick={() => onVideo(row.number)}
              aria-label={`Crear un vídeo de la ronda ${row.number}`}
              title="Crear un vídeo de esta ronda"
            >
              <Clapperboard aria-hidden />
            </Button>
          )}
        </div>
      </td>
    </tr>
  );
}

function WinnerCell({ row, teamNames }: { row: RoundRow; teamNames: PerTeam<string> }): ReactNode {
  const duration = secondsText(row.liveSeconds);
  if (row.winnerSide === '' || row.winnerTeamKey === '') {
    return (
      <span className="flex flex-col">
        <span className="text-fg-3">Sin ganador</span>
        <span className="text-meta whitespace-nowrap text-fg-3">
          {row.round.end_reason === '' ? `Duró ${duration}` : `${endReasonLabel(row.round.end_reason)} en ${duration}`}
        </span>
      </span>
    );
  }
  const start = row.chance?.upset ? row.chance.winnerStart : null;
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <RoundResultTile side={row.winnerSide} endReason={row.round.end_reason} size="sm" />
      <span className="flex min-w-0 flex-col">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="max-w-44 truncate font-medium text-fg-1">{teamNames[row.winnerTeamKey]}</span>
          {start === null ? null : (
            <Badge variant="outline" title={`Contra pronóstico: empezó la ronda con un ${percentText(start)} de victoria`}>
              <span className="sr-only">Contra pronóstico, empezó con un </span>
              {percentText(start)}
            </Badge>
          )}
        </span>
        <span className="text-meta whitespace-nowrap text-fg-3">
          {endReasonLabel(row.round.end_reason)} en {duration}
        </span>
      </span>
    </span>
  );
}

function killText(swing: RoundSwing, names: ReadonlyMap<number, string>): { shown: string; spoken: string } {
  const name = (slot: number): string => names.get(slot) ?? `Jugador ${slot}`;
  const target = swing.targetSlot === undefined ? 'Un jugador' : name(swing.targetSlot);
  if (swing.actorSlot === undefined) return { shown: `${target} muere`, spoken: `${target} muere` };
  const actor = name(swing.actorSlot);
  return { shown: `${actor} > ${target}`, spoken: `${actor} mata a ${target}` };
}

function SwingCell({ row, table }: { row: RoundRow; table: RoundTableProps }): ReactNode {
  if (table.chance === 'loading') return <Skeleton className="h-8 w-36" />;
  if (row.chance === null) return <span className="text-fg-3">Sin dato</span>;
  const swing = row.chance.swing;
  if (swing === null) return <span className="text-fg-3">Ninguna baja</span>;
  const kill = killText(swing, table.playerNames);
  return (
    <span className="flex min-w-0 flex-col">
      <Link
        href={roundEvidenceHref(table.pathname, row.number, swing.tick)}
        onClick={table.onOpenRound}
        aria-label={`Ver en el Replay: ${kill.spoken}`}
        className="max-w-44 truncate text-fg-1 hover:text-primary hover:underline"
      >
        {kill.shown}
      </Link>
      <span className="text-meta whitespace-nowrap text-fg-3">
        {secondsText(swing.seconds)} ·{' '}
        <span className={cn('font-semibold', sideText(swing.side))}>
          {swing.side} +{percentText(swing.gain)}
        </span>
      </span>
    </span>
  );
}
