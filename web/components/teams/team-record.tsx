'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { ArrowLeft, BookMarked, EllipsisVertical, GitMerge, Pencil, PlugZap, Shield } from 'lucide-react';
import { toast } from 'sonner';
import {
  getLibraryTeam,
  getTeamLineups,
  getTeamProfile,
  isLibraryNotFound,
  isLibraryOffline,
  listLibraryMatches,
  listLibraryPlays,
  listLibraryTeams,
  type LibraryMatch,
  type LibraryTeam,
} from '@/lib/api/library';
import type { TeamLineups, TeamProfile } from '@/lib/api/team-insights';
import { lineupSteamIds } from '@/lib/analysis/lineup';
import { TEAMS_HREF, matchHref, teamHref, teamLibretaHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  OUTCOME_LABEL,
  mapPerformance,
  percentLabel,
  rivalOf,
  teamLabel,
  teamOutcome,
} from '@/lib/library/partidas';
import { inScopePeriod, type AnalysisScope, type ScopeField } from '@/lib/analysis/scope';
import { mapPatch, tacticOptions, unknownTactic, type TacticOption } from '@/lib/analysis/scope-tactics';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import { lastResults, writeTab } from '@/lib/analysis/context-bar';
import { COMPARE_PARAM, parseCompareTeams, writeCompareTeams } from '@/lib/analysis/side-stats';
import { SampleLineupProvider } from '@/components/analysis/empty-sample';
import { MapLabel } from '@/components/analysis/map-thumb';
import { CompareGroupPicker } from '@/components/analysis/compare-group-picker';
import { ScopeBar, type ScopeCounts, type ScopeEntity, type ScopeMatch } from '@/components/analysis/scope-bar';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { SlashTitle } from '@/components/records/document-page';
import { RecordTabs, type RecordTab } from '@/components/records/record-tabs';
import { useTableView } from '@/components/records/use-table-view';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';
import { useAnalysisScope } from '@/hooks/use-analysis-scope';
import { useRecordTab } from '@/hooks/use-record-tab';
import { useTeamMapCounts } from '@/hooks/use-team-map-counts';
import { useReferences } from '@/hooks/use-references';
import { cn } from '@/lib/utils';
import { EditTeamDialog, MergeTeamDialog } from './team-dialogs';
import { TeamBombsites } from './team-bombsites';
import { TeamPositions } from './team-positions';
import { TeamMapHistory } from './team-map-history';
import { TeamRoster } from './team-roster';
import { TeamPrepare } from './team-prepare';
import { TEAM_REPLAY_FORMS, TeamReplay, type TeamReplayForm } from './team-replay';
import { TEAM_REPLAY_FIELDS, TEAM_REPLAY_LIMITS } from '@/lib/analysis/team-replay-fields';
import { REPLAY_FORM_QUERY } from '@/lib/replay/forms';
import { TeamPlaybook } from '@/components/playbook/team-playbook';
import { TeamSeries } from './team-series';
import { TeamVeto } from './team-veto';
import { TeamTrend } from './team-trend';
import { TeamSideStats } from './team-side-stats';
import { TeamPlayerPerformance } from './team-player-performance';

/** `lineups` and `profile` are null when they could not be read; the views then count every round. */
type Loaded = { team: LibraryTeam; matches: LibraryMatch[]; lineups: TeamLineups | null; profile: TeamProfile | null };
type LoadState = { kind: 'loading' } | { kind: 'ready'; data: Loaded } | { kind: 'missing' } | { kind: 'offline' };

/** A team's record: roster by date, matches with rival and result, and the record per map. */
export function TeamRecord({ teamId }: { teamId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    Promise.all([
      getLibraryTeam(teamId),
      listLibraryMatches({ team_id: teamId }),
      getTeamLineups(teamId).catch(() => null),
      getTeamProfile(teamId).catch(() => null),
    ])
      .then(([team, matches, lineups, profile]) => {
        if (active) setState({ kind: 'ready', data: { team, matches, lineups, profile } });
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (isLibraryOffline(error)) setState({ kind: 'offline' });
        else if (isLibraryNotFound(error)) setState({ kind: 'missing' });
        else setState({ kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [teamId, attempt]);

  const reload = useCallback(() => setAttempt((n) => n + 1), []);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando equipo" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={Shield}
        title="Equipo no encontrado"
        description="Puede que se haya fusionado con otro equipo o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={TEAMS_HREF}>Ver equipos</Link>
          </Button>
        }
      />
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer el equipo. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <TeamRecordView data={state.data} onChanged={reload} />;
}

const SIDE_STATS_FIELDS: readonly ScopeField[] = ['lineup', 'map', 'side', 'period', 'buy', 'opponentBuy', 'opponent', 'excludeTactics', 'matchTags', 'excludeMatchTags', 'events'];
/** The scope controls each tab reads, in the order the bar shows them. */
const TAB_FIELDS: Readonly<Record<string, readonly ScopeField[]>> = {
  plantilla: ['map', 'reference'],
  partidas: ['map', 'period', 'opponent'],
  mapas: ['map', 'period'],
  estadisticas: SIDE_STATS_FIELDS,
  rendimiento: SIDE_STATS_FIELDS,
  posiciones: ['lineup', 'map', 'side', 'period', 'opponent'],
  veto: ['opponent'],
  sitios: ['lineup', 'map', 'period', 'opponent'],
  preparar: ['lineup', 'map', 'side', 'period', 'opponent'],
  playbook: ['lineup', 'map', 'side', 'period', 'opponent'],
  replay: TEAM_REPLAY_FIELDS,
  'replay-granadas': ['lineup', 'map', ...TEAM_REPLAY_FIELDS.filter((field) => field !== 'lineup')],
  // The pattern carries its own side, so the bar leaves it out.
  'replay-patrones': ['lineup', 'map', ...TEAM_REPLAY_FIELDS.filter((field) => field !== 'lineup' && field !== 'side')],
};
/** Tabs whose own answer carries the sample size; the others count the library matches in scope. */
const REPORTING_TABS = new Set(['mapas', 'estadisticas', 'rendimiento', 'sitios', 'preparar', 'playbook', 'replay']);
/** Tabs whose view reads the matches through the server scope pipeline, the one the map counts follow. */
const COUNTED_TABS = new Set(['estadisticas', 'rendimiento', 'posiciones', 'sitios', 'preparar', 'playbook', 'replay']);
/**
 * EDGE's Team Stats tabs in its order, shown under the scope bar without the record header. Mapas holds
 * the vetoes EDGE's Vetos tab lists; the Veto tab is the simulator, a page of its own in EDGE.
 */
const STATS_TABS = ['estadisticas', 'rendimiento', 'posiciones', 'sitios', 'mapas'];
const TEAM_TABS = [...STATS_TABS, 'plantilla', 'partidas', 'veto', 'preparar', 'playbook', 'replay'];
/** The tab a team opens on, the EDGE Overview; a link without `?tab=` lands here. */
const DEFAULT_TAB = 'estadisticas';
/** Tabs of the statistics pages: their content sits in the centred document column under the full-width header. */
const COLUMN_TABS = new Set(['estadisticas', 'rendimiento', 'plantilla', 'posiciones', 'sitios', 'veto', 'partidas', 'mapas']);
/** Tabs of older links: Scouting is in Preparar, the series are under Partidas, and the radar and the grenades are forms of the Replay. */
const LEGACY_TABS: Readonly<Record<string, string>> = { scouting: 'preparar', series: 'partidas', radar: 'replay', granadas: 'replay' };
const QUERY_TABS = [...TEAM_TABS, ...Object.keys(LEGACY_TABS)];

function TeamRecordView({ data, onChanged }: { data: Loaded; onChanged: () => void }): ReactNode {
  const { team, matches, lineups, profile } = data;
  const [editing, setEditing] = useState(false);
  const [merging, setMerging] = useState(false);
  const [rawTab, setTab] = useRecordTab(QUERY_TABS, DEFAULT_TAB);
  const tab = LEGACY_TABS[rawTab] ?? rawTab;
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const askedForm = search.get(REPLAY_FORM_QUERY) ?? (rawTab === 'granadas' ? 'granadas' : '');
  const replayForm: TeamReplayForm = TEAM_REPLAY_FORMS.find((entry) => entry === askedForm) ?? 'calor';
  // One replace for the tab and the form, so neither write loses the other.
  const openReplay = useCallback(
    (form: TeamReplayForm) => {
      const params = new URLSearchParams(writeTab(search.toString(), 'replay', DEFAULT_TAB));
      params.set(REPLAY_FORM_QUERY, form);
      router.replace(`${pathname}?${params.toString()}`, { scroll: false });
    },
    [pathname, router, search],
  );
  const [reported, setReported] = useState<ScopeCounts | null>(null);
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  // null until the Libreta answers, so a tactic in the URL is not cleared early.
  const [tactics, setTactics] = useState<readonly TacticOption[] | null>(null);
  const [now] = useState(() => Date.now());
  const { scope, setScope, resetScope } = useAnalysisScope({ kind: 'team', id: team.id });
  const references = useReferences(scope.map);
  const proReference = references !== null && references !== 'failed' ? references.pro : null;
  const onCounts = useCallback((counts: ScopeCounts | null) => setReported(counts), []);
  const changeScope = useCallback(
    (patch: Partial<AnalysisScope>) =>
      setScope(patch.map === undefined ? patch : { ...patch, ...mapPatch({ map: patch.map, tactic: patch.tactic ?? scope.tactic, options: tactics ?? [] }) }),
    [scope.tactic, setScope, tactics],
  );
  const onMapChange = useCallback((map: string) => changeScope({ map }), [changeScope]);
  const teamMaps = useMemo(() => [...new Set(matches.map((match) => match.map))].sort(), [matches]);
  // Keyed on the ids, so a view refetches only when the players it counts change.
  const lineupKey = lineupSteamIds(scope.lineup, lineups).join(',');
  const lineup = useMemo(() => (lineupKey === '' ? [] : lineupKey.split(',')), [lineupKey]);
  const showEveryone = useCallback(() => setScope({ lineup: { kind: 'all' } }), [setScope]);
  const compareKey = parseCompareTeams(search.get(COMPARE_PARAM)).join(',');
  // A stale or own id in the URL drops out: the picker lists only the other library teams.
  const compare = useMemo(() => {
    const ids = compareKey === '' ? [] : compareKey.split(',').filter((id) => id !== team.id);
    if (teams === null) return ids;
    const known = new Set(teams.map((entry) => entry.id));
    return ids.filter((id) => known.has(id));
  }, [compareKey, team.id, teams]);
  const setCompare = useCallback(
    (ids: readonly string[]) => router.replace(`${pathname}?${writeCompareTeams(search.toString(), ids)}`, { scroll: false }),
    [pathname, router, search],
  );
  useRouteTitle(team.name);
  useDocumentTitle(`${team.name} · Equipo · ClipHub`);

  const outcomes = matches.map((match) => teamOutcome(match, team.id));
  const wins = outcomes.filter((outcome) => outcome === 'win').length;
  const losses = outcomes.filter((outcome) => outcome === 'loss').length;
  const draws = outcomes.filter((outcome) => outcome === 'draw').length;
  const maps = mapPerformance(matches, team.id);
  const last = matches[0];
  const aliases = team.aliases.filter((alias) => alias.toLowerCase() !== team.name.toLowerCase());
  const subtitle = [profile?.own === true ? 'Mi equipo' : '', aliases.length > 0 ? `También como ${aliases.join(', ')}` : '']
    .filter((part) => part !== '')
    .join(' · ');
  const rank = profile?.hltv_rank ?? 0;

  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (active) setTeams(list.filter((entry) => entry.id !== team.id).sort((a, b) => a.name.localeCompare(b.name, 'es')));
      })
      .catch(() => {
        if (active) setTeams([]);
      });
    return () => {
      active = false;
    };
  }, [team.id]);

  useEffect(() => {
    let active = true;
    listLibraryPlays({ team_id: team.id, include_archived: true })
      .then((plays) => {
        if (active) setTactics(tacticOptions(plays));
      })
      .catch(() => {
        if (active) setTactics([]);
      });
    return () => {
      active = false;
    };
  }, [team.id]);

  // A tactic deleted from the Libreta would empty every view: drop it and say why.
  useEffect(() => {
    if (tactics === null || !unknownTactic(scope.tactic, tactics)) return;
    setScope({ tactic: '' });
    toast.warning('La táctica del filtro ya no está en la Libreta y se ha quitado.');
  }, [scope.tactic, setScope, tactics]);

  const scoped = matches.filter(
    (match) =>
      (scope.map === '' || match.map === scope.map) &&
      inScopePeriod(match.played_at, scope.period, now) &&
      (scope.opponent === '' || rivalOf(match, team.id)?.team_id === scope.opponent),
  );
  const rivals = new Map<string, string>();
  for (const match of matches) {
    const rival = rivalOf(match, team.id);
    if (rival?.team_id) rivals.set(rival.team_id, teamLabel(rival));
  }
  const playedRivals = [...rivals].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'));
  let counts: ScopeCounts | null | undefined = { maps: scoped.length, rounds: scoped.reduce((sum, match) => sum + match.round_count, 0) };
  if (REPORTING_TABS.has(tab)) counts = reported;
  else if (tab === 'veto') counts = undefined;
  // Until a map (and for Playbook a side) is picked there is nothing counted yet: no 0 mapas · 0 rondas.
  if (tab === 'playbook' && (scope.map === '' || scope.side === '')) counts = undefined;
  if (tab === 'replay' && (replayForm === 'granadas' || replayForm === 'patrones') && scope.map === '') counts = undefined;
  const results = lastResults(
    scoped.map((match) => ({ playedAt: match.played_at, outcome: teamOutcome(match, team.id) })),
    10,
  );
  const scopeMatches: ScopeMatch[] = scoped.map((match) => ({
    id: match.id,
    href: matchHref(match.id),
    label: `${prettyMapName(match.map)} contra ${teamLabel(rivalOf(match, team.id))}`,
    date: formatShortDate(match.played_at),
    outcome: teamOutcome(match, team.id),
  }));
  const entity: ScopeEntity = {
    noun: 'Equipo',
    tone: 'team',
    id: team.id,
    options: [{ value: team.id, label: team.name }, ...(teams ?? []).map((entry) => ({ value: entry.id, label: entry.name }))],
    recents: true,
    hrefFor: teamHref,
  };
  const statsView = STATS_TABS.includes(tab);
  const activeScopeId = tab === 'replay' && (replayForm === 'granadas' || replayForm === 'patrones') ? `replay-${replayForm}` : tab;
  const mapCounts = useTeamMapCounts({ teamId: team.id, scope, fields: TAB_FIELDS[activeScopeId] ?? [], lineup, enabled: COUNTED_TABS.has(tab) });

  function scopeBar(id: string): ReactNode {
    const fields = TAB_FIELDS[id] ?? [];
    return (
      <ScopeBar
          scope={scope}
          onChange={changeScope}
          onReset={resetScope}
          proReference={proReference}
          fields={fields}
          entity={entity}
          lineups={lineups}
          maps={[...new Set(matches.map((match) => match.map))].sort()}
          mapCounts={id === activeScopeId ? mapCounts : null}
          mapRequired={id === 'playbook' || id === 'posiciones' || id === 'replay-granadas' || id === 'replay-patrones'}
          opponents={id === 'veto' ? (teams ?? []).map((entry) => ({ value: entry.id, label: entry.name })) : playedRivals}
          limits={id.startsWith('replay') ? TEAM_REPLAY_LIMITS : undefined}
          tactics={tactics ?? []}
          ownTeams={profile?.own_teams ?? 0}
          counts={counts}
          results={id === 'veto' || id === 'series' ? undefined : results}
          matches={id === 'veto' || id === 'series' ? undefined : scopeMatches}
          comparison={
            id === 'estadisticas' || id === 'rendimiento' ? (
              <CompareGroupPicker value={compare} teams={(teams ?? []).map((entry) => ({ value: entry.id, label: entry.name }))} onChange={setCompare} />
            ) : undefined
          }
        />
    );
  }

  function scopeBody(id: string, content: ReactNode): ReactNode {
    const fields = TAB_FIELDS[id] ?? [];
    return (
      <SampleLineupProvider showEveryone={fields.includes('lineup') && lineup.length > 0 ? showEveryone : null}>
        {COLUMN_TABS.has(id) && !statsView ? <div className={DOCUMENT_COLUMN}>{content}</div> : content}
      </SampleLineupProvider>
    );
  }

  // The stats view draws one bar above the tabs; the record view keeps a bar inside each tab.
  function withScope(id: string, content: ReactNode): ReactNode {
    if (statsView) return scopeBody(id, content);
    return (
      <div className="flex flex-col gap-4">
        {scopeBar(id)}
        {scopeBody(id, content)}
      </div>
    );
  }

  const unordered: RecordTab[] = [
      {
        id: 'estadisticas',
        label: 'Resumen',
        content: withScope(
          'estadisticas',
          <TeamSideStats teamId={team.id} scope={scope} lineup={lineup} compare={compare} onCounts={onCounts} />,
        ),
      },
      {
        id: 'rendimiento',
        label: 'Rendimiento de jugadores',
        content: withScope(
          'rendimiento',
          <TeamPlayerPerformance teamId={team.id} scope={scope} lineup={lineup} compare={compare} onCounts={onCounts} />,
        ),
      },
      {
        id: 'plantilla',
        label: 'Plantilla',
        count: team.roster?.length ?? 0,
        content: withScope(
          'plantilla',
          <div className="flex flex-col gap-3">
            <TeamRoster roster={team.roster ?? []} map={scope.map} reference={scope.reference} />
            <TeamTrend teamId={team.id} matches={matches.filter((match) => scope.map === '' || match.map === scope.map)} map={scope.map} />
          </div>,
        ),
      },
      {
        id: 'posiciones',
        label: 'Posiciones',
        content: withScope('posiciones', <TeamPositions teamId={team.id} scope={scope} lineup={lineup} />),
      },
      {
        id: 'sitios',
        label: 'Bombsites',
        content: withScope('sitios', <TeamBombsites teamId={team.id} scope={scope} lineup={lineup} onCounts={onCounts} />),
      },
      {
        id: 'veto',
        label: 'Veto',
        content: withScope('veto', <TeamVeto teamId={team.id} opponent={scope.opponent} teams={teams} />),
      },
      {
        id: 'partidas',
        label: 'Partidas',
        count: matches.length,
        content: withScope(
          'partidas',
          <div className="flex flex-col gap-6">
            <TeamMatches team={team} matches={scoped} />
            <section className="flex flex-col gap-2">
              <SlashTitle>Series</SlashTitle>
              <TeamSeries team={team} />
            </section>
          </div>,
        ),
      },
      {
        id: 'mapas',
        label: 'Mapas',
        count: maps.length,
        content: withScope('mapas', <TeamMapHistory teamId={team.id} matches={matches} scope={scope} onCounts={onCounts} />),
      },
      {
        id: 'preparar',
        label: 'Preparar',
        content: withScope(
          'preparar',
          <TeamPrepare
            team={team}
            maps={teamMaps}
            mapCounts={mapCounts}
            scope={scope}
            lineup={lineup}
            onMapChange={onMapChange}
            onOpenReplay={openReplay}
            onCounts={onCounts}
          />,
        ),
      },
      {
        id: 'playbook',
        label: 'Playbook',
        content: withScope(
          'playbook',
          <div className="flex flex-col gap-2">
            <Button asChild variant="ghost" size="xs" className="self-end">
              <Link href={teamLibretaHref(team.id, false)}>
                <BookMarked aria-hidden />
                Libreta del equipo
              </Link>
            </Button>
            <TeamPlaybook teamId={team.id} scope={scope} lineup={lineup} maps={teamMaps} onScopeChange={setScope} onCounts={onCounts} />
          </div>,
        ),
      },
      {
        id: 'replay',
        label: 'Replay',
        content: withScope(
          replayForm === 'granadas' || replayForm === 'patrones' ? `replay-${replayForm}` : 'replay',
          <TeamReplay
            teamId={team.id}
            scope={scope}
            lineup={lineup}
            matches={matches}
            mapCounts={mapCounts}
            roster={team.roster ?? []}
            form={replayForm}
            onFormChange={(form) => {
              setReported(null);
              openReplay(form);
            }}
            onMapChange={onMapChange}
            onScopeChange={setScope}
            onCounts={onCounts}
          />,
        ),
      },
  ];
  const tabList = TEAM_TABS.flatMap((id) => unordered.filter((entry) => entry.id === id));
  const changeTab = (next: string): void => {
    setReported(null);
    setTab(next);
  };

  if (statsView) {
    const recordHref = `${pathname}?${writeTab(search.toString(), 'plantilla', DEFAULT_TAB)}`;
    return (
      <div className="flex flex-col gap-5">
        {scopeBar(activeScopeId)}
        <div className={DOCUMENT_COLUMN}>
          <RecordTabs
            value={tab}
            onValueChange={changeTab}
            tabs={tabList.filter((entry) => STATS_TABS.includes(entry.id))}
            actions={
              <Button asChild variant="ghost" size="xs">
                <Link href={recordHref}>
                  <Shield aria-hidden />
                  Ficha del equipo
                </Link>
              </Button>
            }
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Equipo"
        icon={Shield}
        title={team.name}
        subtitle={subtitle === '' ? undefined : subtitle}
        fields={[
          { label: 'Partidas', value: matches.length },
          { label: 'Victorias', value: `${wins}-${losses}${draws > 0 ? `-${draws}` : ''}` },
          { label: '% victorias', value: percentLabel(wins, matches.length) || 'Sin partidas' },
          { label: 'Mapas', value: maps.length },
          { label: 'Ranking HLTV', value: rank > 0 ? `#${rank}` : 'Sin ranking' },
          { label: 'Última partida', value: last ? formatShortDate(last.played_at) : 'Sin partidas' },
        ]}
        actions={
          <>
            <Button asChild variant="outline" size="xs">
              <Link href={teamLibretaHref(team.id, true)}>
                <BookMarked aria-hidden />
                Jugadas repetidas
              </Link>
            </Button>
            <Button asChild variant="ghost" size="xs">
              <Link href={TEAMS_HREF}>
                <ArrowLeft aria-hidden />
                Todos los equipos
              </Link>
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button type="button" variant="ghost" size="icon-xs" aria-label="Más acciones del equipo">
                  <EllipsisVertical aria-hidden />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => setEditing(true)}>
                  <Pencil aria-hidden />
                  Editar
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => setMerging(true)}>
                  <GitMerge aria-hidden />
                  Fusionar
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        }
      />
      <RecordTabs value={tab} onValueChange={changeTab} tabs={tabList} />
      <EditTeamDialog open={editing} onOpenChange={setEditing} team={team} profile={profile} onSaved={onChanged} />
      <MergeTeamDialog open={merging} onOpenChange={setMerging} team={team} />
    </div>
  );
}

function TeamMatches({ team, matches }: { team: LibraryTeam; matches: readonly LibraryMatch[] }): ReactNode {
  const view = useTableView('equipo-partidas', {
    sort: { column: 'date', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const columns: DataTableColumn<LibraryMatch>[] = [
    {
      id: 'date',
      header: 'Fecha',
      required: true,
      cell: (match) => formatShortDate(match.played_at),
      sortValue: (match) => Date.parse(match.played_at) || null,
    },
    { id: 'map', header: 'Mapa', cell: (match) => <MapLabel map={match.map} />, sortValue: (match) => prettyMapName(match.map) },
    {
      id: 'rival',
      header: 'Rival',
      cell: (match) => teamLabel(rivalOf(match, team.id)),
      sortValue: (match) => teamLabel(rivalOf(match, team.id)),
    },
    {
      id: 'result',
      header: 'Resultado',
      cell: (match) => {
        const outcome = teamOutcome(match, team.id);
        if (outcome === null) return '';
        return (
          <span className={cn(outcome === 'win' && 'text-outcome-win', outcome === 'loss' && 'text-outcome-loss')}>
            {OUTCOME_LABEL[outcome]}
          </span>
        );
      },
      sortValue: (match) => teamOutcome(match, team.id),
    },
    {
      id: 'score',
      header: 'Marcador',
      cell: (match) => {
        const own = match.teams.find((entry) => entry.team_id === team.id);
        const rival = rivalOf(match, team.id);
        return <span>{`${own?.score ?? 0}-${rival?.score ?? 0}`}</span>;
      },
    },
    {
      id: 'side',
      header: 'Empezó',
      cell: (match) => {
        const side = match.teams.find((entry) => entry.team_id === team.id)?.start_side;
        return <span className={cn('text-meta', side === 'CT' ? 'text-side-ct' : 'text-side-t')}>{side ?? ''}</span>;
      },
    },
  ];
  return (
    <DataTable
      label={`Partidas de ${team.name}`}
      rows={matches}
      columns={columns}
      rowKey={(match) => match.id}
      rowHref={(match) => matchHref(match.id)}
      view={view}
      empty={<p className="text-body-sm text-fg-2">Este equipo aún no tiene partidas.</p>}
    />
  );
}
