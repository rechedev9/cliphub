'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getBaseline, getLeaderboard } from '@/lib/api/library';
import { BASELINE_METRICS, type Baseline, type BaselineMetric, type BaselineScope, type Leaderboard } from '@/lib/api/baseline';
import { DEFAULT_MAP_POOL, EXTRA_POOL_MAPS } from '@/lib/api/veto';
import { scaleBars } from '@/lib/analysis/playbook';
import { METRIC_INFO, SCOPE_LABEL, baselineSampleText, baselineValue, leaderboardEmptyText, proSourceText } from '@/lib/analysis/reference';
import { versusRows, type VersusRow } from '@/lib/analysis/versus-board';
import { MATCHES_HREF, playerHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DeltaValue } from '@/components/ui/delta-value';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { GlobalRanking } from '@/components/ranking/global-ranking';

/** Radix Select reserves the empty string, so "every map" travels as this. */
const ALL_MAPS = '__all__';
const MAPS = [...DEFAULT_MAP_POOL, ...EXTRA_POOL_MAPS].sort();
const MIN_ROUNDS = [20, 50, 100, 200] as const;
const DEFAULT_MIN_ROUNDS = 50;
const LIMIT = 100;
const TRIGGER = 'h-8 gap-1.5 px-2.5 text-body-sm';
const SHORT_METRIC: Readonly<Record<BaselineMetric, string>> = {
  kpr: 'Bajas/r',
  dpr: 'Muertes/r',
  adr: 'ADR',
  kast: 'KAST',
  opening_win_pct: 'Aperturas',
  trade_pct: 'Tradeadas',
  utility_damage_per_round: 'Daño util./r',
  flash_assists_per_round: 'Flash asist./r',
  wpa_per_round: 'WPA/r',
};

/** Which players: the library, the pros, both ranked together, or the own player among every opted-in Studio. */
type BoardScope = BaselineScope | 'versus' | 'global';
const BOARD_SCOPES: readonly { value: BoardScope; label: string }[] = [
  { value: 'library', label: SCOPE_LABEL.library },
  { value: 'pro', label: SCOPE_LABEL.pro },
  { value: 'versus', label: 'Tú frente a profesionales' },
  { value: 'global', label: 'Global' },
];

type Query = { metric: BaselineMetric; map: string; scope: BoardScope; minRounds: number };
/** `board` carries the counts; `rows` are the players shown, pros marked. `baseline` is the mean row's reference. */
type Loaded = { board: Leaderboard; rows: readonly VersusRow[]; baseline: Baseline | null };

async function loadBoard(query: Query): Promise<Loaded> {
  const base = { map: query.map, min_rounds: query.minRounds };
  const reference: BaselineScope = query.scope === 'library' ? 'library' : 'pro';
  const baseline = getBaseline({ ...base, scope: reference }).catch(() => null);
  if (query.scope === 'library' || query.scope === 'pro') {
    const board = await getLeaderboard({ ...base, scope: query.scope, metric: query.metric, limit: LIMIT });
    const pro = query.scope === 'pro';
    return { board, rows: board.players.map((row) => ({ ...row, pro })), baseline: await baseline };
  }
  const [library, pro] = await Promise.all([
    getLeaderboard({ ...base, scope: 'library', metric: query.metric, limit: LIMIT }),
    getLeaderboard({ ...base, scope: 'pro', metric: query.metric, limit: LIMIT }),
  ]);
  return { board: library, rows: versusRows(library, pro, LIMIT), baseline: await baseline };
}

/** The library's players ranked by one metric, with the map, the demos and the minimum rounds as controls. */
export function RankingBoard(): ReactNode {
  const [query, setQuery] = useState<Query>({ metric: 'kpr', map: '', scope: 'library', minRounds: DEFAULT_MIN_ROUNDS });
  const [data, setData] = useState<Loaded | null | 'failed'>(null);
  const [attempt, setAttempt] = useState(0);
  const update = (patch: Partial<Query>): void => setQuery((current) => ({ ...current, ...patch }));

  const global = query.scope === 'global';
  useEffect(() => {
    if (global) return undefined;
    let active = true;
    setData(null);
    loadBoard(query)
      .then((loaded) => {
        if (active) setData(loaded);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [query, attempt, global]);

  const info = METRIC_INFO[query.metric];
  return (
    <div className="flex flex-col gap-3">
      <div
        role="group"
        aria-label="Qué ranking"
        className="sticky top-(--shell-strip-height) z-20 -mx-1 flex flex-col gap-1.5 border-b border-border bg-background px-1 py-1.5"
      >
        <div className="flex flex-wrap items-center gap-1.5">
          <Select value={query.map === '' ? ALL_MAPS : query.map} onValueChange={(value) => update({ map: value === ALL_MAPS ? '' : value })}>
            <SelectTrigger aria-label="Mapa" className={TRIGGER}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_MAPS}>Todos los mapas</SelectItem>
              {MAPS.map((map) => (
                <SelectItem key={map} value={map}>
                  <span className="flex items-center gap-2">
                    <MapThumb map={map} className="h-4 w-6" crop={1.6} />
                    {prettyMapName(map)}
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {global ? null : (
          <Select value={String(query.minRounds)} onValueChange={(value) => update({ minRounds: Number(value) })}>
            <SelectTrigger aria-label="Mínimo de rondas" className={TRIGGER}>
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
          )}
          <div className="ml-auto flex items-center gap-1.5">
            <span className="text-meta whitespace-nowrap text-fg-3">Demos</span>
            <Segmented label="Demos" value={query.scope} options={BOARD_SCOPES} onChange={(scope) => update({ scope })} />
            {!global && data !== null && data !== 'failed' ? (
              <p className="flex h-8 items-center rounded-md border border-border px-2.5 text-meta font-medium whitespace-nowrap text-fg-1 tabular-nums">
                {data.board.matches} {data.board.matches === 1 ? 'partida' : 'partidas'} · {data.board.rounds} rondas
              </p>
            ) : null}
          </div>
        </div>
        {global ? null : (
        <Segmented
          label="Cifra"
          value={query.metric}
          options={BASELINE_METRICS.map((metric) => ({ value: metric, label: SHORT_METRIC[metric] }))}
          onChange={(metric) => update({ metric })}
        />
        )}
      </div>
      {global ? (
        <GlobalRanking map={query.map} />
      ) : (
        <RankingTable
          data={data}
          info={info}
          onRetry={() => setAttempt((n) => n + 1)}
          onLowerMinRounds={query.minRounds > MIN_ROUNDS[0] ? () => update({ minRounds: MIN_ROUNDS[0] }) : undefined}
        />
      )}
    </div>
  );
}

function RankingTable({
  data,
  info,
  onRetry,
  onLowerMinRounds,
}: {
  data: Loaded | null | 'failed';
  info: (typeof METRIC_INFO)[BaselineMetric];
  onRetry: () => void;
  /** Present while a lower minimum exists, so an empty board offers it as a button. */
  onLowerMinRounds?: () => void;
}): ReactNode {
  if (data === 'failed') {
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
  if (data === null) {
    return (
      <div role="status" aria-label="Calculando el ranking" className="studio-panel flex flex-col gap-1.5 p-3">
        {Array.from({ length: 12 }, (_, index) => (
          <Skeleton key={index} className="h-6 w-full" />
        ))}
      </div>
    );
  }
  const { board, rows, baseline } = data;
  if (rows.length === 0) {
    let action: ReactNode = null;
    if (board.matches === 0) {
      action = (
        <Button asChild variant="outline" size="sm">
          <Link href={MATCHES_HREF}>Importar demos</Link>
        </Button>
      );
    } else if (onLowerMinRounds) {
      action = (
        <Button type="button" variant="outline" size="sm" onClick={onLowerMinRounds}>
          Ver con {MIN_ROUNDS[0]} rondas o más
        </Button>
      );
    }
    return (
      <div className="studio-panel flex flex-col items-start gap-3 px-4 py-4">
        <p className="text-body-sm text-fg-2">{leaderboardEmptyText(board)}</p>
        {action}
      </div>
    );
  }
  const metric = board.metric;
  const mean = baselineValue(baseline, metric);
  const widths = scaleBars(rows.map((row) => row.value));
  const label = `Media de ${baseline?.scope === 'pro' ? 'los pros' : 'la biblioteca'}`;
  const mixed = rows.some((row) => row.pro) && rows.some((row) => !row.pro);
  return (
    <section aria-label={`Ranking de ${info.label}`} className="studio-panel flex flex-col">
      <div className="overflow-x-auto">
        <table aria-label={`Ranking de ${info.label}`} className="w-full border-collapse text-left text-body-sm">
          <thead className="bg-surface-2 text-meta text-fg-3">
            <tr className="border-b border-border">
              <th scope="col" className="h-8 w-10 px-2.5 text-right font-medium">#</th>
              <th scope="col" className="sticky left-0 z-10 h-8 bg-surface-2 px-2.5 font-medium">Jugador</th>
              <th scope="col" className="h-8 px-2.5 text-right font-medium whitespace-nowrap">{info.label}</th>
              <th scope="col" className="h-8 w-[40%] px-2.5 font-medium">
                <span className="sr-only">Barra</span>
              </th>
              <th scope="col" className="h-8 px-2.5 text-right font-medium">Rondas</th>
              <th scope="col" className="h-8 px-2.5 text-right font-medium">Partidas</th>
            </tr>
            {mean === null ? null : (
              <tr className="border-b border-border">
                <td />
                <th scope="row" className="sticky left-0 z-10 bg-surface-2 px-2.5 py-1 text-left font-normal">
                  {label}
                </th>
                <td className="px-2.5 py-1 text-right font-medium text-fg-2 tabular-nums">{info.format(mean)}</td>
                <td colSpan={3} className="px-2.5 py-1">
                  {baseline ? baselineSampleText(baseline) : ''}
                </td>
              </tr>
            )}
          </thead>
          <tbody>
            {rows.map((row, index) => (
              <tr key={row.steamid64} className="border-b border-border-subtle last:border-b-0 hover:bg-surface-3/50">
                <td className="px-2.5 py-1 text-right text-fg-3 tabular-nums">{row.rank}</td>
                <th scope="row" className="sticky left-0 z-10 bg-surface-2 px-2.5 py-1 text-left font-medium">
                  <span className="flex items-center gap-2">
                    {row.team === undefined ? (
                      <Link href={playerHref(row.steamid64)} className="text-fg-1 hover:text-primary hover:underline">
                        {row.name || row.steamid64}
                      </Link>
                    ) : (
                      <span className="text-fg-1">{row.name || row.steamid64}</span>
                    )}
                    {mixed && row.pro ? <Badge variant="outline">Pro</Badge> : null}
                    {row.team ? <span className="text-meta font-normal whitespace-nowrap text-fg-3">{row.team}</span> : null}
                  </span>
                </th>
                <td className="px-2.5 py-1 text-right">
                  <DeltaValue
                    value={row.value}
                    format={info.format}
                    reference={mean}
                    higherIsBetter={info.higherIsBetter}
                    epsilon={info.epsilon}
                    referenceLabel={label}
                  />
                </td>
                <td className="px-2.5 py-1">
                  <span aria-hidden className="block h-1.5 rounded-sm bg-surface-3">
                    <span className="block h-full rounded-sm bg-primary/60" style={{ width: `${widths[index] ?? 0}%` }} />
                  </span>
                </td>
                <td className="px-2.5 py-1 text-right text-fg-2 tabular-nums">{row.rounds}</td>
                <td className="px-2.5 py-1 text-right text-fg-2 tabular-nums">{row.matches}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="border-t border-border-subtle px-3 py-1.5 text-meta text-fg-3">
        {board.higher_is_better ? 'De mayor a menor' : 'De menor a mayor, porque morir menos es mejor'}. ▲▼ frente a la media. Leídas{' '}
        {board.matches} {board.matches === 1 ? 'partida' : 'partidas'}
        {board.skipped > 0 ? `; ${board.skipped} sin análisis no cuentan` : ''}. Solo entran jugadores con {board.min_rounds} rondas o más.
        {baseline?.scope === 'pro' ? ` ${proSourceText(baseline)}` : ''}
      </p>
    </section>
  );
}
