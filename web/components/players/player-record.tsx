'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ExternalLink, Link2, PlugZap, User } from 'lucide-react';
import { FACEIT_NOT_CONFIGURED_CODE } from '@/lib/api/types';
import type { MetricsReport } from '@/lib/api/metrics';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { lookupFaceitPlayer, type FaceitPlayer } from '@/lib/api/faceit';
import {
  getLibraryMatch,
  getLibraryPlayer,
  getLibraryPlayerRole,
  isLibraryNotFound,
  isLibraryOffline,
  LibraryApiError,
  linkLibraryPlayerFaceit,
  listLibraryMatches,
  listLibraryPlayers,
  type LibraryMatch,
  type LibraryPlayer,
  type LibraryPlayerRole,
} from '@/lib/api/library';
import { FACEIT_PLAYERS_HREF, TEAMS_HREF, matchHref, playerHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  OUTCOME_LABEL,
  ROLE_LABEL,
  adrLabel,
  kdLabel,
  playerLine,
  playerTotals,
  teamByKey,
  teamLabel,
} from '@/lib/library/partidas';
import { inScopePeriod, type ScopeField } from '@/lib/analysis/scope';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import { lastResults } from '@/lib/analysis/context-bar';
import { MapLabel } from '@/components/analysis/map-thumb';
import { ScopeBar, type ScopeCounts, type ScopeEntity, type ScopeOption } from '@/components/analysis/scope-bar';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { RelatedList } from '@/components/records/related-list';
import { useTableView } from '@/components/records/use-table-view';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { LevelBadge } from '@/components/players/level-badge';
import { PlayerMatches } from '@/components/players/player-matches';
import { BuyHeatTable, MapHeatTable } from '@/components/metrics/metrics-heat';
import {
  PLAYER_METRICS_LIMIT,
  PLAYER_METRICS_LIMITS,
  PlayerMetrics,
  PlayerReportGate,
  usePlayerReport,
} from '@/components/metrics/player-metrics';
import { PlayerIntro } from '@/components/players/player-intro';
import { PlayerRoles } from '@/components/players/player-roles';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useAnalysisScope } from '@/hooks/use-analysis-scope';
import { useRecordTab } from '@/hooks/use-record-tab';
import { useReferences } from '@/hooks/use-references';
import { cn } from '@/lib/utils';

/** How many of the player's most recent matches the record reads in full. */
const MATCH_DETAIL_LIMIT = 100;

type PlayerMatchRow = { match: LibraryMatch; teamName: string; rivalName: string; outcome: 'win' | 'loss' | 'draw' | null };
type Loaded = { player: LibraryPlayer; rows: PlayerMatchRow[]; lines: NonNullable<ReturnType<typeof playerLine>>[] };
type LoadState = { kind: 'loading' } | { kind: 'ready'; data: Loaded } | { kind: 'missing' } | { kind: 'offline' };

/** The player's matches as evidence labels: map, date and rival. */
function rowIndex(rows: readonly PlayerMatchRow[]): ReadonlyMap<string, ScoutMatchSummary> {
  return new Map(
    rows.map((row) => [
      row.match.id,
      {
        match_id: row.match.id, map: row.match.map, played_at: row.match.played_at, team_key: '',
        opponent: row.rivalName, score: 0, opponent_score: 0, rounds: row.match.round_count, wins: 0,
      },
    ]),
  );
}

async function loadPlayer(steamId: string): Promise<Loaded> {
  const [player, listed] = await Promise.all([getLibraryPlayer(steamId), listLibraryMatches({ steamid: steamId })]);
  // The list has no player lines; the record reads each match for them.
  const matches = await Promise.all(listed.slice(0, MATCH_DETAIL_LIMIT).map((match) => getLibraryMatch(match.id)));
  const rows: PlayerMatchRow[] = [];
  const lines: Loaded['lines'] = [];
  for (const match of matches) {
    const line = playerLine(match, steamId);
    if (line === undefined) continue;
    lines.push(line);
    const own = teamByKey(match, line.team_key);
    const rival = match.teams.find((team) => team.key !== line.team_key);
    let outcome: PlayerMatchRow['outcome'] = null;
    if (own !== undefined && rival !== undefined) {
      if (own.score > rival.score) outcome = 'win';
      else if (own.score < rival.score) outcome = 'loss';
      else outcome = 'draw';
    }
    rows.push({ match, teamName: teamLabel(own), rivalName: teamLabel(rival), outcome });
  }
  return { player, rows, lines };
}

/** A player's record: nicks and teams by date, matches with their lines, the inferred role and FACEIT. */
export function PlayerRecord({ steamId }: { steamId: string }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    loadPlayer(steamId)
      .then((data) => {
        if (active) setState({ kind: 'ready', data });
      })
      .catch((error: unknown) => {
        if (!active) return;
        setState(isLibraryNotFound(error) && !isLibraryOffline(error) ? { kind: 'missing' } : { kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [steamId, attempt]);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando jugador" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={User}
        title="Jugador no encontrado"
        description="Este SteamID64 no aparece en ninguna demo de la biblioteca."
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
        description="No se pudo leer el jugador. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <PlayerRecordView data={state.data} />;
}

/** The scope controls each tab reads; a tab missing here has no scope bar. */
const REPORT_FIELDS: readonly ScopeField[] = ['map', 'side', 'period', 'limit'];
const TAB_FIELDS: Readonly<Record<string, readonly ScopeField[]>> = {
  intro: [...REPORT_FIELDS, 'reference'],
  faceit: [],
  roles: ['map', 'period'],
  compras: REPORT_FIELDS,
  mapas: REPORT_FIELDS,
  partidas: ['map', 'period'],
  metricas: REPORT_FIELDS,
};
/** Tabs that read the shared metrics report; its sample is the count they show. */
const REPORT_TABS = new Set(['intro', 'compras', 'mapas', 'metricas']);
const PLAYER_TABS = ['intro', 'roles', 'compras', 'mapas', 'partidas', 'metricas', 'faceit'];
/** The picker lists the most played; a library of thousands would not fit a menu. */
const PICKER_PLAYERS = 200;

function PlayerRecordView({ data }: { data: Loaded }): ReactNode {
  const [player, setPlayer] = useState(data.player);
  const [tab, setTab] = useRecordTab(PLAYER_TABS, 'intro');
  const [others, setOthers] = useState<ScopeOption[]>([]);
  const [reported, setReported] = useState<ScopeCounts | null>(null);
  const [now] = useState(() => Date.now());
  const { scope, setScope, resetScope } = useAnalysisScope({ kind: 'player', id: data.player.steamid64 });
  const references = useReferences(scope.map);
  const proReference = references !== null && references !== 'failed' ? references.pro : null;
  const onCounts = useCallback((counts: ScopeCounts | null) => setReported(counts), []);
  const { state: reportState, retry } = usePlayerReport(data.player.steamid64, scope);
  const [role, setRole] = useState<LibraryPlayerRole | null | undefined>(undefined);
  const [linking, setLinking] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const totals = playerTotals(data.lines);
  const name = player.nick || player.steamid64;
  let roleValue = 'Calculando';
  if (role === null) roleValue = 'Sin dato';
  else if (role !== undefined) roleValue = ROLE_LABEL[role.role];
  useRouteTitle(name);
  useDocumentTitle(`${name} · Jugador · ClipHub`);

  useEffect(() => {
    let active = true;
    getLibraryPlayerRole(player.steamid64)
      .then((next) => {
        if (active) setRole(next);
      })
      .catch(() => {
        if (active) setRole(null);
      });
    return () => {
      active = false;
    };
  }, [player.steamid64]);

  useEffect(() => {
    let active = true;
    listLibraryPlayers()
      .then((list) => {
        if (!active) return;
        const top = [...list].sort((a, b) => b.match_count - a.match_count).slice(0, PICKER_PLAYERS);
        setOthers(
          top
            .map((entry) => ({ value: entry.steamid64, label: entry.nick || entry.steamid64 }))
            .sort((a, b) => a.label.localeCompare(b.label, 'es')),
        );
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  const maps = [...new Set(data.rows.map((row) => row.match.map))].sort();
  const details = new Map(data.rows.map((row) => [row.match.id, row.match]));
  const scopedRows = data.rows.filter(
    (row) => (scope.map === '' || row.match.map === scope.map) && inScopePeriod(row.match.played_at, scope.period, now),
  );

  function countsFor(id: string): ScopeCounts | null {
    if (id === 'roles') return reported;
    if (REPORT_TABS.has(id)) {
      return reportState.kind === 'ready' ? { maps: reportState.report.matches, rounds: reportState.report.rounds } : null;
    }
    return { maps: scopedRows.length, rounds: scopedRows.reduce((sum, row) => sum + row.match.round_count, 0) };
  }

  function withReport(id: string, body: (report: MetricsReport) => ReactNode): ReactNode {
    return withScope(
      id,
      <PlayerReportGate state={reportState} retry={retry}>
        {body}
      </PlayerReportGate>,
    );
  }

  const results = lastResults(
    scopedRows.map((row) => ({ playedAt: row.match.played_at, outcome: row.outcome })),
    10,
  );
  const entity: ScopeEntity = {
    noun: 'Jugador',
    id: player.steamid64,
    options: [{ value: player.steamid64, label: name }, ...others.filter((option) => option.value !== player.steamid64)],
    hrefFor: playerHref,
  };

  function withScope(id: string, content: ReactNode): ReactNode {
    return (
      <div className="flex flex-col gap-3">
        <ScopeBar
          scope={scope}
          onChange={setScope}
          onReset={resetScope}
          proReference={proReference}
          fields={TAB_FIELDS[id] ?? []}
          entity={entity}
          maps={maps}
          limits={{ options: PLAYER_METRICS_LIMITS, fallback: PLAYER_METRICS_LIMIT }}
          counts={id === 'faceit' ? undefined : countsFor(id)}
          results={id === 'faceit' ? undefined : results}
        />
        {content}
      </div>
    );
  }

  const linkFaceit = useCallback(async () => {
    setLinking(true);
    setLinkError(null);
    try {
      setPlayer(await linkLibraryPlayerFaceit(player.steamid64));
    } catch (error) {
      if (error instanceof LibraryApiError && error.status === 404) setLinkError('Este SteamID64 no tiene cuenta de CS2 en FACEIT.');
      else if (error instanceof LibraryApiError && (error.status === 503 || error.code === FACEIT_NOT_CONFIGURED_CODE)) {
        setLinkError('La conexión con FACEIT no está activada en este PC.');
      } else setLinkError('No se pudo consultar FACEIT. Vuelve a intentarlo.');
    } finally {
      setLinking(false);
    }
  }, [player.steamid64]);

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Jugador"
        icon={User}
        title={name}
        subtitle={<span className="font-mono text-meta">{player.steamid64}</span>}
        fields={[
          { label: 'Partidas', value: player.match_count },
          { label: 'Rondas', value: totals.rounds },
          { label: 'K/D', value: totals.deaths + totals.kills > 0 ? kdLabel(totals.kills, totals.deaths) : 'Sin dato' },
          { label: 'ADR', value: adrLabel(totals.damage, totals.rounds) || 'Sin dato' },
          { label: 'Rol', value: roleValue },
          { label: 'FACEIT', value: player.faceit_nickname ?? 'Sin vincular' },
        ]}
        actions={
          player.faceit_player_id ? null : (
            <Button type="button" variant="ghost" size="xs" disabled={linking} onClick={() => void linkFaceit()}>
              <Link2 aria-hidden />
              {linking ? 'Buscando en FACEIT' : 'Vincular FACEIT'}
            </Button>
          )
        }
      />
      {linkError ? <p role="alert" className="text-body-sm text-destructive">{linkError}</p> : null}
      <RecordTabs
        value={tab}
        onValueChange={(next) => {
          setReported(null);
          setTab(next);
        }}
        tabs={[
          {
            id: 'intro',
            label: 'Intro',
            content: (
              <div className="flex flex-col gap-5">
                {withReport('intro', (report) => (
                  <PlayerIntro report={report} details={details} map={scope.map} bothSides={scope.side === ''} reference={scope.reference} />
                ))}
                <PlayerSummary player={player} />
              </div>
            ),
          },
          {
            id: 'roles',
            label: 'Roles',
            content: withScope('roles', <PlayerRoles steamId={player.steamid64} role={role} scope={scope} onCounts={onCounts} />),
          },
          {
            id: 'compras',
            label: 'Compras',
            content: withReport('compras', (report) => (
              <div className="flex flex-col gap-3">
                <p className="measure-read text-body-sm text-fg-2">
                  Cada ronda cuenta con la compra de su propio equipo. Una force es media compra tras perder sin dinero; una semi, media compra planeada.
                </p>
                <BuyHeatTable report={report} options={{}} />
              </div>
            )),
          },
          {
            id: 'mapas',
            label: 'Mapas',
            count: maps.length,
            content: withReport('mapas', (report) => <MapHeatTable report={report} />),
          },
          {
            id: 'partidas',
            label: 'Partidas',
            count: data.rows.length,
            content: withScope('partidas', <PlayerMatchesTable steamId={player.steamid64} rows={scopedRows} />),
          },
          {
            id: 'metricas',
            label: 'Métricas',
            content: withReport('metricas', (report) => <PlayerMetrics report={report} matches={rowIndex(data.rows)} />),
          },
          {
            id: 'faceit',
            label: 'FACEIT',
            content: withScope('faceit', <PlayerFaceit player={player} linking={linking} onLink={() => void linkFaceit()} />),
          },
        ]}
      />
    </div>
  );
}

function PlayerSummary({ player }: { player: LibraryPlayer }): ReactNode {
  return (
    <div className="flex flex-col gap-5">
      <div className="grid gap-5 @[64rem]/content:grid-cols-2">
        <RelatedList title="Equipos" count={player.teams?.length ?? 0}>
          {(player.teams ?? []).length === 0 ? (
            <p className="px-4 py-3 text-body-sm text-fg-2">Sus partidas aún no tienen equipo asignado.</p>
          ) : (
            <ul className="divide-y divide-border-subtle">
              {(player.teams ?? []).map((team) => (
                <li key={team.team_id} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 text-body-sm">
                  <Link href={teamHref(team.team_id)} className="min-w-0 flex-1 truncate font-medium text-fg-1 hover:text-primary hover:underline">
                    {team.team_name}
                  </Link>
                  <span className="text-fg-2">{team.matches === 1 ? '1 partida' : `${team.matches} partidas`}</span>
                  <span className="text-fg-3">
                    {formatShortDate(team.first_played_at)} a {formatShortDate(team.last_played_at)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </RelatedList>
        <RelatedList title="Nicks" count={player.nicks?.length ?? 0}>
          <ul className="divide-y divide-border-subtle">
            {(player.nicks ?? []).map((nick) => (
              <li key={nick.nick} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 text-body-sm">
                <span className="min-w-0 flex-1 truncate font-medium text-fg-1">{nick.nick}</span>
                <span className="text-fg-3">
                  {formatShortDate(nick.first_seen_at)} a {formatShortDate(nick.last_seen_at)}
                </span>
              </li>
            ))}
          </ul>
        </RelatedList>
      </div>
    </div>
  );
}

function PlayerMatchesTable({ steamId, rows }: { steamId: string; rows: readonly PlayerMatchRow[] }): ReactNode {
  const view = useTableView('jugador-partidas', {
    sort: { column: 'date', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const line = (row: PlayerMatchRow) => playerLine(row.match, steamId);
  const columns: DataTableColumn<PlayerMatchRow>[] = [
    {
      id: 'date',
      header: 'Fecha',
      required: true,
      cell: (row) => formatShortDate(row.match.played_at),
      sortValue: (row) => Date.parse(row.match.played_at) || null,
    },
    { id: 'map', header: 'Mapa', cell: (row) => <MapLabel map={row.match.map} />, sortValue: (row) => prettyMapName(row.match.map) },
    { id: 'team', header: 'Equipo', cell: (row) => row.teamName, sortValue: (row) => row.teamName },
    { id: 'rival', header: 'Rival', cell: (row) => row.rivalName, sortValue: (row) => row.rivalName },
    {
      id: 'result',
      header: 'Resultado',
      cell: (row) =>
        row.outcome === null ? (
          ''
        ) : (
          <span className={cn(row.outcome === 'win' && 'text-outcome-win', row.outcome === 'loss' && 'text-outcome-loss')}>
            {OUTCOME_LABEL[row.outcome]}
          </span>
        ),
      sortValue: (row) => row.outcome,
    },
    { id: 'kills', header: 'K', align: 'end', cell: (row) => line(row)?.kills ?? '', sortValue: (row) => line(row)?.kills ?? null },
    { id: 'deaths', header: 'D', align: 'end', cell: (row) => line(row)?.deaths ?? '', sortValue: (row) => line(row)?.deaths ?? null },
    { id: 'assists', header: 'A', align: 'end', cell: (row) => line(row)?.assists ?? '' },
    {
      id: 'adr',
      header: 'ADR',
      align: 'end',
      cell: (row) => adrLabel(line(row)?.damage ?? 0, line(row)?.rounds ?? 0),
      sortValue: (row) => {
        const entry = line(row);
        return entry && entry.rounds > 0 ? entry.damage / entry.rounds : null;
      },
    },
  ];
  return (
    <DataTable
      label="Partidas del jugador"
      rows={rows}
      columns={columns}
      rowKey={(row) => row.match.id}
      rowHref={(row) => matchHref(row.match.id)}
      view={view}
      empty={<p className="text-body-sm text-fg-2">Sin partidas en la biblioteca.</p>}
    />
  );
}

function PlayerFaceit({ player, linking, onLink }: { player: LibraryPlayer; linking: boolean; onLink: () => void }): ReactNode {
  const [profile, setProfile] = useState<FaceitPlayer | null>(null);
  const nickname = player.faceit_nickname;

  useEffect(() => {
    if (!nickname) return;
    let active = true;
    lookupFaceitPlayer(nickname)
      .then((next) => {
        if (active) setProfile(next);
      })
      .catch(() => {
        if (active) setProfile(null);
      });
    return () => {
      active = false;
    };
  }, [nickname]);

  if (!player.faceit_player_id || !nickname) {
    return (
      <EmptyState
        icon={Link2}
        title="Sin cuenta de FACEIT vinculada"
        description="Studio busca en FACEIT la cuenta de este SteamID64 para mostrar su nivel, su ELO y su historial."
        actions={
          <>
            <Button type="button" size="sm" disabled={linking} onClick={onLink}>
              {linking ? 'Buscando en FACEIT' : 'Vincular FACEIT'}
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link href={FACEIT_PLAYERS_HREF}>Seguimiento FACEIT</Link>
            </Button>
          </>
        }
      />
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="studio-panel flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3 text-body-sm">
        <span className="font-semibold text-fg-1">{nickname}</span>
        {profile ? (
          <>
            <LevelBadge level={profile.skill_level} />
            {profile.elo !== undefined ? <span className="text-fg-1">{profile.elo} ELO</span> : null}
            <a href={profile.profile_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
              Perfil en FACEIT
              <ExternalLink className="size-3.5" aria-hidden />
            </a>
          </>
        ) : null}
      </div>
      <section aria-label={`Historial de FACEIT de ${nickname}`} className="studio-panel min-w-0 overflow-hidden">
        <PlayerMatches playerID={player.faceit_player_id} enabled />
      </section>
    </div>
  );
}
