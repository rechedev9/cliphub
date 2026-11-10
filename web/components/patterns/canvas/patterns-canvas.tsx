'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { Plus } from 'lucide-react';
import type { LibraryMatch } from '@/lib/api/library';
import { lineupSteamIds } from '@/lib/analysis/lineup';
import { PATTERN_MODES, PATTERN_MODE_LABEL, patternModeFromQuery, writePatternMode, type PatternMode } from '@/lib/analysis/pattern-canvas-mode';
import { inScopePeriod } from '@/lib/analysis/scope';
import { TEAM_REPLAY_FIELDS, TEAM_REPLAY_LIMITS, TEAM_REPLAY_STACK_FIELDS } from '@/lib/analysis/team-replay-fields';
import { teamScopeEntries } from '@/lib/analysis/team-scope';
import { teamHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { rivalOf, teamLabel } from '@/lib/library/partidas';
import { teamFocusFromQuery, writeTeamFocus } from '@/lib/replay/player-focus';
import { tramoFromQuery, writeTramo, type Tramo } from '@/lib/replay/tramo';
import { useContentStable } from '@/hooks/use-content-stable';
import { useTeamList, useTeamTactics, type TeamAnalysis } from '@/hooks/use-team-analysis';
import { useTeamMapCounts } from '@/hooks/use-team-map-counts';
import { useTeamScopeControl } from '@/hooks/use-team-scope-control';
import { TeamScopeFilter } from '@/components/analysis/team-scope-filter';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { useFinderSlots, type FinderCount, type FinderFrame } from '@/components/patterns/finder-frame';
import { PatternFinder } from '@/components/patterns/pattern-finder';
import { PlayerFinder } from '@/components/patterns/player-finder';
import { TeamGrenades } from '@/components/patterns/team-grenades';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { CanvasPanel, type PanelTone } from './canvas-panel';

type Tab = 'rondas' | 'tacticas';

const PANEL_TONE: Record<PatternMode, PanelTone> = { granadas: 'dark', jugadores: 'mint', patron: 'blue' };
const ADD_LABEL: Record<PatternMode, string> = { granadas: 'Granada', jugadores: 'Jugador', patron: 'Patrón' };
const TABS: readonly { id: Tab; label: string }[] = [
  { id: 'rondas', label: 'Rondas' },
  { id: 'tacticas', label: 'Tácticas' },
];

/**
 * The Buscador de patrones of one team: a full-bleed tool canvas with the scope stack and the finder panels
 * at the left, the map in the centre, the rounds and the saved tactics at the right and the replay with
 * its time window along the bottom. Each panel hosts one of the Replay's finders.
 */
export function PatternsCanvas({ analysis, onTeam }: { analysis: TeamAnalysis; onTeam: (teamId: string) => void }): ReactNode {
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
  const [tab, setTab] = useState<Tab>('rondas');
  const [counts, setCounts] = useState<Partial<Record<PatternMode, FinderCount | null>>>({});
  const [reported, setReported] = useState<ScopeCounts | null>(null);
  const [now] = useState(() => Date.now());
  const { slots, refs } = useFinderSlots();

  const replaceQuery = useCallback((query: string) => router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false }), [pathname, router]);
  const lineupKey = lineupSteamIds(scope.lineup, lineups).join(',');
  const lineup = useMemo(() => (lineupKey === '' ? [] : lineupKey.split(',')), [lineupKey]);
  // The URL also keeps the window and the players, so the scope is rebuilt on every drag: only its content may refetch.
  const criteria = useContentStable(useMemo(() => teamScopeEntries(scope, TEAM_REPLAY_FIELDS), [scope]));
  const onMapChange = useCallback((map: string) => changeScope({ map }), [changeScope]);
  const fields = useMemo(() => (mode === 'patron' ? TEAM_REPLAY_STACK_FIELDS.filter((field) => field !== 'side') : TEAM_REPLAY_STACK_FIELDS), [mode]);
  const mapCounts = useTeamMapCounts({ teamId: team.id, scope, fields, lineup, enabled: true });

  const tramoValue = useMemo(() => tramoFromQuery(search), [search]);
  const tramo = useMemo(
    () => ({ tramo: tramoValue, onChange: (next: Tramo) => replaceQuery(writeTramo(search.toString(), next)) }),
    [replaceQuery, search, tramoValue],
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
    <div
      data-replay-workspace=""
      className="grid min-h-0 gap-2 lg:h-[calc(100dvh-var(--shell-strip-height)-1.5rem)] lg:grid-cols-[19rem_minmax(0,1fr)_23rem] lg:grid-rows-[minmax(0,1fr)_auto]"
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
      </aside>

      <section aria-label="Mapa" className="min-h-[24rem] min-w-0 lg:overflow-auto">
        {finder}
      </section>

      <aside aria-label="Rondas y tácticas" className="flex min-h-0 flex-col rounded-md border border-border-strong bg-surface-2">
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
  );
}

