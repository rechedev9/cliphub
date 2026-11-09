'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { Trophy } from 'lucide-react';
import { getLeaderboard, isLibraryOffline } from '@/lib/api/library';
import type { Leaderboard } from '@/lib/api/baseline';
import { DEFAULT_MAP_POOL, EXTRA_POOL_MAPS } from '@/lib/api/veto';
import { resetScopeFields, type AnalysisScope, type ScopeField } from '@/lib/analysis/scope';
import {
  boardSampleText,
  leaderboardRequest,
  nextDir,
  parseRanking,
  patchRanking,
  rankingEntries,
  type PinnedPlayer,
  type RankingEntry,
  sortRankingRows,
  writeRanking,
  type RankingSort,
  type RankingState,
} from '@/lib/ranking/ranking';
import { ScopeBar } from '@/components/analysis/scope-bar';
import { EmptyState } from '@/components/records/empty-state';
import { RankingFilters } from '@/components/ranking/ranking-filters';
import { RankingTable } from '@/components/ranking/ranking-table';
import { usePinnedPlayers } from '@/components/ranking/use-pinned-players';
import { Skeleton } from '@/components/ui/skeleton';

type Load = { key: string; board: Leaderboard } | { key: string; failed: 'offline' | 'error' } | null;

const SCOPE_FIELDS: readonly ScopeField[] = ['map', 'side', 'period'];
const MAP_OPTIONS = [...DEFAULT_MAP_POOL, ...EXTRA_POOL_MAPS].sort();

function failureMessage(failed: 'offline' | 'error', pro: boolean): string {
  if (failed === 'offline') return 'No se pudo leer el ranking. Comprueba que ClipHub Studio sigue abierto.';
  if (pro) return 'Los datos pro aún no están disponibles. Prueba de nuevo en un rato o usa los datos de tu biblioteca.';
  return 'El ranking no se pudo calcular con este filtro. Prueba con otro mapa o periodo.';
}

function RankingBody({
  load,
  entries,
  state,
  onSort,
  onPin,
}: {
  load: Load;
  entries: readonly RankingEntry[];
  state: RankingState;
  onSort: (sort: RankingSort) => void;
  onPin: (pin: PinnedPlayer) => void;
}): ReactNode {
  if (load === null) {
    return (
      <div role="status" aria-label="Calculando el ranking" className="flex flex-col gap-2">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  const pro = state.scope.reference === 'pro';
  if ('failed' in load) {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        {failureMessage(load.failed, pro)}
      </p>
    );
  }
  if (entries.length === 0) {
    return (
      <EmptyState
        icon={Trophy}
        title="Nadie llega al mínimo de rondas"
        description={`Baja las rondas mínimas, amplía el periodo o quita el rol y la posición${pro ? ', el evento o el nivel de los equipos' : ''}. ${
          pro ? 'Los profesionales salen de los mapas de HLTV que indexa el servidor de ClipHub.' : 'El ranking sale de las demos analizadas de tu biblioteca.'
        }`}
      />
    );
  }
  return (
    <>
      <RankingTable entries={entries} state={state} boardMetric={load.board.metric} onSort={onSort} onPin={onPin} />
      <p className="measure-read text-meta text-fg-3">
        {boardSampleText(load.board)}. El rol se lee en los primeros 25 segundos de cada ronda; con menos de 4 rondas leídas no hay rol. El rating es el
        Rating de ClipHub; su fórmula está en la cabecera de la columna.{state.scope.reference === 'pro' && load.board.days ? ` Últimos ${load.board.days} días.` : ''}
        {pro ? ' Estos mapas no están en tu biblioteca, así que sus rondas no se abren en el Replay.' : ''}
      </p>
    </>
  );
}

/** The ranking of players: filters in the URL, one table, pinned players on top, and their rounds in the Replay. */
export function RankingView(): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const query = useSearchParams().toString();
  const state = useMemo(() => parseRanking(new URLSearchParams(query)), [query]);
  const { pins, toggle } = usePinnedPlayers();
  const [load, setLoad] = useState<Load>(null);

  const change = useCallback(
    (next: RankingState) => {
      const search = writeRanking(next, query);
      if (search !== query) router.replace(search === '' ? pathname : `${pathname}?${search}`, { scroll: false });
    },
    [pathname, query, router],
  );

  useEffect(() => {
    let active = true;
    getLeaderboard(leaderboardRequest(state, Date.now()))
      .then((board) => {
        if (active) setLoad({ key: query, board });
      })
      .catch((error: unknown) => {
        if (active) setLoad({ key: query, failed: isLibraryOffline(error) ? 'offline' : 'error' });
      });
    return () => {
      active = false;
    };
  }, [state, query]);

  const current = load?.key === query ? load : null;
  const board = current !== null && 'board' in current ? current.board : null;
  const sorted = useMemo(() => (board === null ? [] : sortRankingRows(board.players, state, board.metric)), [board, state]);
  const entries = useMemo(() => rankingEntries(sorted, pins), [sorted, pins]);

  function sortBy(sort: RankingSort): void {
    change({ ...state, sort, dir: nextDir(state, sort) });
  }

  function resetScope(fields: readonly ScopeField[]): void {
    change(patchRanking(state, { scope: resetScopeFields(state.scope, fields) }));
  }

  const patchScope = (patch: Partial<AnalysisScope>): void => change(patchRanking(state, { scope: patch }));

  return (
    <div className="flex flex-col gap-3">
      <ScopeBar
        scope={state.scope}
        fields={SCOPE_FIELDS}
        maps={MAP_OPTIONS}
        onChange={patchScope}
        onReset={resetScope}
        counts={board === null ? null : { maps: board.matches, rounds: board.rounds }}
      />
      <RankingFilters
        state={state}
        onRole={(role) => change(patchRanking(state, { role }))}
        onLevel={(level) => change(patchRanking(state, { level }))}
        onTop={(top) => change(patchRanking(state, { top }))}
        onTeamTop={(teamTop) => change(patchRanking(state, { teamTop }))}
        onEvent={(event) => change(patchRanking(state, { event }))}
        onMapRole={(mapRole) => change(patchRanking(state, { mapRole }))}
        onMinRounds={(minRounds) => change(patchRanking(state, { minRounds }))}
        onReference={(reference) => change(patchRanking(state, { scope: { reference }, role: '', level: '', top: null, teamTop: null, event: '' }))}
        mapRoles={board?.map_roles ?? []}
        events={board?.events ?? []}
        players={board?.players ?? []}
        pins={pins}
        onPin={toggle}
      />
      {board !== null && state.scope.reference === 'pro' && board.description ? <p className="measure-read text-meta text-fg-3">{board.description}</p> : null}
      <RankingBody load={current} entries={entries} state={state} onSort={sortBy} onPin={toggle} />
    </div>
  );
}
