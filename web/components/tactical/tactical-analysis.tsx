'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { FileWarning, TriangleAlert } from 'lucide-react';
import { fetchTacticalDocument, isPositionsMissingError } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalSource } from '@/lib/api/tactical';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import type { LibraryMatch } from '@/lib/api/library';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { matchTitle } from '@/lib/library/partidas';
import { DEFAULT_REPLAY_FORM, REPLAY_FORMS, replayFormFromQuery, writeReplayForm } from '@/lib/replay/forms';
import type { ReplayForm } from '@/lib/replay/forms';
import { tramoFromQuery, writeTramo } from '@/lib/replay/tramo';
import { heatChoiceFromQuery, writeHeatChoice } from '@/lib/replay/heat-choice';
import type { HeatChoice } from '@/lib/replay/heat-choice';
import type { Tramo } from '@/lib/replay/tramo';
import { filterTacticalRounds, tacticalFilterFromQuery, tacticalFilterToQuery, withoutOrphanOutcome } from '@/lib/tactical-filter';
import { TACTICAL_EVIDENCE_QUERY } from '@/lib/library/rounds';
import { TACTICAL_FILTER_PARAMS } from '@/lib/api/tactical';
import { POSITIONS_MISSING_MESSAGE, analysisErrorMessage, loadAnalysis } from '@/lib/replay/loaded-analysis';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { ReplayWorkspace } from '@/components/replay/replay-workspace';
import { TacticalDemoSummary } from '@/components/tactical/tactical-demo-summary';
import { MatchGrenades } from '@/components/replay/match-grenades';
import { ReplayPlayers } from '@/components/replay/replay-players';
import { focusFromQuery, writeFocus } from '@/lib/replay/player-focus';
import { ReplayFormTabs, ReplayScopeBar } from '@/components/replay/replay-frame';
import { TacticalTendenciesPanel } from '@/components/tactical/tactical-tendencies';
import { useTeamTendencies } from '@/components/tactical/use-team-tendencies';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
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

/** A Crear job has no library match behind it, so only its rounds replay. */
const JOB_FORMS: readonly ReplayForm[] = [DEFAULT_REPLAY_FORM];

/** One match's analysis; `analysis` is null when its positions file is missing and only the document reads. */
type Loaded = { key: string; analysis: LoadedAnalysis | null; doc: TacticalDocument };

function positiveIntParam(raw: string | null): number | null {
  if (raw === null || !/^[0-9]{1,9}$/.test(raw)) return null;
  const value = Number(raw);
  return value > 0 ? value : null;
}

/**
 * The Replay of one analysed demo: its scope, the four forms that read the
 * rounds the scope kept, and their tendencies. The scope and the form live in
 * the URL; `?round=` and `?tick=` open it on the round and moment a number
 * elsewhere links to.
 */
export function TacticalAnalysis({
  source,
  generatedAt,
  match,
}: {
  source: TacticalSource;
  generatedAt: string;
  /** The library partida, which the Granadas form reads its teams from. */
  match?: LibraryMatch;
}): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const forms = source.kind === 'library' ? REPLAY_FORMS : JOB_FORMS;
  const asked = replayFormFromQuery(searchParams);
  const form = forms.includes(asked) ? asked : DEFAULT_REPLAY_FORM;

  const { kind: sourceKind, id: sourceId } = source;
  const sourceKey = `${sourceKind}:${sourceId}`;
  // Every piece of state is tagged with the match it belongs to, so a new match never shows the last one's.
  const [state, setState] = useState<Loaded | null>(null);
  const [failure, setFailure] = useState<{ key: string; message: string } | null>(null);
  const [picked, setPicked] = useState<{ key: string; round: number | null } | null>(null);
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
    if (!ready || form !== DEFAULT_REPLAY_FORM || root === null) return;
    const stop = measureReplayAbove(root);
    root.scrollIntoView({ block: 'start' });
    return stop;
  }, [ready, form]);

  // A pasted link may carry a result with no team or side, which selects nothing; it is read without it.
  const filter = useMemo<TacticalFilter>(() => withoutOrphanOutcome(tacticalFilterFromQuery(searchParams)), [searchParams]);
  const filterQuery = tacticalFilterToQuery(filter);
  const linkedRound = positiveIntParam(searchParams.get(TACTICAL_EVIDENCE_QUERY.round));
  const linkedTick = positiveIntParam(searchParams.get(TACTICAL_EVIDENCE_QUERY.tick));

  useEffect(() => {
    if (linkedRound !== null) setSelectedRound(linkedRound);
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

  const tendencies = useTeamTendencies({ source, filter, doc, enabled: sourceKind !== 'library' });

  const rounds = useMemo(() => (doc === null ? [] : filterTacticalRounds(doc.teams, filter, doc.rounds)), [doc, filter]);
  // A filter that hides the open round moves to the first one it kept.
  const found = rounds.findIndex((round) => round.number === selectedRound);
  const index = found < 0 ? 0 : found;
  const shownRound = rounds[index]?.number ?? null;

  const applyFilter = useCallback(
    (next: TacticalFilter) => {
      // The round on screen stays open when the new filter keeps it, "Limpiar" included.
      if (shownRound !== null) setSelectedRound(shownRound);
      // The filter owns its own keys; the tab, the form and anything else in the URL stay.
      const kept = new URLSearchParams(searchParams.toString());
      for (const key of Object.values(TACTICAL_FILTER_PARAMS)) kept.delete(key);
      for (const key of Object.values(TACTICAL_EVIDENCE_QUERY)) kept.delete(key);
      const query = [kept.toString(), tacticalFilterToQuery(next)].filter((part) => part !== '').join('&');
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, searchParams, setSelectedRound, shownRound],
  );
  const tramoValue = useMemo(() => tramoFromQuery(searchParams), [searchParams]);
  const setTramo = useCallback(
    (next: Tramo) => {
      const query = writeTramo(searchParams.toString(), next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, searchParams],
  );
  const tramo = useMemo(() => ({ tramo: tramoValue, onChange: setTramo }), [tramoValue, setTramo]);
  const heatValue = useMemo(() => heatChoiceFromQuery(searchParams), [searchParams]);
  const setHeat = useCallback(
    (next: HeatChoice) => {
      const query = writeHeatChoice(searchParams.toString(), next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, searchParams],
  );
  const heat = useMemo(() => ({ choice: heatValue, onChange: setHeat }), [heatValue, setHeat]);
  // The players in the foreground: one choice for the four forms, kept in the URL.
  const focus = useMemo(() => focusFromQuery(searchParams, doc?.players.map((player) => player.slot) ?? []), [doc, searchParams]);
  const setFocus = useCallback(
    (next: number[]) => {
      const query = writeFocus(searchParams.toString(), next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, searchParams],
  );
  const players = useMemo(() => ({ focus, onChange: setFocus }), [focus, setFocus]);
  const setForm = useCallback(
    (next: ReplayForm) => {
      const query = writeReplayForm(searchParams.toString(), next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, searchParams],
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
  // The strip names the match with its date, so a round number always says whose it is.
  const groupLabel = useCallback(
    () => (match === undefined ? null : `${prettyMapName(match.map)} · ${matchTitle(match)} · ${formatShortDate(match.played_at)}`),
    [match],
  );
  const matchDate = useCallback(() => (match === undefined ? null : formatShortDate(match.played_at)), [match]);
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

  // The forms that count rounds read one team when the scope has one, so a rival in focus does not count there.
  const scopeSlots = doc.teams.find((team) => team.key === filter.team_key)?.slots;
  const counted = scopeSlots === undefined ? focus : focus.filter((slot) => scopeSlots.includes(slot));
  const spatialScope = { kind: 'match', matchId: sourceId, filter: writeFocus(filterQuery, counted) } as const;
  const playerList = <ReplayPlayers doc={doc} teamKey={filter.team_key} focus={counted} onChange={setFocus} />;
  let rounded: ReactNode;
  if (loaded === null) {
    rounded = <StudioEmptyState icon={FileWarning} title="No se puede reproducir esta demo" description={POSITIONS_MISSING_MESSAGE} compact />;
  } else if (form === 'calor') {
    rounded = (
      <>
        {playerList}
        <SpatialWorkspace scope={spatialScope} view="heatmap" hideViewSwitch tramo={tramo} heat={heat} />
      </>
    );
  } else if (form === 'recorridos') {
    rounded = (
      <>
        {playerList}
        <SpatialWorkspace scope={spatialScope} view="overlay" hideViewSwitch tramo={tramo} />
      </>
    );
  } else if (form === 'granadas' && match !== undefined) {
    rounded = (
      <>
        {playerList}
        <MatchGrenades match={match} doc={doc} filter={filter} rounds={rounds} onPickTeam={(team_key) => applyFilter({ ...filter, team_key })} tramo={tramo} focus={counted} />
      </>
    );
  } else {
    rounded = (
      <ReplayWorkspace
        source={sourceKind}
        entries={entries}
        index={index}
        onIndexChange={(next) => setSelectedRound(rounds[next]?.number ?? null)}
        seed={seed}
        players={players}
        tramo={tramo}
        groupLabel={groupLabel}
        matchDate={matchDate}
      />
    );
  }

  return (
    <div ref={rootRef} style={REPLAY_ABOVE} className="flex scroll-mt-[calc(var(--shell-strip-height)+var(--replay-tabs))] flex-col gap-2">
      <TacticalDemoSummary doc={doc} generatedAt={generatedAt} />
      <ReplayScopeBar doc={doc} filter={filter} onChange={applyFilter} matched={rounds.length} total={doc.rounds.length} />
      {forms.length > 1 ? <ReplayFormTabs forms={forms} value={form} onChange={setForm} /> : null}

      {rounded}

      {/* A library match keeps its tendencies in its Métricas tab; a Crear job has no other place for them. */}
      {sourceKind === 'library' ? null : (
        <div className="mt-6">
          <TacticalTendenciesPanel {...tendencies} roundCount={rounds.length} />
        </div>
      )}
    </div>
  );
}
