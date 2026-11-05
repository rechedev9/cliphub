'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getLeaderboard } from '@/lib/api/library';
import { BASELINE_METRICS, BASELINE_SCOPES, type BaselineMetric, type BaselineScope, type Leaderboard } from '@/lib/api/baseline';
import { DEFAULT_MAP_POOL, EXTRA_POOL_MAPS } from '@/lib/api/veto';
import { METRIC_INFO, SCOPE_LABEL, leaderboardEmptyText } from '@/lib/analysis/reference';
import { playerHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';

/** Radix Select reserves the empty string, so "every map" travels as this. */
const ALL_MAPS = '__all__';
const MAPS = [...DEFAULT_MAP_POOL, ...EXTRA_POOL_MAPS].sort();
const MIN_ROUNDS = [20, 50, 100, 200] as const;
const DEFAULT_MIN_ROUNDS = 50;
const LIMIT = 100;

type Query = { metric: BaselineMetric; map: string; scope: BaselineScope; minRounds: number };

function isMetric(value: string): value is BaselineMetric {
  return BASELINE_METRICS.some((metric) => metric === value);
}

/** The library's players ranked by one metric, with the map, the demos and the minimum rounds as controls. */
export function RankingBoard(): ReactNode {
  const [query, setQuery] = useState<Query>({ metric: 'kpr', map: '', scope: 'library', minRounds: DEFAULT_MIN_ROUNDS });
  const [board, setBoard] = useState<Leaderboard | null | 'failed'>(null);
  const [attempt, setAttempt] = useState(0);
  const update = (patch: Partial<Query>): void => setQuery((current) => ({ ...current, ...patch }));

  useEffect(() => {
    let active = true;
    setBoard(null);
    getLeaderboard({ metric: query.metric, map: query.map, scope: query.scope, min_rounds: query.minRounds, limit: LIMIT })
      .then((next) => {
        if (active) setBoard(next);
      })
      .catch(() => {
        if (active) setBoard('failed');
      });
    return () => {
      active = false;
    };
  }, [query, attempt]);

  const info = METRIC_INFO[query.metric];
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <Select value={query.metric} onValueChange={(value) => (isMetric(value) ? update({ metric: value }) : undefined)}>
          <SelectTrigger aria-label="Cifra" className="h-9 min-w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {BASELINE_METRICS.map((metric) => (
              <SelectItem key={metric} value={metric}>
                {METRIC_INFO[metric].label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={query.map === '' ? ALL_MAPS : query.map} onValueChange={(value) => update({ map: value === ALL_MAPS ? '' : value })}>
          <SelectTrigger aria-label="Mapa" className="h-9 min-w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_MAPS}>Todos los mapas</SelectItem>
            {MAPS.map((map) => (
              <SelectItem key={map} value={map}>
                {prettyMapName(map)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Segmented
          label="Demos"
          value={query.scope}
          options={BASELINE_SCOPES.map((value) => ({ value, label: SCOPE_LABEL[value] }))}
          onChange={(scope) => update({ scope })}
        />
        <Select value={String(query.minRounds)} onValueChange={(value) => update({ minRounds: Number(value) })}>
          <SelectTrigger aria-label="Mínimo de rondas" className="h-9 min-w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {MIN_ROUNDS.map((rounds) => (
              <SelectItem key={rounds} value={String(rounds)}>
                {rounds} rondas o más
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <RankingTable board={board} info={info} onRetry={() => setAttempt((n) => n + 1)} />
    </div>
  );
}

function RankingTable({
  board,
  info,
  onRetry,
}: {
  board: Leaderboard | null | 'failed';
  info: (typeof METRIC_INFO)[BaselineMetric];
  onRetry: () => void;
}): ReactNode {
  if (board === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el ranking. Comprueba que ClipHub Studio sigue abierto y{' '}
        <button type="button" onClick={onRetry} className="underline underline-offset-4">
          vuelve a intentarlo
        </button>
        .
      </p>
    );
  }
  if (board === null) {
    return (
      <div role="status" aria-label="Calculando el ranking" className="studio-panel flex flex-col gap-2 p-4">
        {Array.from({ length: 8 }, (_, index) => (
          <Skeleton key={index} className="h-7 w-full" />
        ))}
      </div>
    );
  }
  if (board.players.length === 0) return <p className="text-body-sm text-fg-2">{leaderboardEmptyText(board)}</p>;
  return (
    <section aria-label={`Ranking de ${info.label}`} className="studio-panel flex flex-col">
      <div className="overflow-x-auto">
        <table aria-label={`Ranking de ${info.label}`} className="w-full border-collapse text-left text-body-sm">
          <thead className="bg-surface-2 text-fg-3">
            <tr className="border-b border-border">
              <th scope="col" className="h-9 w-12 px-3 text-right font-medium">#</th>
              <th scope="col" className="h-9 px-3 font-medium">Jugador</th>
              <th scope="col" className="h-9 px-3 text-right font-medium">{info.label}</th>
              <th scope="col" className="h-9 px-3 text-right font-medium">Rondas</th>
              <th scope="col" className="h-9 px-3 text-right font-medium">Partidas</th>
            </tr>
          </thead>
          <tbody>
            {board.players.map((row) => (
              <tr key={row.steamid64} className="border-b border-border-subtle last:border-b-0">
                <td className="px-3 py-2 text-right text-fg-3 tabular-nums">{row.rank}</td>
                <th scope="row" className="px-3 py-2 text-left font-medium">
                  <Link href={playerHref(row.steamid64)} className="text-fg-1 hover:text-primary hover:underline">
                    {row.name || row.steamid64}
                  </Link>
                </th>
                <td className="px-3 py-2 text-right font-medium text-fg-1 tabular-nums">{info.format(row.value)}</td>
                <td className="px-3 py-2 text-right text-fg-2 tabular-nums">{row.rounds}</td>
                <td className="px-3 py-2 text-right text-fg-2 tabular-nums">{row.matches}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="border-t border-border-subtle px-4 py-2 text-meta text-fg-3">
        {board.higher_is_better ? 'De mayor a menor' : 'De menor a mayor, porque morir menos es mejor'}. Leídas {board.matches}{' '}
        {board.matches === 1 ? 'partida' : 'partidas'}
        {board.skipped > 0 ? `; ${board.skipped} sin análisis no cuentan` : ''}. Solo entran jugadores con {board.min_rounds} rondas o más.
      </p>
    </section>
  );
}
