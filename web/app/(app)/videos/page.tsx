'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { AlertTriangle, Film, PlugZap, RefreshCw, Swords } from 'lucide-react';
import type { Video } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { api } from '@/lib/api';
import { startPollLoop } from '@/lib/poll-loop';
import { publishShellActivity } from '@/lib/shell-activity';
import { IconTile } from '@/components/studio/icon-tile';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { RenderingCard } from '@/components/videos/rendering-card';
import { ReadyCard } from '@/components/videos/ready-card';
import { FailedCard } from '@/components/videos/failed-card';
import { VideoFilters, type VideoFormatFilter } from '@/components/videos/video-filters';


// Poll fast while a reel is advancing through the pipeline; once every reel is
// terminal (ready/failed) there is nothing to drive, so back off to an idle
// cadence to stop hammering the orchestrator. A newly created reel resumes fast
// polling on the next tick.
const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

/**
 * One auto-fill grid for every reel, at every stage. The minimum track is the
 * narrowest card the four-segment stage track can hold without wrapping a label,
 * and the maximum is `1fr`, not a fixed 300px: capping it stranded 244px of dead
 * space at 1920 next to a `justify-start` row. Because the cards are now as tall
 * as their real format, the min is kept tight so a wide workspace answers with
 * more columns instead of taller 9:16 covers.
 *
 * `items-start` because a card's shape now follows the reel's render format, so
 * a 9:16 short and a 16:9 landscape genuinely are different heights and
 * stretching the shorter one would only open a void under its actions.
 */
const REEL_GRID_CLASS =
  'grid grid-cols-[repeat(auto-fill,minmax(min(100%,15.5rem),1fr))] items-start gap-5';

/** True when an API error means the local analysis service is unreachable. */
function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

function hasActiveReel(list: Video[] | undefined): boolean {
  return !!list && list.some((v) => v.status !== 'ready' && v.status !== 'failed');
}

function matchesFormat(video: Video, filter: VideoFormatFilter): boolean {
  if (filter === 'all') return true;
  return video.editConfig?.format === filter;
}

/**
 * Failures first: they are the only cards in the grid that need a decision.
 * `Array.prototype.sort` is stable, so everything else keeps the API's
 * newest-first order.
 */
function byAttentionFirst(a: Video, b: Video): number {
  return Number(b.status === 'failed') - Number(a.status === 'failed');
}

type LoadError = { offline: boolean };

export default function VideosPage() {
  const [videos, setVideos] = useState<Video[] | null>(null);
  const [loadError, setLoadError] = useState<LoadError | null>(null);
  const [filter, setFilter] = useState<VideoFormatFilter>('all');

  // Guards against overlapping listVideos() calls if a manual refresh is still
  // in flight when the next poll tick fires (the poll loop itself never overlaps
  // its own ticks).
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

  /**
   * The one place fresh reels land. It also pushes them to the shell: the same
   * 1.5s loop that drives this grid tells the chrome what the machine is doing,
   * so `html[data-capture-active]` stands the ambient GPU effects down while
   * HLAE/CS2 owns the card — instead of a second poll loop doing it again.
   */
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

    // A tick that throws (transient proxy/orchestrator hiccup) must not kill the
    // loop and must not strand the page on a skeleton either: the rejection is
    // recorded so the screen can say "offline" instead of pretending to load
    // forever, and the cadence still backs off to idle exactly as before.
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
    content = <LibraryGrid videos={visible} onChange={() => void refresh()} />;
  }

  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="TUS REELS"
        description="Sigue cada captura desde la cola hasta el MP4 y publica solo lo que merece salir del rig."
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

/**
 * Every reel, at every stage, in one grid. Queued/capturing/editing/ready/failed
 * cards sit side by side at equal width because each one already carries its own
 * stage treatment — an edge tone, a stage indicator over the cover and a filled
 * segment on its instrument strip — so a stage-grouping header on top of that
 * would repeat what the card already says.
 *
 * Failures used to live in a separate stack above the grid as full-width
 * horizontal rows, i.e. a second card system on the same page. They are now the
 * same tile with a destructive edge, sorted to the front.
 */
function LibraryGrid({ videos, onChange }: { videos: Video[]; onChange(): void }) {
  const failedCount = videos.filter((v) => v.status === 'failed').length;

  if (videos.length === 0) return <FilteredEmpty />;

  return (
    <div className="flex flex-col gap-5">
      {failedCount > 0 ? <AttentionStrip count={failedCount} /> : null}

      <section className={REEL_GRID_CLASS} aria-label="Reels">
        {videos.map((v) => {
          if (v.status === 'failed') return <FailedCard key={v.id} video={v} onChange={onChange} />;
          if (v.status === 'ready') return <ReadyCard key={v.id} video={v} onDeleted={onChange} />;
          return <RenderingCard key={v.id} video={v} />;
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
        {count === 1 ? 'reel necesita atención' : 'reels necesitan atención'}. Reintenta o elimínalos
        desde su tarjeta.
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
          No hay reels en este formato
        </p>
        <p className="mt-1 text-body-sm text-fg-2">
          Cambia el filtro para volver a ver el resto de la biblioteca.
        </p>
      </div>
    </div>
  );
}

/**
 * A poll tick rejected before any reel ever arrived. The old page had no branch
 * for this at all: `videos` stayed null, the skeleton animated forever and the
 * rejection was swallowed.
 */
function LibraryUnavailable({ offline, onRetry }: { offline: boolean; onRetry: () => void }) {
  return (
    <div role="alert">
      <StudioEmptyState
        icon={PlugZap}
        title={offline ? 'Servicio local sin responder' : 'No se pudo cargar la biblioteca'}
        description={
          offline
            ? 'FragForge no ha podido contactar con el servicio de análisis local. Arráncalo y reintenta: tus reels siguen en el disco.'
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

/**
 * The placeholder mirrors the real card: media flush to the panel edges, then a
 * title, a meta line and the instrument strip. It draws a 9:16 frame because
 * that is the default render format and the product's default deliverable is a
 * vertical reel, so the common case does not jump when the data lands.
 */
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
      title="Todavía no hay reels"
      description="Elige una jugada y FragForge seguirá la captura, la edición y el render desde esta biblioteca."
      compact
      actions={
        <Button asChild variant="hero">
          <Link href="/matches">
            <Swords aria-hidden />
            BUSCAR JUGADAS
          </Link>
        </Button>
      }
      note="CAPTURA Y EDICIÓN EN TU RIG"
    />
  );
}
