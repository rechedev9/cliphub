'use client';

import { useState, type ReactNode } from 'react';
import { ChevronLeft, ChevronRight, ExternalLink, RefreshCw } from 'lucide-react';
import type { FaceitMatch, FaceitMatchStats } from '@/lib/api/faceit';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { cn } from '@/lib/utils';
import { MapCover } from '@/components/brand/map-cover';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const ALL = 'all';
const UNKNOWN = 'unknown';
const PAGE_SIZE = 7;
const HEAD = 'px-3 py-3 text-left text-body-sm font-medium whitespace-nowrap text-fg-2';
const CELL = 'px-3 py-1 text-body-sm whitespace-nowrap tabular-nums text-fg-2';
const RESULTS = {
  win: { label: 'Victoria', tone: 'success' },
  loss: { label: 'Derrota', tone: 'danger' },
  unknown: { label: 'Sin resultado', tone: 'neutral' },
} as const;

export function MatchHistory({ matches, refreshing, onRefresh }: {
  matches: FaceitMatch[];
  refreshing: boolean;
  onRefresh: () => void;
}): ReactNode {
  const [map, setMap] = useState(ALL);
  const [result, setResult] = useState(ALL);
  const [page, setPage] = useState(1);
  const maps = [...new Set(matches.flatMap((match) => match.stats?.map ? [match.stats.map] : []))].sort();
  const filtered = matches.filter((match) => (map === ALL || match.stats?.map === map)
    && (result === ALL || (match.stats?.result ?? UNKNOWN) === result));
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const currentPage = Math.min(page, pages);
  const start = (currentPage - 1) * PAGE_SIZE;
  const visible = filtered.slice(start, start + PAGE_SIZE);

  return (
    <section aria-labelledby="match-history-heading" aria-busy={refreshing} className="min-w-0">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-baseline gap-3">
          <h3 id="match-history-heading" className="text-body font-semibold text-fg-1">Historial de partidas</h3>
          <span className="text-body-sm text-fg-3">{matches.length} partidas</span>
        </div>
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Select value={map} onValueChange={(value) => { setMap(value); setPage(1); }}>
            <SelectTrigger aria-label="Filtrar por mapa" className="h-10"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value={ALL}>Todos los mapas</SelectItem>
              {maps.map((value) => <SelectItem key={value} value={value}>{prettyMapName(value)}</SelectItem>)}
            </SelectContent>
          </Select>
          <Select value={result} onValueChange={(value) => { setResult(value); setPage(1); }}>
            <SelectTrigger aria-label="Filtrar por resultado" className="h-10"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value={ALL}>Todos los resultados</SelectItem>
              <SelectItem value="win">Victorias</SelectItem><SelectItem value="loss">Derrotas</SelectItem>
              <SelectItem value={UNKNOWN}>Sin resultado</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" size="icon-sm" aria-label="Actualizar partidas" disabled={refreshing} onClick={onRefresh}>
            <RefreshCw aria-hidden className={cn('size-4', refreshing && 'animate-spin motion-reduce:animate-none')} />
          </Button>
        </div>
      </div>
      <div className="hidden overflow-hidden rounded-lg border border-border @[40rem]/content:block">
        <div className="overflow-x-auto focus-visible:outline-primary" role="region" aria-label="Tabla de partidas" tabIndex={0}>
          <table className="w-full min-w-[780px] border-collapse">
            <caption className="sr-only">Partidas recientes de FACEIT. Abre una sala para descargar su demo.</caption>
            <thead className="bg-surface-3"><tr className="border-b border-border">
              {['Fecha', 'Mapa', 'Resultado', 'Marcador', 'K / D / A', 'K/D', 'ADR', 'HS', 'Acción'].map((heading) => (
                <th key={heading} scope="col" className={HEAD}>{heading}</th>
              ))}
            </tr></thead>
            <tbody>
              {visible.map((match) => <MatchRow key={match.id} match={match} />)}
              {visible.length === 0 ? <tr><td colSpan={9} className="px-4 py-10 text-center text-body-sm text-fg-2">
                No hay partidas con estos filtros.
                <Button variant="link" onClick={() => { setMap(ALL); setResult(ALL); setPage(1); }}>Restablecer filtros</Button>
              </td></tr> : null}
            </tbody>
          </table>
        </div>
      </div>
      <ul aria-label="Partidas recientes" className="space-y-3 @[40rem]/content:hidden">
        {visible.map((match) => <MobileMatch key={match.id} match={match} />)}
        {visible.length === 0 ? <li className="rounded-lg border border-border p-4 text-body-sm text-fg-2">
          No hay partidas con estos filtros.
          <Button variant="link" onClick={() => { setMap(ALL); setResult(ALL); setPage(1); }}>Restablecer filtros</Button>
        </li> : null}
      </ul>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
        <p role="status" className="text-body-sm tabular-nums text-fg-3">
          Mostrando {filtered.length === 0 ? 0 : start + 1}–{Math.min(start + PAGE_SIZE, filtered.length)} de {filtered.length} partidas
        </p>
        <nav aria-label="Paginación de partidas" className="flex items-center gap-1">
          <Button variant="outline" size="icon-sm" aria-label="Página anterior" disabled={currentPage === 1} onClick={() => setPage(currentPage - 1)}><ChevronLeft aria-hidden className="size-4" /></Button>
          {Array.from({ length: pages }, (_, index) => index + 1).map((number) => (
            <Button key={number} variant={number === currentPage ? 'outline-primary' : 'outline'} size="icon-sm"
              aria-label={`Página ${number}`} aria-current={number === currentPage ? 'page' : undefined}
              onClick={() => setPage(number)} className="shadow-none">{number}</Button>
          ))}
          <Button variant="outline" size="icon-sm" aria-label="Página siguiente" disabled={currentPage === pages} onClick={() => setPage(currentPage + 1)}><ChevronRight aria-hidden className="size-4" /></Button>
        </nav>
      </div>
    </section>
  );
}

function MatchRow({ match }: { match: FaceitMatch }): ReactNode {
  const stats = match.stats;
  const map = prettyMapName(stats?.map ?? '') || 'Sin mapa';
  const score = match.score.for !== undefined && match.score.against !== undefined ? `${match.score.for}–${match.score.against}` : '—';
  return (
    <tr className="h-12 border-b border-border-subtle transition-colors last:border-b-0 hover:bg-surface-3 focus-within:bg-surface-3">
      <td className={CELL}>{match.finished_at ? formatShortDate(match.finished_at) : '—'}</td>
      <td className={CELL}><div className="flex items-center gap-2.5">
        <MapCover map={stats?.map ?? ''} className="h-8 w-11 shrink-0 rounded-sm" />
        <span className="font-semibold text-fg-1">{map}</span>
      </div></td>
      <td className={CELL}><ResultBadge result={stats?.result} /></td>
      <td className={cn(CELL, 'text-fg-1')}>{score}</td>
      <td className={CELL}>{stats ? `${stats.kills} / ${stats.deaths} / ${stats.assists}` : '—'}</td>
      <td className={cn(CELL, stats?.kd_ratio !== undefined && stats.kd_ratio >= 1.3 && 'text-success')}>{stats?.kd_ratio?.toFixed(2) ?? '—'}</td>
      <td className={cn(CELL, stats?.adr !== undefined && stats.adr >= 100 && 'text-success')}>{stats?.adr !== undefined ? Math.round(stats.adr) : '—'}</td>
      <td className={CELL}>{stats?.headshots_percent !== undefined ? `${Math.round(stats.headshots_percent)}%` : '—'}</td>
      <td className={cn(CELL, 'text-right')}><Button asChild variant="link" size="sm" className="px-0 has-[>svg]:px-0">
        <a href={match.room_url} target="_blank" rel="noreferrer" aria-label={`Abrir sala FACEIT de ${map}`}>
          Abrir sala <ExternalLink aria-hidden className="size-3.5" />
        </a>
      </Button></td>
    </tr>
  );
}

function ResultBadge({ result }: { result?: FaceitMatchStats['result'] }): ReactNode {
  const { label, tone } = RESULTS[result ?? UNKNOWN];
  return <StatusTag tone={tone} className="rounded-sm font-sans normal-case tracking-normal">{label}</StatusTag>;
}

function MobileMatch({ match }: { match: FaceitMatch }): ReactNode {
  const stats = match.stats;
  const map = prettyMapName(stats?.map ?? '') || 'Sin mapa';
  const score = match.score.for !== undefined && match.score.against !== undefined ? `${match.score.for}–${match.score.against}` : '—';
  return (
    <li className="rounded-lg border border-border p-3">
      <div className="flex items-center gap-2.5">
        <MapCover map={stats?.map ?? ''} className="h-9 w-12 shrink-0 rounded-sm" />
        <div className="min-w-0 flex-1"><p className="text-body-sm font-semibold text-fg-1">{map}</p>
          <p className="text-meta tracking-normal text-fg-3">{match.finished_at ? formatShortDate(match.finished_at) : '—'}</p>
        </div>
        <ResultBadge result={stats?.result} />
      </div>
      <dl className="mt-3 grid grid-cols-3 gap-2 border-y border-border-subtle py-3 text-meta tracking-normal tabular-nums">
        <div><dt className="text-fg-3">K / D / A</dt><dd className="mt-1 text-fg-1">{stats ? `${stats.kills} / ${stats.deaths} / ${stats.assists}` : '—'}</dd></div>
        <div><dt className="text-fg-3">K/D</dt><dd className="mt-1 text-fg-1">{stats?.kd_ratio?.toFixed(2) ?? '—'}</dd></div>
        <div><dt className="text-fg-3">ADR · HS</dt><dd className="mt-1 text-fg-1">{stats?.adr !== undefined ? Math.round(stats.adr) : '—'} · {stats?.headshots_percent !== undefined ? `${Math.round(stats.headshots_percent)}%` : '—'}</dd></div>
      </dl>
      <div className="mt-1 flex items-center justify-between gap-2">
        <span className="text-body-sm font-semibold tabular-nums text-fg-1">{score}</span>
        <Button asChild variant="link" size="sm" className="px-0 has-[>svg]:px-0">
          <a href={match.room_url} target="_blank" rel="noreferrer" aria-label={`Abrir sala FACEIT de ${map}`}>Abrir sala <ExternalLink aria-hidden className="size-3.5" /></a>
        </Button>
      </div>
    </li>
  );
}
