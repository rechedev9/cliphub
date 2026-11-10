'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { CloudOff, PlugZap, Trophy, X } from 'lucide-react';
import { LibraryApiError, isLibraryOffline } from '@/lib/api/library';
import {
  PRO_PERIODS,
  followProTeam,
  groupByDay,
  proDayLabel,
  importProMatch,
  listProMatches,
  parseProQuery,
  proCatalogNotice,
  proQueryAddress,
  unfollowProTeam,
  type ProMatch,
  type ProMatchMap,
  type ProMatchQuery,
  type ProMatchTeam,
  type ProMatchesPage,
} from '@/lib/api/pro-matches';
import { prettyMapName } from '@/lib/format';
import { EmptyState } from '@/components/records/empty-state';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ProMatchCard } from './pro-match-card';

const ALL = 'all';

type Load = { kind: 'loading' } | { kind: 'offline' } | { kind: 'page'; page: ProMatchesPage };

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
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [importing, setImporting] = useState<string | null>(null);
  const [following, setFollowing] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  // The filters live in the address, so Back from the Replay keeps them; the entry is replaced, not added.
  useEffect(() => {
    const address = proQueryAddress(query);
    window.history.replaceState(null, '', address === '' ? window.location.pathname : `${window.location.pathname}?${address}`);
  }, [query]);

  useEffect(() => {
    let active = true;
    listProMatches(query)
      .then((page) => active && setLoad({ kind: 'page', page }))
      .catch((err: unknown) => active && setLoad(isLibraryOffline(err) ? { kind: 'offline' } : { kind: 'page', page: unavailablePage(query) }));
    return () => {
      active = false;
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
  const notice = proCatalogNotice(page.status);
  if (notice) {
    return (
      <EmptyState icon={CloudOff} title={notice.title} description={notice.description}
        actions={page.status === 'unavailable' ? <RetryButton onClick={() => setAttempt((n) => n + 1)} /> : undefined} />
    );
  }
  return (
    <div className="flex flex-col gap-5">
      <ProFilters page={page} query={query} onChange={change} />
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
      {page.matches.length === 0 ? (
        <EmptyState icon={Trophy} title="No hay partidas pro con estos filtros"
          description="Prueba con un periodo más largo o quita algún filtro. El servidor de ClipHub añade las partidas de HLTV cada día." />
      ) : (
        groupByDay(page.matches).map((group) => (
          <section key={group.day} aria-label={proDayLabel(group.day)} className="flex flex-col gap-3">
            <h2 className="text-body-sm font-semibold text-fg-2">{proDayLabel(group.day)}</h2>
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

/** The period choices, plus the one an address or an assistant link asked for. */
function periodOptions(days: number): { value: string; label: string }[] {
  const options = PRO_PERIODS.map((p) => ({ value: String(p.days), label: p.label }));
  return options.some((o) => o.value === String(days)) ? options : [...options, { value: String(days), label: `${days} días` }];
}

function ProFilters({ page, query, onChange }: { page: ProMatchesPage; query: ProMatchQuery; onChange: (patch: Partial<ProMatchQuery>) => void }): ReactNode {
  const period = String(query.days);
  return (
    <div className="flex flex-wrap items-center gap-3">
      <Segmented label="Periodo" value={period} onChange={(v) => onChange({ days: Number(v) })}
        options={periodOptions(query.days)} />
      <FilterSelect label="Equipo" all="Todos los equipos" value={query.team} onChange={(team) => onChange({ team })}
        options={page.facets.teams.map((t) => ({ value: String(t.id), label: t.rank ? `${t.name} (#${t.rank})` : t.name }))} />
      <FilterSelect label="Evento" all="Todos los eventos" value={query.event} onChange={(event) => onChange({ event })}
        options={page.facets.events.map((e) => ({ value: e.name, label: e.name }))} />
      <FilterSelect label="Mapa" all="Todos los mapas" value={query.map} onChange={(map) => onChange({ map })}
        options={page.facets.maps.map((m) => ({ value: m.name, label: prettyMapName(m.name) }))} />
    </div>
  );
}

type FilterSelectProps = {
  label: string;
  all: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
};

function FilterSelect({ label, all, value, options, onChange }: FilterSelectProps): ReactNode {
  // A chosen value stays listed even when the new period no longer has it.
  const shown = value && !options.some((o) => o.value === value) ? [{ value, label: value }, ...options] : options;
  return (
    <Select value={value || ALL} onValueChange={(next) => onChange(next === ALL ? '' : next)}>
      <SelectTrigger aria-label={label} className="h-8 w-48">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{all}</SelectItem>
        {shown.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
