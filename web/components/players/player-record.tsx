'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, ExternalLink, Link2, PlugZap, User } from 'lucide-react';
import { FACEIT_NOT_CONFIGURED_CODE } from '@/lib/api/types';
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
  type LibraryMatch,
  type LibraryPlayer,
  type LibraryPlayerRole,
  type LibraryRoleMetric,
} from '@/lib/api/library';
import { FACEIT_PLAYERS_HREF, PLAYERS_HREF, matchHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import {
  OUTCOME_LABEL,
  ROLE_LABEL,
  adrLabel,
  kdLabel,
  percentLabel,
  playerLine,
  playerTotals,
  teamByKey,
  teamLabel,
  utilityRatioLabel,
} from '@/lib/library/partidas';
import { roundEvidenceHref } from '@/lib/library/rounds';
import { FILTER_ANY, SEARCH_FILTER } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { RelatedList } from '@/components/records/related-list';
import { useTableView } from '@/components/records/use-table-view';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { LevelBadge } from '@/components/players/level-badge';
import { PlayerMatches } from '@/components/players/player-matches';
import { PlayerMetrics } from '@/components/metrics/player-metrics';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
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
            <Link href={PLAYERS_HREF}>Ver jugadores</Link>
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

function PlayerRecordView({ data }: { data: Loaded }): ReactNode {
  const [player, setPlayer] = useState(data.player);
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
          <>
            {player.faceit_player_id ? null : (
              <Button type="button" variant="outline" size="sm" disabled={linking} onClick={() => void linkFaceit()}>
                <Link2 aria-hidden />
                {linking ? 'Buscando en FACEIT' : 'Vincular FACEIT'}
              </Button>
            )}
            <Button asChild variant="ghost" size="sm">
              <Link href={PLAYERS_HREF}>
                <ArrowLeft aria-hidden />
                Todos los jugadores
              </Link>
            </Button>
          </>
        }
      />
      {linkError ? <p role="alert" className="text-body-sm text-destructive">{linkError}</p> : null}
      <RecordTabs
        tabs={[
          {
            id: 'resumen',
            label: 'Resumen',
            content: <PlayerSummary player={player} role={role} />,
          },
          {
            id: 'partidas',
            label: 'Partidas',
            count: data.rows.length,
            content: <PlayerMatchesTable steamId={player.steamid64} rows={data.rows} />,
          },
          {
            id: 'metricas',
            label: 'Métricas',
            content: (
              <PlayerMetrics
                steamId={player.steamid64}
                maps={[...new Set(data.rows.map((row) => row.match.map))].sort()}
                matches={rowIndex(data.rows)}
              />
            ),
          },
          {
            id: 'faceit',
            label: 'FACEIT',
            content: <PlayerFaceit player={player} linking={linking} onLink={() => void linkFaceit()} />,
          },
        ]}
      />
    </div>
  );
}

function PlayerSummary({ player, role }: { player: LibraryPlayer; role: LibraryPlayerRole | null | undefined }): ReactNode {
  return (
    <div className="flex flex-col gap-5">
      <RoleEvidence role={role} />
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

function RoleEvidence({ role }: { role: LibraryPlayerRole | null | undefined }): ReactNode {
  if (role === undefined) {
    return (
      <div role="status" aria-label="Calculando el rol">
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }
  if (role === null) {
    return (
      <RelatedList title="Rol">
        <p className="px-4 py-3 text-body-sm text-fg-2">No se pudo leer el rol de las partidas de este jugador.</p>
      </RelatedList>
    );
  }
  const { thresholds } = role;
  const ratio = utilityRatioLabel(role.utility, role.team_utility);
  return (
    <RelatedList title={`Rol: ${ROLE_LABEL[role.role]}`}>
      <p className="border-b border-border-subtle px-4 py-3 text-body-sm text-fg-2">
        Leído de {role.rounds} rondas en {role.match_count === 1 ? '1 partida' : `${role.match_count} partidas`}. Se prueba en este orden: AWPer con al menos el{' '}
        {Math.round(thresholds.awp_kill_share * 100)} % de sus kills con AWP, entry si abre el duelo en al menos el{' '}
        {Math.round(thresholds.entry_opening_share * 100)} % de sus rondas de T, soporte si lanza {thresholds.support_utility_ratio} veces las granadas por ronda de sus compañeros; si no, rifler. Con menos de {thresholds.min_rounds} rondas no se lee rol.
      </p>
      <dl className="grid divide-y divide-border-subtle @[64rem]/content:grid-cols-3 @[64rem]/content:divide-x @[64rem]/content:divide-y-0">
        <RoleMetricView
          label="Kills con AWP"
          metric={role.awp_kills}
          value={percentLabel(role.awp_kills.count, role.awp_kills.total)}
          summary={`${role.awp_kills.count} de ${role.awp_kills.total} kills`}
        />
        <RoleMetricView
          label="Duelos de apertura en T"
          metric={role.t_openings}
          value={percentLabel(role.t_openings.count, role.t_openings.total)}
          summary={`${role.t_openings.count} de ${role.t_openings.total} rondas de T`}
        />
        <RoleMetricView
          label="Granadas frente a sus compañeros"
          metric={role.utility}
          value={ratio === '' ? '' : `${ratio} veces`}
          summary={`${role.utility.count} granadas en ${role.utility.total} rondas`}
          note={`Compañeros: ${role.team_utility.count} granadas en ${role.team_utility.total} rondas.`}
        />
      </dl>
    </RelatedList>
  );
}

function RoleMetricView({
  label,
  metric,
  value,
  summary,
  note,
}: {
  label: string;
  metric: LibraryRoleMetric;
  value: string;
  summary: string;
  note?: string;
}): ReactNode {
  return (
    <div className="flex flex-col gap-1.5 px-4 py-3">
      <dt className="text-meta text-fg-3">{label}</dt>
      <dd className="flex flex-col gap-1.5">
        <span className="font-mono text-title font-semibold text-fg-1">{value || 'Sin dato'}</span>
        <span className="text-meta text-fg-2">{summary}</span>
        {note ? <span className="text-meta text-fg-3">{note}</span> : null}
        {metric.examples.length > 0 ? (
          <span className="flex flex-wrap gap-x-3 gap-y-1 text-meta">
            {metric.examples.map((example) => (
              <Link
                key={`${example.match_id}-${example.round}-${example.tick}`}
                href={roundEvidenceHref(matchHref(example.match_id), example.round, example.tick)}
                className="font-mono text-primary hover:underline"
                aria-label={`Abrir la ronda ${example.round}, tick ${example.tick}`}
              >
                R{example.round} t{example.tick}
              </Link>
            ))}
          </span>
        ) : null}
      </dd>
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
    { id: 'map', header: 'Mapa', cell: (row) => prettyMapName(row.match.map), sortValue: (row) => prettyMapName(row.match.map) },
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
            {profile.elo !== undefined ? <span className="font-mono text-fg-1">{profile.elo} ELO</span> : null}
            <a href={profile.profile_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-primary hover:underline">
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
