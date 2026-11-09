'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { FileWarning, TriangleAlert } from 'lucide-react';
import { fetchTacticalDocument, isPositionsMissingError } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalSource } from '@/lib/api/tactical';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { filterTacticalRounds, tacticalFilterFromQuery, tacticalFilterToQuery, withoutOrphanOutcome } from '@/lib/tactical-filter';
import { TACTICAL_EVIDENCE_QUERY } from '@/lib/library/rounds';
import { POSITIONS_MISSING_MESSAGE, analysisErrorMessage, loadAnalysis } from '@/lib/replay/loaded-analysis';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { ReplayWorkspace } from '@/components/replay/replay-workspace';
import { TacticalDemoSummary } from '@/components/tactical/tactical-demo-summary';
import { TacticalFilterBar } from '@/components/tactical/tactical-filter-bar';
import { ReplayFilters } from '@/components/replay/replay-filters';
import { TacticalTendenciesPanel } from '@/components/tactical/tactical-tendencies';
import { useTeamTendencies } from '@/components/tactical/use-team-tendencies';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
import { Segmented } from '@/components/ui/segmented';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

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

/** One match's analysis; `analysis` is null when its positions file is missing and only the document reads. */
type Loaded = { key: string; analysis: LoadedAnalysis | null; doc: TacticalDocument };

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

  const { kind: sourceKind, id: sourceId } = source;
  const sourceKey = `${sourceKind}:${sourceId}`;
  // Every piece of state is tagged with the match it belongs to, so a new match never shows the last one's.
  const [state, setState] = useState<Loaded | null>(null);
  const [failure, setFailure] = useState<{ key: string; message: string } | null>(null);
  const [picked, setPicked] = useState<{ key: string; round: number | null } | null>(null);
  const [mode, setMode] = useState<AnalysisMode>('replay');
  const rootRef = useRef<HTMLDivElement>(null);
  const current = state?.key === sourceKey ? state : null;
  const loaded = current?.analysis ?? null;
  const doc = current?.doc ?? null;
  const loadError = failure?.key === sourceKey ? failure.message : null;
  const selectedRound = picked?.key === sourceKey ? picked.round : null;
  const setSelectedRound = useCallback((round: number | null) => setPicked({ key: sourceKey, round }), [sourceKey]);
  const ready = loaded !== null;

  // The replay is sized to the window under the tab list, so the tab list goes under the top bar once.
  useEffect(() => {
    const root = rootRef.current;
    if (!ready || mode !== 'replay' || root === null) return;
    const stop = measureReplayAbove(root);
    root.scrollIntoView({ block: 'start' });
    return stop;
  }, [ready, mode]);

  // A pasted link may carry a result with no team or side, which selects nothing; it is read without it.
  const filter = useMemo<TacticalFilter>(() => withoutOrphanOutcome(tacticalFilterFromQuery(searchParams)), [searchParams]);
  const filterQuery = tacticalFilterToQuery(filter);
  const linkedRound = positiveIntParam(searchParams.get(TACTICAL_EVIDENCE_QUERY.round));
  const linkedTick = positiveIntParam(searchParams.get(TACTICAL_EVIDENCE_QUERY.tick));

  useEffect(() => {
    if (linkedRound === null) return;
    setSelectedRound(linkedRound);
    setMode('replay');
  }, [linkedRound, linkedTick, setSelectedRound]);

  useEffect(() => {
    let active = true;
    const key = `${sourceKind}:${sourceId}`;
    const from: TacticalSource = { kind: sourceKind, id: sourceId };
    const fail = (error: unknown): void => {
      if (active) setFailure({ key, message: analysisErrorMessage(error) });
    };
    loadAnalysis(from)
      .then((next) => {
        if (active) setState({ key, analysis: next, doc: next.doc });
      })
      .catch((error: unknown) => {
        if (!isPositionsMissingError(error)) return fail(error);
        // Without positions the replay cannot play, but the filters and the tendencies still read.
        fetchTacticalDocument(from)
          .then((only) => {
            if (active) setState({ key, analysis: null, doc: only });
          })
          .catch(fail);
      });
    return () => {
      active = false;
    };
  }, [sourceKind, sourceId]);

  const tendencies = useTeamTendencies({ source, filter, doc });

  const rounds = useMemo(() => (doc === null ? [] : filterTacticalRounds(doc.teams, filter, doc.rounds)), [doc, filter]);
  // A filter that hides the open round moves to the first one it kept.
  const found = rounds.findIndex((round) => round.number === selectedRound);
  const index = found < 0 ? 0 : found;
  const shownRound = rounds[index]?.number ?? null;

  const applyFilter = useCallback(
    (next: TacticalFilter) => {
      // The round on screen stays open when the new filter keeps it, "Limpiar" included.
      if (shownRound !== null) setSelectedRound(shownRound);
      const query = tacticalFilterToQuery(next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, setSelectedRound, shownRound],
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

  if (doc === null) return <TacticalWorkspaceSkeleton />;

  const filterBar = <TacticalFilterBar doc={doc} filter={filter} onChange={applyFilter} matched={rounds.length} total={doc.rounds.length} />;
  const modes = sourceKind === 'library' ? <Segmented label="Cómo leer las rondas" value={mode} options={MODES} onChange={setMode} /> : undefined;

  let rounded: ReactNode;
  if (loaded === null) {
    rounded = (
      <div className="grid gap-5 lg:grid-cols-[230px_minmax(0,1fr)]">
        {filterBar}
        <StudioEmptyState icon={FileWarning} title="No se puede reproducir esta demo" description={POSITIONS_MISSING_MESSAGE} compact />
      </div>
    );
  } else if (mode === 'heatmap' && sourceKind === 'library') {
    rounded = (
      <div className="grid gap-5 lg:grid-cols-[230px_minmax(0,1fr)]">
        {filterBar}
        <SpatialWorkspace scope={{ kind: 'match', matchId: sourceId, filter: filterQuery }} view="heatmap" hideViewSwitch />
      </div>
    );
  } else {
    rounded = (
      <ReplayWorkspace
        source={sourceKind}
        entries={entries}
        index={index}
        onIndexChange={(next) => setSelectedRound(rounds[next]?.number ?? null)}
        seed={seed}
        filters={<ReplayFilters doc={doc} filter={filter} onChange={applyFilter} matched={rounds.length} total={doc.rounds.length} />}
      />
    );
  }

  return (
    <div ref={rootRef} style={REPLAY_ABOVE} className="flex scroll-mt-[calc(var(--shell-strip-height)+var(--replay-tabs))] flex-col gap-2">
      <TacticalDemoSummary doc={doc} generatedAt={generatedAt} actions={modes} />

      {rounded}

      <div className="mt-6">
        <TacticalTendenciesPanel {...tendencies} roundCount={rounds.length} />
      </div>
    </div>
  );
}
