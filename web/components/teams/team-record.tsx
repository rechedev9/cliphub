'use client';

import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { ArrowLeft, BookMarked, GitMerge, Pencil, PlugZap, Shield } from 'lucide-react';
import {
  getLibraryTeam,
  getTeamLineups,
  isLibraryNotFound,
  isLibraryOffline,
  listLibraryMatches,
  listLibraryTeams,
  mergeLibraryTeam,
  patchLibraryTeam,
  type LibraryMatch,
  type LibraryTeam,
} from '@/lib/api/library';
import type { TeamLineups } from '@/lib/api/team-insights';
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
import { inScopePeriod, type ScopeField } from '@/lib/analysis/scope';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import { lastResults } from '@/lib/analysis/context-bar';
import { COMPARE_PARAM, parseCompareTeams, writeCompareTeams } from '@/lib/analysis/side-stats';
import { SampleLineupProvider } from '@/components/analysis/empty-sample';
import { MapLabel } from '@/components/analysis/map-thumb';
import { CompareGroupPicker } from '@/components/analysis/compare-group-picker';
import { ScopeBar, type ScopeCounts, type ScopeEntity } from '@/components/analysis/scope-bar';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { useTableView } from '@/components/records/use-table-view';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { useAnalysisScope } from '@/hooks/use-analysis-scope';
import { useRecordTab } from '@/hooks/use-record-tab';
import { useReferences } from '@/hooks/use-references';
import { cn } from '@/lib/utils';
import { TeamBombsites } from './team-bombsites';
import { TeamMapHistory } from './team-map-history';
import { TeamRoster } from './team-roster';
import { TeamPrepare } from './team-prepare';
import { TEAM_REPLAY_FORMS, TeamReplay, type TeamReplayForm } from './team-replay';
import { REPLAY_FORM_QUERY } from '@/lib/replay/forms';
import { writeTab } from '@/lib/analysis/context-bar';
import { TeamPlaybook } from '@/components/playbook/team-playbook';
import { TeamSeries } from './team-series';
import { TeamVeto } from './team-veto';
import { TeamTrend } from './team-trend';
import { TeamSideStats } from './team-side-stats';

/** `lineups` is null when they could not be read; the views then count every round. */
type Loaded = { team: LibraryTeam; matches: LibraryMatch[]; lineups: TeamLineups | null };
type LoadState = { kind: 'loading' } | { kind: 'ready'; data: Loaded } | { kind: 'missing' } | { kind: 'offline' };

/** A team's record: roster by date, matches with rival and result, and the record per map. */
export function TeamRecord({ teamId }: { teamId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    Promise.all([getLibraryTeam(teamId), listLibraryMatches({ team_id: teamId }), getTeamLineups(teamId).catch(() => null)])
      .then(([team, matches, lineups]) => {
        if (active) setState({ kind: 'ready', data: { team, matches, lineups } });
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

/** The scope controls each tab reads, in the order the bar shows them. */
const TAB_FIELDS: Readonly<Record<string, readonly ScopeField[]>> = {
  plantilla: ['map', 'reference'],
  series: [],
  partidas: ['map', 'period', 'opponent'],
  mapas: ['map', 'period'],
  estadisticas: ['lineup', 'map', 'side', 'period', 'buy', 'opponentBuy', 'opponent'],
  veto: ['opponent'],
  sitios: ['lineup', 'map', 'period', 'opponent'],
  preparar: ['lineup', 'map', 'side', 'period', 'opponent'],
  playbook: ['lineup', 'map', 'side', 'period', 'opponent'],
  replay: ['lineup', 'side', 'period', 'buy', 'opponentBuy', 'opponent'],
  'replay-granadas': ['lineup', 'map', 'side', 'period', 'buy', 'opponent'],
};
/** Tabs whose own answer carries the sample size; the others count the library matches in scope. */
const REPORTING_TABS = new Set(['mapas', 'estadisticas', 'sitios', 'preparar', 'playbook', 'replay']);
const TEAM_TABS = ['plantilla', 'partidas', 'mapas', 'estadisticas', 'series', 'veto', 'sitios', 'preparar', 'playbook', 'replay'];
/** Tabs of older links: Scouting is in Preparar, and the radar and the grenades are forms of the Replay. */
const LEGACY_TABS: Readonly<Record<string, string>> = { scouting: 'preparar', radar: 'replay', granadas: 'replay' };
const QUERY_TABS = [...TEAM_TABS, ...Object.keys(LEGACY_TABS)];

function TeamRecordView({ data, onChanged }: { data: Loaded; onChanged: () => void }): ReactNode {
  const { team, matches, lineups } = data;
  const [editing, setEditing] = useState(false);
  const [merging, setMerging] = useState(false);
  const [rawTab, setTab] = useRecordTab(QUERY_TABS, 'plantilla');
  const tab = LEGACY_TABS[rawTab] ?? rawTab;
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const askedForm = search.get(REPLAY_FORM_QUERY) ?? (rawTab === 'granadas' ? 'granadas' : '');
  const replayForm: TeamReplayForm = TEAM_REPLAY_FORMS.find((entry) => entry === askedForm) ?? 'calor';
  // One replace for the tab and the form, so neither write loses the other.
  const openReplay = useCallback(
    (form: TeamReplayForm) => {
      const params = new URLSearchParams(writeTab(search.toString(), 'replay', 'plantilla'));
      params.set(REPLAY_FORM_QUERY, form);
      router.replace(`${pathname}?${params.toString()}`, { scroll: false });
    },
    [pathname, router, search],
  );
  const [reported, setReported] = useState<ScopeCounts | null>(null);
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [now] = useState(() => Date.now());
  const { scope, setScope, resetScope } = useAnalysisScope({ kind: 'team', id: team.id });
  const references = useReferences(scope.map);
  const proReference = references !== null && references !== 'failed' ? references.pro : null;
  const onCounts = useCallback((counts: ScopeCounts | null) => setReported(counts), []);
  const onMapChange = useCallback((map: string) => setScope({ map }), [setScope]);
  const teamMaps = useMemo(() => [...new Set(matches.map((match) => match.map))].sort(), [matches]);
  // Keyed on the ids, so a view refetches only when the players it counts change.
  const lineupKey = lineupSteamIds(scope.lineup, lineups).join(',');
  const lineup = useMemo(() => (lineupKey === '' ? [] : lineupKey.split(',')), [lineupKey]);
  const showEveryone = useCallback(() => setScope({ lineup: { kind: 'all' } }), [setScope]);
  const compareKey = parseCompareTeams(search.get(COMPARE_PARAM)).join(',');
  const compare = useMemo(() => (compareKey === '' ? [] : compareKey.split(',')), [compareKey]);
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
  if (tab === 'replay' && replayForm === 'granadas' && scope.map === '') counts = undefined;
  const results = lastResults(
    scoped.map((match) => ({ playedAt: match.played_at, outcome: teamOutcome(match, team.id) })),
    10,
  );
  const entity: ScopeEntity = {
    noun: 'Equipo',
    id: team.id,
    options: [{ value: team.id, label: team.name }, ...(teams ?? []).map((entry) => ({ value: entry.id, label: entry.name }))],
    hrefFor: teamHref,
  };

  function withScope(id: string, content: ReactNode): ReactNode {
    const fields = TAB_FIELDS[id] ?? [];
    return (
      <div className="flex flex-col gap-3">
        <ScopeBar
          scope={scope}
          onChange={setScope}
          onReset={resetScope}
          proReference={proReference}
          fields={fields}
          entity={entity}
          lineups={lineups}
          maps={[...new Set(matches.map((match) => match.map))].sort()}
          mapRequired={id === 'playbook' || id === 'replay-granadas'}
          opponents={id === 'veto' ? (teams ?? []).map((entry) => ({ value: entry.id, label: entry.name })) : playedRivals}
          counts={counts}
          results={id === 'veto' || id === 'series' ? undefined : results}
          comparison={
            id === 'estadisticas' ? (
              <CompareGroupPicker value={compare} teams={(teams ?? []).map((entry) => ({ value: entry.id, label: entry.name }))} onChange={setCompare} />
            ) : undefined
          }
        />
        <SampleLineupProvider showEveryone={fields.includes('lineup') && lineup.length > 0 ? showEveryone : null}>{content}</SampleLineupProvider>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Equipo"
        icon={Shield}
        title={team.name}
        subtitle={aliases.length > 0 ? `También como ${aliases.join(', ')}` : undefined}
        fields={[
          { label: 'Partidas', value: matches.length },
          { label: 'Victorias', value: `${wins}-${losses}${draws > 0 ? `-${draws}` : ''}` },
          { label: '% victorias', value: percentLabel(wins, matches.length) || 'Sin partidas' },
          { label: 'Mapas', value: maps.length },
          { label: 'Última partida', value: last ? formatShortDate(last.played_at) : 'Sin partidas' },
        ]}
        actions={
          <>
            <Button type="button" variant="ghost" size="xs" onClick={() => setEditing(true)}>
              <Pencil aria-hidden />
              Editar
            </Button>
            <Button type="button" variant="ghost" size="xs" onClick={() => setMerging(true)}>
              <GitMerge aria-hidden />
              Fusionar
            </Button>
            <Button asChild variant="ghost" size="xs">
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
          </>
        }
      />
      <RecordTabs
        value={tab}
        onValueChange={(next) => {
          setReported(null);
          setTab(next);
        }}
        tabs={[
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
            id: 'partidas',
            label: 'Partidas',
            count: matches.length,
            content: withScope('partidas', <TeamMatches team={team} matches={scoped} />),
          },
          {
            id: 'mapas',
            label: 'Mapas',
            count: maps.length,
            content: withScope('mapas', <TeamMapHistory teamId={team.id} matches={matches} scope={scope} onCounts={onCounts} />),
          },
          {
            id: 'estadisticas',
            label: 'Estadísticas',
            content: withScope(
              'estadisticas',
              <TeamSideStats teamId={team.id} scope={scope} lineup={lineup} compare={compare} onCounts={onCounts} />,
            ),
          },
          {
            id: 'series',
            label: 'Series',
            content: withScope('series', <TeamSeries team={team} />),
          },
          {
            id: 'veto',
            label: 'Veto',
            content: withScope('veto', <TeamVeto teamId={team.id} opponent={scope.opponent} teams={teams} />),
          },
          {
            id: 'sitios',
            label: 'Bombsites',
            content: withScope('sitios', <TeamBombsites teamId={team.id} scope={scope} lineup={lineup} onCounts={onCounts} />),
          },
          {
            id: 'preparar',
            label: 'Preparar',
            content: withScope(
              'preparar',
              <TeamPrepare
                team={team}
                matches={scoped}
                maps={teamMaps}
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
              <TeamPlaybook teamId={team.id} scope={scope} lineup={lineup} maps={teamMaps} onScopeChange={setScope} onCounts={onCounts} />,
            ),
          },
          {
            id: 'replay',
            label: 'Replay',
            content: withScope(
              replayForm === 'granadas' ? 'replay-granadas' : 'replay',
              <TeamReplay
                teamId={team.id}
                scope={scope}
                lineup={lineup}
                matches={matches}
                roster={team.roster ?? []}
                form={replayForm}
                onFormChange={(form) => {
                  setReported(null);
                  openReplay(form);
                }}
                onMapChange={onMapChange}
                onCounts={onCounts}
              />,
            ),
          },
        ]}
      />
      <EditTeamDialog open={editing} onOpenChange={setEditing} team={team} onSaved={onChanged} />
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

function EditTeamDialog({
  open,
  onOpenChange,
  team,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  team: LibraryTeam;
  onSaved: () => void;
}): ReactNode {
  const [name, setName] = useState(team.name);
  const [aliases, setAliases] = useState(team.aliases.join(', '));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setName(team.name);
    setAliases(team.aliases.join(', '));
    setError(null);
  }, [open, team.name, team.aliases]);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const list = aliases.split(',').map((alias) => alias.trim()).filter((alias) => alias !== '');
      await patchLibraryTeam(team.id, { name: name.trim(), aliases: list });
      onOpenChange(false);
      onSaved();
    } catch {
      setError('No se pudo guardar. Un alias que ya tiene otro equipo se resuelve fusionando los dos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar equipo</DialogTitle>
            <DialogDescription>
              Los alias son los nombres de clan que resuelven a este equipo. Las partidas asignadas solas con un alias nuevo pasan a él.
            </DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Nombre
            <Input autoFocus value={name} maxLength={120} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Alias, separados por comas
            <Input value={aliases} maxLength={600} onChange={(event) => setAliases(event.target.value)} />
          </label>
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || name.trim() === ''}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function MergeTeamDialog({
  open,
  onOpenChange,
  team,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  team: LibraryTeam;
}): ReactNode {
  const router = useRouter();
  const [teams, setTeams] = useState<LibraryTeam[]>([]);
  const [target, setTarget] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTarget('');
    setError(null);
    listLibraryTeams()
      .then((list) => setTeams(list.filter((entry) => entry.id !== team.id).sort((a, b) => a.name.localeCompare(b.name, 'es'))))
      .catch(() => setTeams([]));
  }, [open, team.id]);

  async function merge(): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      const into = await mergeLibraryTeam(team.id, target);
      onOpenChange(false);
      router.replace(teamHref(into.id));
    } catch {
      setError('No se pudieron fusionar los equipos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Fusionar {team.name}</DialogTitle>
          <DialogDescription>
            Sus partidas y sus alias pasan al equipo que elijas, y esta ficha desaparece. Sirve para un mismo equipo que aparece con dos nombres.
          </DialogDescription>
        </DialogHeader>
        {teams.length === 0 ? (
          <p className="text-body-sm text-fg-2">No hay otro equipo con el que fusionarlo.</p>
        ) : (
          <Select value={target} onValueChange={setTarget}>
            <SelectTrigger aria-label="Equipo que se queda" className="h-9 w-full">
              <SelectValue placeholder="Elige el equipo que se queda" />
            </SelectTrigger>
            <SelectContent>
              {teams.map((entry) => (
                <SelectItem key={entry.id} value={entry.id}>
                  {entry.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
            Cancelar
          </Button>
          <Button type="button" disabled={busy || target === ''} onClick={() => void merge()}>
            <GitMerge aria-hidden />
            Fusionar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
