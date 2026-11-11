'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { CloudOff, PlugZap, Trophy, X } from 'lucide-react';
import { LibraryApiError, isLibraryOffline } from '@/lib/api/library';
import {
  followProTeam,
  groupByDay,
  importProMatch,
  listProMatches,
  parseProQuery,
  proCatalogNotice,
  proQueryAddress,
  refreshedProPage,
  unfollowProTeam,
  type ProMatch,
  type ProMatchMap,
  type ProMatchQuery,
  type ProMatchTeam,
  type ProMatchesPage,
} from '@/lib/api/pro-matches';
import { startPollLoop } from '@/lib/poll-loop';
import { DayHeading } from '@/components/matches/series-block';
import { EmptyState } from '@/components/records/empty-state';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { Button } from '@/components/ui/button';
import { ProMatchCard } from './pro-match-card';
import { ProFilters } from './pro-match-filters';

/** The address key of the second team of "VS."; the catalog filters one team, so this one narrows the page read. */
const RIVAL_KEY = 'contra';

const PRO_REFRESH_MS = 10 * 60_000;
/** While the local service is not answering yet, such as right after Studio starts. */
const PRO_RETRY_MS = 15_000;
/** Switching windows often reads the catalog at most once a minute. */
const PRO_REFRESH_GAP_MS = 60_000;

type Load ={ kind: 'loading' } | { kind: 'offline' } | { kind: 'page'; page: ProMatchesPage };

const IMPORT_ERROR: Record<string, string> = {
  no_pro_map: 'El servidor de ClipHub ya no tiene el análisis de este mapa.',
  pro_map_unreadable: 'El servidor de ClipHub envió un análisis de este mapa que Studio no puede leer. Prueba con otro mapa o vuelve a intentarlo cuando se actualice.',
  pro_map_pending: 'La demo de este mapa ya está en tu biblioteca y se está analizando. Ábrela desde Partidas cuando termine.',
  community_unavailable: 'El servidor de ClipHub no responde. Vuelve a intentarlo en un rato.',
  community_disabled: 'Este Studio no se conecta al servidor de ClipHub.',
};

function importErrorText(error: unknown): string {
  if (error instanceof LibraryApiError && error.code && IMPORT_ERROR[error.code]) return IMPORT_ERROR[error.code] ?? '';
  return 'No se pudo abrir el mapa. Vuelve a intentarlo.';
}

/** Partidas pro: the server's recent HLTV matches, filtered, with a 2D button per map and Seguir equipo. */
export function ProMatchList(): ReactNode {
  const router = useRouter();
  const search = useSearchParams();
  const [query, setQuery] = useState<ProMatchQuery>(() => parseProQuery(search));
  const [rival, setRival] = useState(() => (search.get(RIVAL_KEY) ?? '').trim().slice(0, 20));
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [importing, setImporting] = useState<string | null>(null);
  const [following, setFollowing] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  // The filters live in the address, so Back from the Replay keeps them; the entry is replaced, not added.
  useEffect(() => {
    const params = new URLSearchParams(proQueryAddress(query));
    if (rival !== '' && query.team !== '') params.set(RIVAL_KEY, rival);
    const address = params.toString();
    window.history.replaceState(null, '', address === '' ? window.location.pathname : `${window.location.pathname}?${address}`);
  }, [query, rival]);

  // The server adds matches every day and the follow sync imports maps: read again on a timer and when the window comes back.
  useEffect(() => {
    let active = true;
    let first = true;
    let readAt = 0;
    const stop = startPollLoop({
      tick: async () => {
        if (Date.now() - readAt < PRO_REFRESH_GAP_MS) return 'idle';
        const shown = first;
        first = false;
        try {
          const page = await listProMatches(query);
          if (active) setLoad((prev) => ({ kind: 'page', page: refreshedProPage(prev.kind === 'page' ? prev.page : null, page) }));
          // A server that did not answer is asked again on the next focus, not every few seconds.
          if (page.status !== 'unavailable') readAt = Date.now();
          return 'idle';
        } catch (err: unknown) {
          // A failed refresh keeps what is on screen; only the first read shows the error.
          if (active && shown) setLoad(isLibraryOffline(err) ? { kind: 'offline' } : { kind: 'page', page: unavailablePage(query) });
          return 'fast';
        }
      },
      fastMs: PRO_RETRY_MS,
      idleMs: PRO_REFRESH_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [query, attempt]);

  const change = (patch: Partial<ProMatchQuery>): void => {
    setQuery((q) => ({ ...q, page: 1, ...patch }));
    setError(null);
  };

  async function open(match: ProMatch, map: ProMatchMap): Promise<void> {
    setImporting(`${match.id}/${map.entry}`);
    setError(null);
    try {
      const res = await importProMatch(match.id, map.entry);
      router.push(res.studio_path);
    } catch (err: unknown) {
      setError(importErrorText(err));
      setImporting(null);
    }
  }

  async function toggleFollow(team: { id: number; name: string; followed: boolean }): Promise<void> {
    setFollowing(team.id);
    setError(null);
    try {
      if (team.followed) await unfollowProTeam(team.id);
      else await followProTeam(team.id, team.name);
      setAttempt((n) => n + 1);
    } catch {
      setError(team.followed ? `No se pudo dejar de seguir a ${team.name}.` : `No se pudo seguir a ${team.name}.`);
    } finally {
      setFollowing(null);
    }
  }

  if (load.kind === 'loading') return <TableSkeleton label="Cargando partidas pro" columns={3} />;
  if (load.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer las partidas pro. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={<RetryButton onClick={() => setAttempt((n) => n + 1)} />}
      />
    );
  }
  const { page } = load;
  const matches = rival === '' || query.team === '' ? page.matches : page.matches.filter((m) => m.teams.some((team) => String(team.id) === rival));
  const notice = proCatalogNotice(page.status);
  if (notice) {
    return (
      <EmptyState icon={CloudOff} title={notice.title} description={notice.description}
        actions={page.status === 'unavailable' ? <RetryButton onClick={() => setAttempt((n) => n + 1)} /> : undefined} />
    );
  }
  return (
    <div className="flex flex-col gap-5">
      <ProFilters page={page} query={query} rival={rival} onChange={change} onRivalChange={setRival} />
      {page.follows.length > 0 ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-body-sm text-fg-2">Sigues a</span>
          {page.follows.map((f) => (
            <Button key={f.team_id} type="button" variant="secondary" size="sm" disabled={following === f.team_id}
              onClick={() => void toggleFollow({ id: f.team_id, name: f.name, followed: true })} aria-label={`Dejar de seguir a ${f.name}`}>
              {f.name}
              <X aria-hidden />
            </Button>
          ))}
        </div>
      ) : null}
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      {matches.length === 0 ? (
        <EmptyState icon={Trophy} title="No hay partidas pro con estos filtros"
          description="Prueba con un periodo más largo o quita algún filtro. El servidor de ClipHub añade las partidas de HLTV cada día." />
      ) : (
        groupByDay(matches).map((group) => (
          <section key={group.day} className="flex flex-col gap-2">
            <DayHeading day={group.day} />
            {group.matches.map((m) => (
              <ProMatchCard key={m.id} match={m} importing={importing} following={following}
                onOpen={(match, map) => void open(match, map)} onToggleFollow={(team: ProMatchTeam) => void toggleFollow(team)} />
            ))}
          </section>
        ))
      )}
      {page.pages > 1 ? (
        <nav aria-label="Páginas" className="flex items-center justify-between gap-3">
          <Button type="button" variant="outline" size="sm" disabled={query.page <= 1} onClick={() => setQuery((q) => ({ ...q, page: q.page - 1 }))}>
            Más recientes
          </Button>
          <span className="text-meta text-fg-3 tabular-nums">Página {page.page} de {page.pages}</span>
          <Button type="button" variant="outline" size="sm" disabled={query.page >= page.pages} onClick={() => setQuery((q) => ({ ...q, page: q.page + 1 }))}>
            Anteriores
          </Button>
        </nav>
      ) : null}
    </div>
  );
}

function RetryButton({ onClick }: { onClick: () => void }): ReactNode {
  return (
    <Button type="button" variant="outline" size="sm" onClick={onClick}>
      Reintentar
    </Button>
  );
}

function unavailablePage(query: ProMatchQuery): ProMatchesPage {
  return { status: 'unavailable', days: query.days, page: 1, pages: 1, total: 0, matches: [], facets: { teams: [], events: [], maps: [] }, follows: [] };
}
