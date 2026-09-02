'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import Link from '@/src/compat/link';
import { ExternalLink, Search, UploadCloud, Users, X } from 'lucide-react';
import {
  FACEIT_CODES,
  FaceitServiceError,
  followFaceitPlayer,
  listFaceitMatches,
  listFollowedFaceitPlayers,
  lookupFaceitPlayer,
  unfollowFaceitPlayer,
  type FaceitFollowedPlayer,
  type FaceitMatch,
  type FaceitMatchStats,
} from '@/lib/api/faceit';
import { FACEIT_NOT_CONFIGURED_CODE, SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

type LoadState = 'loading' | 'ready' | 'offline' | 'unconfigured';

const HEAD =
  'px-3 py-2.5 text-left font-[family-name:var(--font-mono)] text-meta font-normal uppercase tracking-wider text-fg-4';
const CELL = 'px-3 py-3 font-[family-name:var(--font-mono)] text-meta tabular-nums';

const LEVEL_COLOR: Record<number, string> = {
  1: 'bg-zinc-500',
  2: 'bg-emerald-500',
  3: 'bg-emerald-500',
  4: 'bg-amber-400',
  5: 'bg-amber-400',
  6: 'bg-amber-400',
  7: 'bg-orange-500',
  8: 'bg-orange-500',
  9: 'bg-rose-500',
  10: 'bg-rose-500',
};

function levelBg(level: number | undefined): string {
  if (level === undefined) return 'bg-fg-4';
  return LEVEL_COLOR[level] ?? 'bg-fg-4';
}

export default function PlayersPage(): ReactNode {
  const [state, setState] = useState<LoadState>('loading');
  const [players, setPlayers] = useState<FaceitFollowedPlayer[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [matches, setMatches] = useState<FaceitMatch[] | 'loading' | 'error' | null>(null);

  const selected = useMemo(
    () => players.find((player) => player.id === selectedID) ?? null,
    [players, selectedID],
  );
  const playersRef = useRef(players);
  playersRef.current = players;

  const refresh = useCallback(async () => {
    try {
      const listed = await listFollowedFaceitPlayers();
      setPlayers(listed.players);
      setState(listed.enabled ? 'ready' : 'unconfigured');
      setError(null);
      setSelectedID((current) => {
        if (current !== null && listed.players.some((player) => player.id === current)) return current;
        return listed.players[0]?.id ?? null;
      });
    } catch (err) {
      if (err instanceof FaceitServiceError && err.code === FACEIT_NOT_CONFIGURED_CODE) {
        setState('unconfigured');
        return;
      }
      setState('offline');
      setError(err instanceof Error ? err.message : 'No se pudieron cargar los jugadores');
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (selectedID === null) {
      setMatches(null);
      return;
    }
    let cancelled = false;
    setMatches('loading');
    const nickname = playersRef.current.find((player) => player.id === selectedID)?.nickname;
    void (async () => {
      if (nickname !== undefined) {
        try {
          const live = await lookupFaceitPlayer(nickname);
          if (cancelled) return;
          setPlayers((current) => current.map((player) => (player.id === live.id ? { ...player, ...live } : player)));
          await followFaceitPlayer(live.nickname);
        } catch {
          void 0;
        }
      }
      try {
        const rows = await listFaceitMatches(selectedID, 20);
        if (!cancelled) setMatches(rows);
      } catch {
        if (!cancelled) setMatches('error');
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedID]);

  async function onFollow(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const nickname = query.trim();
    if (nickname === '') return;
    setBusy(true);
    setError(null);
    try {
      const followed = await followFaceitPlayer(nickname);
      setPlayers((current) => [followed, ...current.filter((player) => player.id !== followed.id)]);
      setSelectedID(followed.id);
      setQuery('');
      setState('ready');
    } catch (err) {
      setError(followErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function onUnfollow(playerID: string): Promise<void> {
    try {
      await unfollowFaceitPlayer(playerID);
      setPlayers((current) => {
        const next = current.filter((player) => player.id !== playerID);
        setSelectedID((selectedNow) => (selectedNow === playerID ? next[0]?.id ?? null : selectedNow));
        return next;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo dejar de seguir');
    }
  }

  let body: ReactNode;
  if (state === 'loading') {
    body = <p className="text-body-sm text-fg-3">Cargando jugadores…</p>;
  } else if (players.length === 0) {
    body = (
      <StudioEmptyState
        icon={Users}
        title="Aún no sigues a nadie"
        description="Busca un nick o pega una URL de FACEIT para abrir su historial aquí."
        compact
      />
    );
  } else {
    body = (
      <div className="grid gap-6 lg:grid-cols-[minmax(220px,280px)_1fr] lg:gap-8">
        <nav aria-label="Jugadores seguidos" className="flex flex-col gap-1.5">
          {players.map((player) => (
            <button
              key={player.id}
              type="button"
              onClick={() => setSelectedID(player.id)}
              aria-current={selectedID === player.id ? 'true' : undefined}
              className={cn(
                'studio-panel studio-panel-interactive flex items-center gap-3 px-3 py-2.5 text-left transition-colors',
                selectedID === player.id
                  ? 'border-primary/50 bg-primary/8 shadow-[var(--elev-2),var(--glow-primary-sm)]'
                  : '',
              )}
            >
              <PlayerAvatar nickname={player.nickname} playerID={player.id} size={40} />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-body-sm font-semibold text-fg-1">{player.nickname}</span>
                <span className="flex items-center gap-2 font-[family-name:var(--font-mono)] text-meta tabular-nums text-fg-3">
                  <span className={cn('inline-block size-2 shrink-0 rounded-full', levelBg(player.skill_level))} aria-hidden />
                  {player.skill_level ?? '—'}
                  <span className="text-fg-4">/</span>
                  {player.elo ?? '—'}
                </span>
              </span>
            </button>
          ))}
        </nav>
        {selected ? (
          <div className="flex min-w-0 flex-col gap-5">
            <PlayerHeader player={selected} onUnfollow={() => void onUnfollow(selected.id)} />
            <MatchSection matches={matches} />
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8 sm:gap-10">
      <StudioPageHeader
        title="JUGADORES"
        description="Historial FACEIT de los jugadores que sigues. Abre la sala para descargar la demo y súbela a ClipHub."
        actions={
          <form onSubmit={(event) => void onFollow(event)} className="flex w-full max-w-md gap-2">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="nick o URL de FACEIT"
              aria-label="Buscar jugador FACEIT"
              disabled={state === 'unconfigured' || state === 'offline' || busy}
            />
            <Button type="submit" disabled={state !== 'ready' || busy || query.trim() === ''} loading={busy} loadingText="…">
              <Search className="size-4" />
              Seguir
            </Button>
          </form>
        }
      />

      {error ? <p className="text-body-sm text-destructive">{error}</p> : null}
      {state === 'unconfigured' ? (
        <StatusTag tone="danger">Configura FACEIT_API_KEY y reinicia el orquestador.</StatusTag>
      ) : null}
      {state === 'offline' ? <StatusTag tone="danger">Servicio local sin conexión.</StatusTag> : null}

      {body}
    </div>
  );
}

function PlayerHeader({
  player,
  onUnfollow,
}: {
  player: FaceitFollowedPlayer;
  onUnfollow: () => void;
}): ReactNode {
  return (
    <section className="studio-panel studio-panel-raised relative overflow-hidden p-5 sm:p-6">
      <div
        className="pointer-events-none absolute inset-0 opacity-40"
        style={{
          background: 'linear-gradient(135deg, color-mix(in oklch, var(--primary) 6%, transparent), transparent 60%)',
        }}
        aria-hidden
      />
      <div className="relative flex flex-wrap items-center gap-5">
        <div className="relative">
          <PlayerAvatar nickname={player.nickname} playerID={player.id} size={80} />
          <span
            className={cn(
              'absolute -right-1 -bottom-1 grid size-7 place-items-center rounded-md font-[family-name:var(--font-mono)] text-[11px] font-bold text-white shadow-md',
              levelBg(player.skill_level),
            )}
            title={`FACEIT Level ${player.skill_level ?? '?'}`}
          >
            {player.skill_level ?? '?'}
          </span>
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-3">
            <h2 className="font-display text-display-sm font-bold text-fg-1">{player.nickname}</h2>
            {player.country ? (
              <span className="font-[family-name:var(--font-mono)] text-body-sm uppercase tracking-wider text-fg-3">
                {player.country}
              </span>
            ) : null}
          </div>
          <div className="mt-3 flex flex-wrap gap-5">
            <StatPill label="ELO" value={player.elo !== undefined ? String(player.elo) : '—'} highlight />
            <StatPill label="Steam ID" value={player.steam_id64 ?? '—'} mono />
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button asChild variant="outline" size="sm" className="font-[family-name:var(--font-display)] uppercase tracking-wide">
            <a href={player.profile_url} target="_blank" rel="noreferrer">
              <ExternalLink className="size-3.5" />
              FACEIT
            </a>
          </Button>
          <Button asChild variant="outline-primary" size="sm">
            <Link href="/upload">
              <UploadCloud className="size-4" />
              Subir demo
            </Link>
          </Button>
          <Button type="button" variant="ghost" size="icon-sm" aria-label={`Dejar de seguir a ${player.nickname}`} onClick={onUnfollow}>
            <X className="size-4" />
          </Button>
        </div>
      </div>
    </section>
  );
}

function StatPill({ label, value, highlight, mono }: { label: string; value: string; highlight?: boolean; mono?: boolean }): ReactNode {
  return (
    <div className="flex flex-col">
      <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.14em] text-fg-4">{label}</span>
      <span
        className={cn(
          'text-body font-semibold',
          highlight ? 'text-primary' : 'text-fg-1',
          mono ? 'font-[family-name:var(--font-mono)] text-body-sm tracking-normal' : 'font-[family-name:var(--font-display)]',
        )}
      >
        {value}
      </span>
    </div>
  );
}

function MatchSection({ matches }: { matches: FaceitMatch[] | 'loading' | 'error' | null }): ReactNode {
  if (matches === null || matches === 'loading') {
    return <p className="text-body-sm text-fg-3">Cargando partidas…</p>;
  }
  if (matches === 'error') {
    return <p className="text-body-sm text-destructive">No se pudieron cargar las partidas.</p>;
  }
  if (matches.length === 0) {
    return <p className="text-body-sm text-fg-3">Sin partidas recientes en FACEIT.</p>;
  }

  const wins = matches.filter((match) => match.stats?.result === 'win').length;
  const losses = matches.length - wins;
  const winRate = matches.length > 0 ? Math.round((wins / matches.length) * 100) : 0;
  const kdValues = matches.map((match) => match.stats?.kd_ratio).filter((value): value is number => value !== undefined);
  const adrValues = matches.map((match) => match.stats?.adr).filter((value): value is number => value !== undefined);
  const hsValues = matches.map((match) => match.stats?.headshots_percent).filter((value): value is number => value !== undefined);
  const kd = average(kdValues);
  const adr = average(adrValues);
  const hs = average(hsValues);

  return (
    <div className="flex flex-col gap-5">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SummaryCard label="Win Rate" value={`${winRate}%`} sub={`${wins}W ${losses}L`} tone={winRate >= 50 ? 'good' : 'bad'} />
        <SummaryCard label="K/D" value={kd?.toFixed(2) ?? '—'} tone={kd !== undefined && kd >= 1.0 ? 'good' : 'bad'} />
        <SummaryCard label="ADR" value={adr !== undefined ? String(Math.round(adr)) : '—'} tone={adr !== undefined && adr >= 80 ? 'good' : 'neutral'} />
        <SummaryCard label="HS%" value={hs !== undefined ? `${Math.round(hs)}%` : '—'} tone="neutral" />
      </div>

      <section className="studio-panel overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[740px] border-collapse">
            <thead>
              <tr className="border-b border-border">
                <th className={HEAD}>Fecha</th>
                <th className={HEAD}>Mapa</th>
                <th className={HEAD}>Res</th>
                <th className={HEAD}>Score</th>
                <th className={HEAD}>K</th>
                <th className={HEAD}>D</th>
                <th className={HEAD}>A</th>
                <th className={HEAD}>K/D</th>
                <th className={HEAD}>ADR</th>
                <th className={HEAD}>HS%</th>
              </tr>
            </thead>
            <tbody>
              {matches.map((match) => {
                const isWin = match.stats?.result === 'win';
                const isLoss = match.stats?.result === 'loss';
                const mapName = prettyMapName(match.stats?.map ?? '') || 'partida';
                return (
                  <tr
                    key={match.id}
                    className={cn(
                      'cursor-pointer border-b border-border/60 transition-colors last:border-b-0',
                      isWin && 'bg-success/[0.04] hover:bg-success/[0.08]',
                      isLoss && 'bg-destructive/[0.04] hover:bg-destructive/[0.08]',
                      !isWin && !isLoss && 'hover:bg-surface-2',
                    )}
                    onClick={(event) => {
                      const link = event.currentTarget.querySelector('a');
                      const target = event.target;
                      if (!(link instanceof HTMLAnchorElement)) return;
                      if (target instanceof Node && link.contains(target)) return;
                      link.click();
                    }}
                  >
                    <td className={cn(CELL, 'text-fg-3')}>
                      <a
                        href={match.room_url}
                        target="_blank"
                        rel="noreferrer"
                        aria-label={`Abrir sala FACEIT de ${mapName}`}
                        className="text-inherit no-underline"
                      >
                        {match.finished_at ? formatShortDate(match.finished_at) : '—'}
                      </a>
                    </td>
                    <td className={cn(CELL, 'font-semibold text-fg-1')}>{prettyMapName(match.stats?.map ?? '') || '—'}</td>
                    <td className={cn(CELL, 'font-bold', resultClass(match.stats?.result))}>{resultLabel(match.stats?.result)}</td>
                    <td className={cn(CELL, 'text-fg-2')}>{scoreLabel(match)}</td>
                    <td className={cn(CELL, 'text-fg-1')}>{match.stats?.kills ?? '—'}</td>
                    <td className={cn(CELL, 'text-fg-2')}>{match.stats?.deaths ?? '—'}</td>
                    <td className={cn(CELL, 'text-fg-3')}>{match.stats?.assists ?? '—'}</td>
                    <td className={cn(CELL, kdClass(match.stats?.kd_ratio))}>{match.stats?.kd_ratio?.toFixed(2) ?? '—'}</td>
                    <td className={cn(CELL, adrClass(match.stats?.adr))}>{match.stats?.adr !== undefined ? Math.round(match.stats.adr) : '—'}</td>
                    <td className={cn(CELL, 'text-fg-2')}>{match.stats?.headshots_percent !== undefined ? Math.round(match.stats.headshots_percent) : '—'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function SummaryCard({
  label,
  value,
  sub,
  tone,
}: {
  label: string;
  value: string;
  sub?: string;
  tone: 'good' | 'bad' | 'neutral';
}): ReactNode {
  const toneStyles: Record<typeof tone, { border: string; value: string }> = {
    good: { border: 'border-success/30', value: 'text-success' },
    bad: { border: 'border-destructive/30', value: 'text-destructive' },
    neutral: { border: 'border-border', value: 'text-fg-1' },
  };
  const { border, value: valueColor } = toneStyles[tone];
  return (
    <div className={cn('studio-panel flex flex-col items-center gap-1 border px-4 py-3 text-center', border)}>
      <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.14em] text-fg-4">{label}</span>
      <span className={cn('font-[family-name:var(--font-display)] text-section font-bold', valueColor)}>{value}</span>
      {sub ? <span className="font-[family-name:var(--font-mono)] text-meta text-fg-3">{sub}</span> : null}
    </div>
  );
}

function PlayerAvatar({
  nickname,
  playerID,
  size,
}: {
  nickname: string;
  playerID?: string;
  size: number;
}): ReactNode {
  const [broken, setBroken] = useState(false);
  const letter = nickname.slice(0, 1).toUpperCase() || '?';
  const src = playerID !== undefined ? `/api/faceit/players/${encodeURIComponent(playerID)}/avatar` : undefined;
  if (src !== undefined && !broken) {
    return (
      <img
        src={src}
        alt=""
        width={size}
        height={size}
        onError={() => setBroken(true)}
        className="shrink-0 rounded-md object-cover"
        style={{ width: size, height: size }}
      />
    );
  }
  return (
    <span
      className="grid shrink-0 place-items-center rounded-md bg-surface-3 font-display font-bold text-fg-3"
      style={{ width: size, height: size, fontSize: size * 0.4 }}
      aria-hidden
    >
      {letter}
    </span>
  );
}

function resultLabel(result: FaceitMatchStats['result'] | undefined): string {
  if (result === 'win') return 'W';
  if (result === 'loss') return 'L';
  return '·';
}

function resultClass(result: FaceitMatchStats['result'] | undefined): string {
  if (result === 'win') return 'text-success';
  if (result === 'loss') return 'text-destructive';
  return 'text-fg-3';
}

function kdClass(kd: number | undefined): string {
  if (kd === undefined) return 'text-fg-3';
  if (kd >= 1.3) return 'text-emerald-400';
  if (kd >= 1.0) return 'text-fg-1';
  return 'text-fg-2';
}

function adrClass(adr: number | undefined): string {
  if (adr === undefined) return 'text-fg-3';
  if (adr >= 100) return 'text-emerald-400';
  if (adr >= 80) return 'text-fg-1';
  return 'text-fg-2';
}

function scoreLabel(match: FaceitMatch): string {
  if (match.score.for === undefined || match.score.against === undefined) return '—';
  return `${match.score.for}-${match.score.against}`;
}

function average(values: number[]): number | undefined {
  if (values.length === 0) return undefined;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function followErrorMessage(err: unknown): string {
  if (!(err instanceof FaceitServiceError)) {
    return err instanceof Error ? err.message : 'No se pudo seguir al jugador';
  }
  if (err.code === FACEIT_NOT_CONFIGURED_CODE) return 'Configura FACEIT_API_KEY y reinicia el orquestador.';
  if (err.code === SERVICE_UNAVAILABLE_CODE) return 'Servicio local sin conexión.';
  if (err.code === FACEIT_CODES.rateLimited) return 'FACEIT está limitando peticiones.';
  if (err.status === 404) return 'Jugador no encontrado.';
  if (err.status === 409) return 'Límite de jugadores seguidos.';
  return err.message;
}
