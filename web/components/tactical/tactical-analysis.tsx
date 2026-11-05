'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { TriangleAlert } from 'lucide-react';
import { fetchTacticalTendencies } from '@/lib/api/tactical';
import type { TacticalFilter, TacticalSource, TacticalTendencies } from '@/lib/api/tactical';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { filterTacticalRounds, tacticalFilterFromQuery, tacticalFilterToQuery } from '@/lib/tactical-filter';
import { TACTICAL_EVIDENCE_QUERY } from '@/lib/library/rounds';
import { analysisErrorMessage, loadAnalysis } from '@/lib/replay/loaded-analysis';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { ReplayWorkspace } from '@/components/replay/replay-workspace';
import { TacticalDemoSummary } from '@/components/tactical/tactical-demo-summary';
import { TacticalFilterBar } from '@/components/tactical/tactical-filter-bar';
import { TacticalTendenciesPanel } from '@/components/tactical/tactical-tendencies';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
import { Segmented } from '@/components/ui/segmented';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

/** How the filtered rounds are read: replayed, or as heatmaps. */
type AnalysisMode = 'replay' | 'heatmap';

const MODES: readonly { value: AnalysisMode; label: string }[] = [
  { value: 'replay', label: 'Repetición' },
  { value: 'heatmap', label: 'Mapas de calor' },
];

function positiveIntParam(raw: string | null): number | null {
  if (raw === null || !/^[0-9]{1,9}$/.test(raw)) return null;
  const value = Number(raw);
  return value > 0 ? value : null;
}

/**
 * The ready workspace of one analysed demo: the replay of its filtered rounds
 * and their tendencies. `?round=` and `?tick=` open it on the round and moment
 * a number elsewhere links to.
 */
export function TacticalAnalysis({
  source,
  generatedAt,
}: {
  source: TacticalSource;
  generatedAt: string;
}): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [loaded, setLoaded] = useState<LoadedAnalysis | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedRound, setSelectedRound] = useState<number | null>(null);
  const [tendencies, setTendencies] = useState<TacticalTendencies | null>(null);
  const [tendenciesError, setTendenciesError] = useState<string | null>(null);
  const [mode, setMode] = useState<AnalysisMode>('replay');

  const filter = useMemo<TacticalFilter>(() => tacticalFilterFromQuery(searchParams), [searchParams]);
  const filterQuery = tacticalFilterToQuery(filter);
  const linkedRound = positiveIntParam(searchParams.get(TACTICAL_EVIDENCE_QUERY.round));
  const linkedTick = positiveIntParam(searchParams.get(TACTICAL_EVIDENCE_QUERY.tick));
  const { kind: sourceKind, id: sourceId } = source;

  useEffect(() => {
    if (linkedRound === null) return;
    setSelectedRound(linkedRound);
    setMode('replay');
  }, [linkedRound, linkedTick]);

  useEffect(() => {
    let active = true;
    setLoadError(null);
    loadAnalysis({ kind: sourceKind, id: sourceId })
      .then((next) => {
        if (active) setLoaded(next);
      })
      .catch((error: unknown) => {
        if (active) setLoadError(analysisErrorMessage(error));
      });
    return () => {
      active = false;
    };
  }, [sourceKind, sourceId]);

  useEffect(() => {
    let active = true;
    setTendenciesError(null);
    fetchTacticalTendencies({ kind: sourceKind, id: sourceId }, tacticalFilterFromQuery(new URLSearchParams(filterQuery)))
      .then((next) => {
        if (active) setTendencies(next);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setTendencies(null);
        setTendenciesError(analysisErrorMessage(error));
      });
    return () => {
      active = false;
    };
  }, [sourceKind, sourceId, filterQuery]);

  const applyFilter = useCallback(
    (next: TacticalFilter) => {
      const query = tacticalFilterToQuery(next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router],
  );

  const rounds = useMemo(
    () => (loaded === null ? [] : filterTacticalRounds(loaded.doc.teams, filter, loaded.doc.rounds)),
    [loaded, filter],
  );
  const entries = useMemo<ReplayRef[]>(
    () =>
      rounds.map((round) =>
        round.number === linkedRound && linkedTick !== null
          ? { matchId: sourceId, round: round.number, tick: linkedTick }
          : { matchId: sourceId, round: round.number },
      ),
    [rounds, sourceId, linkedRound, linkedTick],
  );
  // A filter that hides the open round moves to the first one it kept.
  const found = rounds.findIndex((round) => round.number === selectedRound);
  const index = found < 0 ? 0 : found;
  const seed = useMemo(() => (loaded === null ? undefined : { id: sourceId, analysis: loaded }), [loaded, sourceId]);

  if (loadError !== null) {
    return (
      <StudioEmptyState
        icon={TriangleAlert}
        title="No se pudo abrir el análisis"
        description={<span className="font-mono text-body-sm break-words text-destructive">{loadError}</span>}
        compact
      />
    );
  }

  if (loaded === null) return <TacticalWorkspaceSkeleton />;

  const filterBar = (
    <TacticalFilterBar doc={loaded.doc} filter={filter} onChange={applyFilter} matched={rounds.length} total={loaded.doc.rounds.length} />
  );

  return (
    <div className="flex flex-col gap-6 sm:gap-8">
      <TacticalDemoSummary doc={loaded.doc} generatedAt={generatedAt} />

      {sourceKind === 'library' ? <Segmented label="Cómo leer las rondas" value={mode} options={MODES} onChange={setMode} /> : null}

      {mode === 'heatmap' && sourceKind === 'library' ? (
        <div className="grid gap-5 lg:grid-cols-[230px_minmax(0,1fr)]">
          {filterBar}
          <SpatialWorkspace scope={{ kind: 'match', matchId: sourceId, filter: filterQuery }} view="heatmap" hideViewSwitch />
        </div>
      ) : (
        <ReplayWorkspace
          source={sourceKind}
          entries={entries}
          index={index}
          onIndexChange={(next) => setSelectedRound(rounds[next]?.number ?? null)}
          seed={seed}
          filters={filterBar}
        />
      )}

      <TacticalTendenciesPanel tendencies={tendencies} error={tendenciesError} roundCount={rounds.length} />
    </div>
  );
}
