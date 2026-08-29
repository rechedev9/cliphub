'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { AlertTriangle, Film, PlugZap, RefreshCw, Swords } from 'lucide-react';
import type { Video } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { api } from '@/lib/api';
import { startPollLoop } from '@/lib/poll-loop';
import { publishShellActivity } from '@/lib/shell-activity';
import { isLandscapeRecap } from '@/lib/reel-brief';
import { FULL_DEMO_HREF } from '@/lib/full-demo';
import { IconTile } from '@/components/studio/icon-tile';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { RenderingCard } from '@/components/videos/rendering-card';
import { ReadyCard } from '@/components/videos/ready-card';
import { FailedCard } from '@/components/videos/failed-card';
import { VideoFilters, type VideoFormatFilter } from '@/components/videos/video-filters';
import { cn } from '@/lib/utils';


// Poll active reels quickly, then back off once every reel is terminal.
// A newly created reel resumes fast polling on the next tick.
const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

// Auto-fill keeps cards useful from narrow windows to 1920px; items-start
// preserves each reel's real 9:16 or 16:9 height.
const REEL_GRID_CLASS =
  'grid grid-cols-[repeat(auto-fill,minmax(min(100%,15.5rem),1fr))] items-start gap-5';

/** True when an API error means the local analysis service is unreachable. */
function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

function hasActiveReel(list: Video[] | undefined): boolean {
  return !!list && list.some(
    (v) => v.status !== 'ready' && v.status !== 'review_required' && v.status !== 'failed',
  );
}

function matchesFormat(video: Video, filter: VideoFormatFilter): boolean {
  if (filter === 'all') return true;
  if (filter === 'full-demo') {
    return video.editConfig != null && isLandscapeRecap(video.editConfig);
  }
  return video.editConfig?.format === filter;
}

// Failures need a decision, so stable-sort them first without reordering the rest.
function byAttentionFirst(a: Video, b: Video): number {
  return Number(b.status === 'failed') - Number(a.status === 'failed');
}

type LoadError = { offline: boolean };

export function VideosPageClient() {
  const searchParams = useSearchParams();
  const nuevoId = searchParams.get('nuevo');
  const [videos, setVideos] = useState<Video[] | null>(null);
  const [loadError, setLoadError] = useState<LoadError | null>(null);
  const [filter, setFilter] = useState<VideoFormatFilter>('all');
  const highlighted = useRef<string | null>(null);

  // Avoid overlapping listVideos() calls when a manual refresh races a poll.
  const inFlight = useRef(false);

  const reload = useCallback(async (): Promise<Video[] | undefined> => {
    if (inFlight.current) return undefined;
    inFlight.current = true;
    try {
      return await api.listVideos();
    } finally {
      inFlight.current = false;
    }
  }, []);

  // Reuse the grid poll to tell the shell when capture should suspend ambient effects.
  const accept = useCallback((next: Video[]) => {
    setVideos(next);
    setLoadError(null);
    publishShellActivity(next, Date.now());
  }, []);

  const refresh = useCallback(async () => {
    try {
      const next = await reload();
      if (next) accept(next);
    } catch (err) {
      setLoadError({ offline: isServiceUnavailable(err) });
    }
  }, [accept, reload]);

  useEffect(() => {
    let active = true;

    // A rejected tick records the offline state without killing the poll loop.
    const stop = startPollLoop({
      tick: async () => {
        try {
          const next = await reload();
          if (!active) return 'idle';
          if (next) accept(next);
          // `next` is undefined only if a manual refresh raced this tick; treat
          // that as "keep polling fast" so a just-created reel is never stranded.
          return next === undefined || hasActiveReel(next) ? 'fast' : 'idle';
        } catch (err) {
          if (active) setLoadError({ offline: isServiceUnavailable(err) });
          return 'idle';
        }
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });

    return () => {
      active = false;
      stop();
    };
  }, [accept, reload]);

  useEffect(() => {
    if (!nuevoId || !videos || highlighted.current === nuevoId) return;
    const el = document.getElementById(`reel-${nuevoId}`);
    if (!el) return;
    highlighted.current = nuevoId;
    el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, [nuevoId, videos]);

  const visible = useMemo(
    () => (videos ?? []).filter((v) => matchesFormat(v, filter)).sort(byAttentionFirst),
    [videos, filter],
  );

  let content: ReactNode;
  if (videos === null && loadError !== null) {
    content = <LibraryUnavailable offline={loadError.offline} onRetry={() => void refresh()} />;
  } else if (videos === null) {
    content = <LibrarySkeleton />;
  } else if (videos.length === 0) {
    content = <EmptyState />;
  } else {
    content = (
      <LibraryGrid videos={visible} onChange={() => void refresh()} highlightId={nuevoId} />
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="BIBLIOTECA"
        description="Sigue cada captura desde la cola hasta el MP4: Shorts y partidas completas comparten el mismo estado local."
        actions={
          videos !== null && videos.length > 0 ? (
            <VideoFilters filter={filter} onFilterChange={setFilter} />
          ) : undefined
        }
      />

      {/* A failed tick on top of data already on screen is not a dead end: the
          cards stay, and the strip says why they stopped moving. */}
      {videos !== null && loadError !== null ? (
        <StaleDataNotice offline={loadError.offline} onRetry={() => void refresh()} />
      ) : null}

      {content}
    </div>
  );
}

// Every stage uses the same card grid; failures carry their own destructive treatment.
function LibraryGrid({
  videos,
  onChange,
  highlightId,
}: {
  videos: Video[];
  onChange(): void;
  highlightId: string | null;
}) {
  const failedCount = videos.filter((v) => v.status === 'failed').length;

  if (videos.length === 0) return <FilteredEmpty />;

  return (
    <div className="flex flex-col gap-5">
      {failedCount > 0 ? <AttentionStrip count={failedCount} /> : null}

      <section className={REEL_GRID_CLASS} aria-label="Vídeos">
        {videos.map((v) => {
          const flash = highlightId === v.id;
          let card: ReactNode;
          if (v.status === 'failed') {
            card = <FailedCard video={v} onChange={onChange} />;
          } else if (v.status === 'ready' || v.status === 'review_required') {
            card = <ReadyCard video={v} onChange={onChange} />;
          } else {
            card = <RenderingCard video={v} />;
          }
          return (
            <div
              key={v.id}
              id={`reel-${v.id}`}
              className={cn(flash && 'studio-reveal ring-2 ring-primary ring-offset-2 ring-offset-surface-1')}
            >
              {card}
            </div>
          );
        })}
      </section>
    </div>
  );
}

/** Count only: "reintentar todo" would be a behaviour change, not a redesign. */
function AttentionStrip({ count }: { count: number }) {
  return (
    <div
      role="status"
      className="studio-panel flex items-center gap-3 border-destructive/45 px-4 py-3"
    >
      <IconTile icon={AlertTriangle} size="sm" tone="danger" depth="inset" />
      <p className="min-w-0 text-body-sm text-fg-2">
        <span className="font-mono text-body-lg tabular-nums text-destructive">{count}</span>{' '}
        {count === 1 ? 'vídeo necesita atención' : 'vídeos necesitan atención'}.{' '}
        {count === 1 ? 'Resuélvelo' : 'Resuélvelos'} desde {count === 1 ? 'su tarjeta' : 'sus tarjetas'}.
      </p>
    </div>
  );
}

function FilteredEmpty() {
  return (
    <div className="studio-panel flex max-w-xl items-center gap-4 px-5 py-4" role="status">
      <IconTile icon={Film} size="sm" tone="primary" depth="inset" />
      <div className="min-w-0">
        <p className="font-display text-label font-bold uppercase text-fg-1">
          No hay vídeos en este formato
        </p>
        <p className="mt-1 text-body-sm text-fg-2">
          Cambia el filtro para volver a ver el resto de la biblioteca.
        </p>
      </div>
    </div>
  );
}

// A first-load failure must replace the otherwise permanent loading skeleton.
function LibraryUnavailable({ offline, onRetry }: { offline: boolean; onRetry: () => void }) {
  return (
    <div role="alert">
      <StudioEmptyState
        icon={PlugZap}
        title={offline ? 'Servicio local sin responder' : 'No se pudo cargar la biblioteca'}
        description={
          offline
            ? 'ClipHub no ha podido contactar con el servicio de análisis local. Arráncalo y reintenta: tus reels siguen en el disco.'
            : 'La biblioteca no respondió. Reintenta; si sigue fallando, revisa el servicio local.'
        }
        compact
        actions={
          <Button type="button" variant="outline-primary" onClick={onRetry}>
            <RefreshCw aria-hidden />
            REINTENTAR
          </Button>
        }
        note="LA BIBLIOTECA SE RECUPERA SOLA EN CUANTO EL SERVICIO VUELVE"
      />
    </div>
  );
}

/** Data on screen, but the last tick failed. Keep the cards; say they are stale. */
function StaleDataNotice({ offline, onRetry }: { offline: boolean; onRetry: () => void }) {
  return (
    <div
      role="status"
      className="studio-panel flex flex-wrap items-center gap-3 border-warning/45 px-4 py-3"
    >
      <IconTile icon={PlugZap} size="sm" tone="warning" depth="inset" />
      <p className="min-w-0 flex-1 text-body-sm text-fg-2">
        {offline
          ? 'Servicio local sin responder: estas tarjetas pueden estar desactualizadas.'
          : 'La última actualización falló: estas tarjetas pueden estar desactualizadas.'}
      </p>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        <RefreshCw aria-hidden />
        REINTENTAR
      </Button>
    </div>
  );
}

// Mirror the default 9:16 card so the common case does not jump when data lands.
function LibrarySkeleton() {
  return (
    <div className={REEL_GRID_CLASS} role="status" aria-label="Cargando biblioteca">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="studio-panel flex flex-col overflow-hidden">
          <Skeleton className="aspect-[9/16] w-full rounded-none" />
          <div className="flex flex-col gap-3 p-4">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-3 w-1/2" />
            <Skeleton className="h-7 w-24" />
          </div>
          <Skeleton className="h-9 w-full rounded-none" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <StudioEmptyState
      icon={Film}
      title="Todavía no hay vídeos"
      description="Forja Shorts desde Partidas o captura una demo completa 16:9; ClipHub sigue captura y render aquí."
      compact
      actions={
        <>
          <Button asChild variant="hero">
            <Link href="/matches">
              <Swords aria-hidden />
              BUSCAR JUGADAS
            </Link>
          </Button>
          <Button asChild variant="outline-primary">
            <Link href={FULL_DEMO_HREF}>DEMO COMPLETA 16:9</Link>
          </Button>
        </>
      }
      note="CAPTURA Y EDICIÓN EN TU RIG"
    />
  );
}
