'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
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
import { RADAR_2D_VIEW } from '@/components/replay/radar-centre';
import { ReplayWorkspace } from '@/components/replay/replay-workspace';
import { THREE_D_VIEW } from '@/components/replay/three-centre';
import { TacticalDemoSummary } from '@/components/tactical/tactical-demo-summary';
import { TacticalFilterBar } from '@/components/tactical/tactical-filter-bar';
import { ReplayFilters } from '@/components/replay/replay-filters';
import { TacticalTendenciesPanel } from '@/components/tactical/tactical-tendencies';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
import { Segmented } from '@/components/ui/segmented';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

/** The radar first; the 3D view next to it. */
const CENTRE_VIEWS = [RADAR_2D_VIEW, THREE_D_VIEW] as const;

/** Between the top bar and the replay once the tab list sits under the bar: the tab list, its gap, the summary line and its gap. */
/** Until the first measure: the tab list and the one-line summary over the replay. */
const REPLAY_ABOVE: CSSProperties & { '--replay-above': string; '--replay-tabs': string } = { '--replay-above': '6.25rem', '--replay-tabs': '3.5rem' };

/**
 * Keeps `--replay-above` (everything between the top bar and the replay) and
 * `--replay-tabs` (the tab list, the scroll margin) measured, so the replay
 * fills the window exactly whatever the summary or the tab list wrap to.
 */
function measureReplayAbove(root: HTMLElement): () => void {
  const tablist = root.closest('[data-slot="tabs"]')?.querySelector('[role="tablist"]') ?? null;
  const measure = (): void => {
    const workspace = root.querySelector('[data-replay-workspace]');
    if (workspace === null) return;
    const top = root.getBoundingClientRect().top;
    const tabs = tablist === null ? 0 : Math.max(0, top - tablist.getBoundingClientRect().top);
    const summary = Math.max(0, workspace.getBoundingClientRect().top - top);
    root.style.setProperty('--replay-tabs', `${tabs}px`);
    root.style.setProperty('--replay-above', `${tabs + summary}px`);
  };
  measure();
  const observer = new ResizeObserver(measure);
  observer.observe(root);
  if (root.firstElementChild !== null) observer.observe(root.firstElementChild);
  if (tablist !== null) observer.observe(tablist);
  return () => observer.disconnect();
}

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
  const rootRef = useRef<HTMLDivElement>(null);
  const ready = loaded !== null;

  // The replay is sized to the window under the tab list, so the tab list goes under the top bar once.
  useEffect(() => {
    const root = rootRef.current;
    if (!ready || mode !== 'replay' || root === null) return;
    const stop = measureReplayAbove(root);
    root.scrollIntoView({ block: 'start' });
    return stop;
  }, [ready, mode]);

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
  const modes = sourceKind === 'library' ? <Segmented label="Cómo leer las rondas" value={mode} options={MODES} onChange={setMode} /> : undefined;

  return (
    <div ref={rootRef} style={REPLAY_ABOVE} className="flex scroll-mt-[calc(var(--shell-strip-height)+var(--replay-tabs))] flex-col gap-2">
      <TacticalDemoSummary doc={loaded.doc} generatedAt={generatedAt} actions={modes} />

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
          filters={<ReplayFilters doc={loaded.doc} filter={filter} onChange={applyFilter} matched={rounds.length} total={loaded.doc.rounds.length} />}
          centreViews={CENTRE_VIEWS}
        />
      )}

      <div className="mt-6">
        <TacticalTendenciesPanel tendencies={tendencies} error={tendenciesError} roundCount={rounds.length} />
      </div>
    </div>
  );
}
