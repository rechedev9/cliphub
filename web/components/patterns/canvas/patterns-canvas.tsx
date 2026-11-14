'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { Plus } from 'lucide-react';
import type { LibraryMatch } from '@/lib/api/library';
import { PATTERN_MODES, PATTERN_MODE_LABEL, patternModeFromQuery, writePatternMode, type PatternMode } from '@/lib/analysis/pattern-canvas-mode';
import { inScopePeriod } from '@/lib/analysis/scope';
import { TEAM_REPLAY_LIMITS, TEAM_REPLAY_STACK_FIELDS, teamReplayEntries } from '@/lib/analysis/team-replay-fields';
import { teamHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { rivalOf, teamLabel } from '@/lib/library/partidas';
import { teamFocusFromQuery, writeTeamFocus } from '@/lib/replay/player-focus';
import { useContentStable } from '@/hooks/use-content-stable';
import { useTeamList, useTeamTactics, type TeamAnalysis } from '@/hooks/use-team-analysis';
import { useMapLineup } from '@/hooks/use-map-lineup';
import { useTeamScopeControl } from '@/hooks/use-team-scope-control';
import { SampleLineupProvider } from '@/components/analysis/empty-sample';
import { TeamScopeFilter } from '@/components/analysis/team-scope-filter';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { useFinderSlots, type FinderCount, type FinderFrame } from '@/components/patterns/finder-frame';
import { PatternFinder } from '@/components/patterns/pattern-finder';
import { PlayerFinder } from '@/components/patterns/player-finder';
import { TeamDocuments } from '@/components/patterns/team-documents';
import { TeamGrenades } from '@/components/patterns/team-grenades';
import { ImportDemosButton } from '@/components/matches/import-demos-button';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { CanvasPanel, type PanelTone } from './canvas-panel';
import { useUrlTramo } from '@/hooks/use-url-tramo';

type Tab = 'rondas' | 'tacticas' | 'documentos';

const PANEL_TONE: Record<PatternMode, PanelTone> = { granadas: 'dark', jugadores: 'mint', patron: 'blue' };
const ADD_LABEL: Record<PatternMode, string> = { granadas: 'Granada', jugadores: 'Jugador', patron: 'Patrón' };
const TABS: readonly { id: Tab; label: string }[] = [
  { id: 'rondas', label: 'Rondas' },
  { id: 'tacticas', label: 'Tácticas' },
  { id: 'documentos', label: 'Documentos' },
];
/** Query keys of the right tab and of the document in the viewer, so a link opens a report directly. */
const TAB_QUERY = 'panel';
const DOCUMENT_QUERY = 'documento';

/**
 * The Buscador de patrones of one team: a full-bleed tool canvas with the scope stack and the finder panels
 * at the left, the map in the centre, the rounds and the saved tactics at the right and the replay with
 * its time window along the bottom. Each panel hosts one of the Replay's finders.
 */
export function PatternsCanvas({ analysis, onTeam, onImported }: { analysis: TeamAnalysis; onTeam: (teamId: string) => void; onImported: () => void }): ReactNode {
  const { team, matches, lineups } = analysis;
  const teams = useTeamList();
  const tactics = useTeamTactics(team.id);
  const control = useTeamScopeControl({ teamId: team.id, tactics, matches, defaultMap: true });
  const { scope, changeScope, setScope } = control;
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const mode = patternModeFromQuery(search);
  const [shown, setShown] = useState<readonly PatternMode[]>(['granadas', 'jugadores']);
  const [tab, setTab] = useState<Tab>(() => TABS.find((entry) => entry.id === search.get(TAB_QUERY))?.id ?? (search.has(DOCUMENT_QUERY) ? 'documentos' : 'rondas'));
  const [counts, setCounts] = useState<Partial<Record<PatternMode, FinderCount | null>>>({});
  const [reported, setReported] = useState<ScopeCounts | null>(null);
  const [now] = useState(() => Date.now());
  const { slots, refs } = useFinderSlots();

  const replaceQuery = useCallback((query: string) => router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false }), [pathname, router]);
  // The URL also keeps the window and the players, so the scope is rebuilt on every drag: only its content may refetch.
  const criteria = useContentStable(useMemo(() => teamReplayEntries(scope), [scope]));
  const onMapChange = useCallback((map: string) => changeScope({ map }), [changeScope]);
  const fields = useMemo(() => (mode === 'patron' ? TEAM_REPLAY_STACK_FIELDS.filter((field) => field !== 'side') : TEAM_REPLAY_STACK_FIELDS), [mode]);
  const { lineup, mapCounts, fallback: lineupFallback } = useMapLineup({ teamId: team.id, scope, fields, lineups });
  // A stand-in in the latest match can leave a map with no rounds: the empty finders then offer every player.
  const showEveryone = useCallback(() => changeScope({ lineup: { kind: 'all' } }), [changeScope]);

  const tramo = useUrlTramo();
  const openDocument = search.get(DOCUMENT_QUERY);
  const setOpenDocument = useCallback(
    (id: string | null) => {
      const params = new URLSearchParams(search.toString());
      if (id === null) params.delete(DOCUMENT_QUERY);
      else params.set(DOCUMENT_QUERY, id);
      replaceQuery(params.toString());
    },
    [replaceQuery, search],
  );
  const picked = useMemo(() => teamFocusFromQuery(search), [search]);
  const pick = useMemo(() => ({ picked, onChange: (next: readonly string[]) => replaceQuery(writeTeamFocus(search.toString(), next)) }), [picked, replaceQuery, search]);

  const matchLabel = useCallback(
    (matchId: string): string => {
      const match = matches.find((entry) => entry.id === matchId);
      return match === undefined ? 'Partida' : `vs ${teamLabel(rivalOf(match, team.id))} ${formatShortDate(match.played_at)}`;
    },
    [matches, team.id],
  );
  const sampleMatches = useMemo<LibraryMatch[]>(
    () =>
      matches.filter(
        (match) =>
          (scope.map === '' || match.map === scope.map) &&
          inScopePeriod(match.played_at, scope.period, now) &&
          (scope.opponent === '' || rivalOf(match, team.id)?.team_id === scope.opponent),
      ),
    [matches, scope.map, scope.period, scope.opponent, now, team.id],
  );

  const onCount = useCallback(
    (count: FinderCount | null) =>
      setCounts((previous) => {
        const known = previous[mode];
        if (known === count || (known !== undefined && known !== null && count !== null && known.matched === count.matched && known.total === count.total)) return previous;
        return { ...previous, [mode]: count };
      }),
    [mode],
  );
  const frame = useMemo<FinderFrame>(() => ({ slots, onCount }), [slots, onCount]);

  const visible = PATTERN_MODES.filter((entry) => shown.includes(entry) || entry === mode);
  const hidden = PATTERN_MODES.filter((entry) => !visible.includes(entry));
  const open = (next: PatternMode): void => {
    setShown((current) => (current.includes(next) ? current : [...current, next]));
    replaceQuery(writePatternMode(search.toString(), next));
  };
  const remove = (gone: PatternMode): void => {
    const left = visible.filter((entry) => entry !== gone);
    setShown(left);
    if (gone === mode && left[0] !== undefined) replaceQuery(writePatternMode(search.toString(), left[0]));
  };

  const moreHeatHref = useMemo(() => {
    const params = new URLSearchParams(search.toString());
    for (const key of ['modo', 'forma', 'tab']) params.delete(key);
    params.set('tab', 'replay');
    params.set('forma', 'calor');
    return `${teamHref(team.id)}?${params.toString()}`;
  }, [search, team.id]);

  let finder: ReactNode;
  if (mode === 'granadas') {
    finder = <TeamGrenades teamId={team.id} scope={scope} lineup={lineup} criteria={criteria} matches={matches} onMapChange={onMapChange} onCounts={setReported} frame={frame} tramo={tramo} />;
  } else if (mode === 'jugadores') {
    finder = (
      <PlayerFinder
        frame={frame}
        teamId={team.id}
        scope={scope}
        lineup={lineup}
        criteria={criteria}
        roster={team.roster ?? []}
        tramo={tramo}
        pick={pick}
        matchLabel={matchLabel}
        moreHeatHref={moreHeatHref}
        onCounts={setReported}
      />
    );
  } else {
    finder = (
      <PatternFinder
        frame={frame}
        teamId={team.id}
        scope={scope}
        lineup={lineup}
        criteria={criteria}
        matches={matches}
        roster={team.roster ?? []}
        onMapChange={onMapChange}
        onScopeChange={setScope}
        onCounts={setReported}
      />
    );
  }

  return (
    <SampleLineupProvider showEveryone={lineup.length > 0 ? showEveryone : null}>
      <div
        data-replay-workspace=""
        className="grid min-h-0 gap-2 lg:h-[calc(100dvh-var(--shell-strip-height)-1.5rem)] lg:grid-cols-[18rem_minmax(0,1fr)_20rem] lg:grid-rows-[minmax(0,1fr)_auto]"
      >
        <aside aria-label="Filtros y paneles" className="flex min-h-0 flex-col gap-2 lg:overflow-y-auto lg:pr-1">
          <TeamScopeFilter
            form="stack"
            control={control}
            analysis={analysis}
            teams={teams}
            tactics={tactics}
            fields={fields}
            mapRequired
            limits={TEAM_REPLAY_LIMITS}
            mapCounts={mapCounts}
        lineupFallback={lineupFallback}
            counts={scope.map === '' ? undefined : reported}
            sampleMatches={sampleMatches}
            onTeam={onTeam}
          />
          {visible.map((entry) => (
            <CanvasPanel
              key={entry}
              title={PATTERN_MODE_LABEL[entry]}
              tone={PANEL_TONE[entry]}
              count={counts[entry]}
              open={entry === mode}
              removable={visible.length > 1}
              onOpen={() => open(entry)}
              onRemove={() => remove(entry)}
            >
              <div ref={refs.panel} />
            </CanvasPanel>
          ))}
          {hidden.length === 0 ? null : (
            <div className="flex flex-wrap gap-1.5">
              {hidden.map((entry) => (
                <Button key={entry} type="button" variant="outline" size="sm" onClick={() => open(entry)}>
                  <Plus aria-hidden />
                  {ADD_LABEL[entry]}
                </Button>
              ))}
            </div>
          )}
          <div className="mt-auto pt-1">
            <ImportDemosButton onImported={onImported} />
          </div>
        </aside>

        <section aria-label="Mapa" className="min-h-[24rem] min-w-0 lg:overflow-auto">
          {finder}
        </section>

        <aside aria-label="Rondas, tácticas y documentos" className="flex min-h-0 flex-col rounded-md border border-border-strong bg-surface-2">
          <div role="tablist" aria-label="Resultados" className="flex gap-5 border-b border-border px-3">
            {TABS.map((entry) => (
              <button
                key={entry.id}
                type="button"
                role="tab"
                id={`patrones-tab-${entry.id}`}
                aria-selected={tab === entry.id}
                aria-controls={`patrones-panel-${entry.id}`}
                onClick={() => setTab(entry.id)}
                className={cn(
                  'h-9 border-b-2 text-body-sm font-medium outline-none focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
                  tab === entry.id ? 'border-primary text-fg-1' : 'border-transparent text-fg-3 hover:text-fg-1',
                )}
              >
                {entry.label}
              </button>
            ))}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-3">
            <div role="tabpanel" id="patrones-panel-rondas" aria-labelledby="patrones-tab-rondas" hidden={tab !== 'rondas'}>
              <div ref={refs.rounds} />
            </div>
            <div role="tabpanel" id="patrones-panel-documentos" aria-labelledby="patrones-tab-documentos" hidden={tab !== 'documentos'}>
              {tab === 'documentos' || openDocument !== null ? <TeamDocuments key={team.id} teamId={team.id} teamName={team.name} openId={openDocument} onOpen={setOpenDocument} /> : null}
            </div>
            <div role="tabpanel" id="patrones-panel-tacticas" aria-labelledby="patrones-tab-tacticas" hidden={tab !== 'tacticas'}>
              <div ref={refs.tactics} />
              {mode === 'patron' ? null : (
                <div className="flex flex-col items-start gap-2 text-body-sm text-fg-2">
                  <p>Las tácticas guardadas son patrones: ábrelas desde el panel Patrón.</p>
                  <Button type="button" variant="outline" size="sm" onClick={() => open('patron')}>
                    Abrir Patrón
                  </Button>
                </div>
              )}
            </div>
          </div>
        </aside>

        <footer aria-label="Reproducción y tramo" className="rounded-md border border-border-strong bg-surface-2 px-3 py-2 lg:col-span-3">
          <div ref={refs.footer} />
        </footer>
      </div>
    </SampleLineupProvider>
  );
}

