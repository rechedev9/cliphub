'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ArrowDown, ArrowUp, Pin, PinOff, Play, TriangleAlert } from 'lucide-react';
import type { LeaderboardMetric, LeaderboardRow } from '@/lib/api/baseline';
import { playerHref } from '@/lib/clips/routes';
import { RANKING_COLUMNS, libraryRoleText, roleChips } from '@/lib/ranking/columns';
import {
  LOW_SAMPLE_ROUNDS,
  cellValue,
  lowSample,
  naturalDir,
  playerRoundsHref,
  type PinnedPlayer,
  type RankingEntry,
  type RankingSort,
  type RankingState,
} from '@/lib/ranking/ranking';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { RoleChip } from '@/components/ui/role-chip';
import { cn } from '@/lib/utils';

/** The sorted column reads as a slightly lighter band down the table. */
const BAND = 'bg-surface-3/50';
/** Role pairs sit under the nick as plain coloured text, not boxed chips. */
const ROLE_PAIR = 'min-h-0 border-transparent bg-transparent px-0 py-0 font-semibold';
const CAPS = 'text-meta font-semibold tracking-widest text-fg-3 uppercase';

function subtitle(row: LeaderboardRow): string {
  const matches = `${row.matches} ${row.matches === 1 ? 'partida' : 'partidas'}`;
  return row.team === undefined ? matches : `${row.team} · ${matches}`;
}

function ariaSort(active: boolean, dir: 'asc' | 'desc'): 'ascending' | 'descending' | 'none' {
  if (!active) return 'none';
  return dir === 'asc' ? 'ascending' : 'descending';
}

function SortHeader({ column, state, onSort }: { column: (typeof RANKING_COLUMNS)[number]; state: RankingState; onSort: (sort: RankingSort) => void }): ReactNode {
  const active = state.sort === column.sort;
  const dir = state.dir ?? naturalDir(column.sort);
  const Arrow = dir === 'asc' ? ArrowUp : ArrowDown;
  return (
    <th scope="col" aria-sort={ariaSort(active, dir)} className={cn('px-2 py-2.5 text-center whitespace-nowrap', CAPS, active && cn(BAND, 'text-fg-1'))}>
      <button
        type="button"
        title={column.title}
        onClick={() => onSort(column.sort)}
        className={cn(
          'inline-flex items-center gap-1 rounded-sm outline-none hover:text-fg-1 focus-visible:ring-[3px] focus-visible:ring-ring/50',
          active && 'text-fg-1',
        )}
      >
        {column.header}
        {active ? <Arrow aria-hidden className="size-3" /> : null}
      </button>
    </th>
  );
}

function PinButton({ pin, pinned, onToggle }: { pin: PinnedPlayer; pinned: boolean; onToggle: (pin: PinnedPlayer) => void }): ReactNode {
  const Icon = pinned ? PinOff : Pin;
  return (
    <button
      type="button"
      aria-pressed={pinned}
      aria-label={pinned ? `Dejar de fijar a ${pin.name}` : `Fijar a ${pin.name}`}
      title={pinned ? 'Dejar de fijar' : 'Fijar arriba'}
      onClick={() => onToggle(pin)}
      className={cn(
        'grid size-8 shrink-0 place-items-center rounded-sm text-fg-3 outline-none hover:text-fg-1 focus-visible:ring-[3px] focus-visible:ring-ring/50',
        pinned && 'text-primary',
      )}
    >
      <Icon aria-hidden className="size-3.5" />
    </button>
  );
}

function PlayerRow({
  entry,
  state,
  boardMetric,
  onPin,
}: {
  entry: RankingEntry;
  state: RankingState;
  boardMetric: LeaderboardMetric;
  onPin: (pin: PinnedPlayer) => void;
}): ReactNode {
  const { row } = entry;
  const pin: PinnedPlayer = entry.pinned ?? { steamid64: row?.steamid64 ?? '', name: row?.name ?? '' };
  const name = row?.name ?? pin.name;
  const steamId = row?.steamid64 ?? pin.steamid64;
  const chips = row === null ? [] : roleChips(row, state.scope.side);
  const libraryRole = row === null ? '' : libraryRoleText(row.library_role);
  const href = row === null ? null : playerRoundsHref(row, name);
  return (
    <tr className={cn('h-[45px] border-t border-border-subtle', entry.pinned && 'bg-surface-3/30')}>
      <td className="w-12 px-2 py-1 text-center font-semibold text-fg-2 tabular-nums">{entry.place ?? ''}</td>
      <td className="max-w-80 px-2 py-1">
        <div className="flex items-center gap-1">
          <PinButton pin={{ steamid64: steamId, name }} pinned={entry.pinned !== null} onToggle={onPin} />
          <PlayerAvatar nickname={name} playerID={steamId} size={28} />
          <div className="flex min-w-0 flex-col">
            <span className="flex min-w-0 items-baseline gap-1.5">
              {row?.team === undefined ? (
                <Link href={playerHref(steamId)} className="truncate font-bold text-fg-1 hover:underline">
                  {name}
                </Link>
              ) : (
                <span className="truncate font-bold text-fg-1">{name}</span>
              )}
              {row === null ? (
                <span className="truncate text-meta text-fg-3">Bajo el mínimo de rondas con este filtro</span>
              ) : (
                <span className="truncate text-meta text-fg-3" title={subtitle(row)}>
                  {subtitle(row)}
                </span>
              )}
            </span>
            {chips.length === 0 && libraryRole === '' ? null : (
              <span className="flex flex-wrap items-center gap-x-2">
                {chips.map((chip) => (
                  <RoleChip key={chip.side} side={chip.side} label={chip.label} breakdown={chip.breakdown} className={ROLE_PAIR} />
                ))}
                {libraryRole === '' ? null : <RoleChip label={libraryRole} className={ROLE_PAIR} />}
              </span>
            )}
          </div>
        </div>
      </td>
      {RANKING_COLUMNS.map((column) => {
        const value = row === null ? null : cellValue({ row, sort: column.sort, boardMetric });
        return (
          <td key={column.sort} className={cn('px-2 py-1 text-center font-bold whitespace-nowrap text-fg-1 tabular-nums', state.sort === column.sort && BAND)}>
            {value === null ? <span className="text-fg-4">-</span> : column.format(value)}
            {column.sort === 'rounds' && row !== null && lowSample(row) ? (
              <TriangleAlert
                aria-label={`Menos de ${LOW_SAMPLE_ROUNDS} rondas: las cifras son una lectura aproximada`}
                className="ml-1 inline size-3 text-fg-3"
              />
            ) : null}
          </td>
        );
      })}
      <td className="px-2 py-1 text-center">
        {href === null ? null : (
          <Link
            href={href}
            className="inline-flex h-8 items-center gap-1 rounded-sm px-2 text-body-sm whitespace-nowrap text-fg-1 outline-none hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <Play aria-hidden className="size-3" />
            Ver sus rondas
          </Link>
        )}
      </td>
    </tr>
  );
}

/** The ranking table. Sorting is the page's state, so the order lives in the URL. */
export function RankingTable({
  entries,
  state,
  boardMetric,
  onSort,
  onPin,
}: {
  entries: readonly RankingEntry[];
  state: RankingState;
  boardMetric: LeaderboardMetric;
  onSort: (sort: RankingSort) => void;
  onPin: (pin: PinnedPlayer) => void;
}): ReactNode {
  return (
    <div className="studio-panel overflow-x-auto">
      <table aria-label="Ranking de jugadores" className="w-full min-w-[56rem] text-body-sm">
        <thead>
          <tr>
            <th scope="col" className={cn('w-12 px-2 py-2.5 text-center', CAPS)}>
              Rank
            </th>
            <th scope="col" className={cn('px-2 py-2.5 text-left', CAPS)}>
              Jugador
            </th>
            {RANKING_COLUMNS.map((column) => (
              <SortHeader key={column.sort} column={column} state={state} onSort={onSort} />
            ))}
            <th scope="col" className="px-2 py-1.5">
              <span className="sr-only">Rondas en el Replay</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => (
            <PlayerRow
              key={entry.row?.steamid64 ?? entry.pinned?.steamid64}
              entry={entry}
              state={state}
              boardMetric={boardMetric}
              onPin={onPin}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
