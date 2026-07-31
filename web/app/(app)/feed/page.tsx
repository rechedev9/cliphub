'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Compass, Film, PlugZap, RefreshCw, UploadCloud } from 'lucide-react';
import { api } from '@/lib/api';
import type { FeedItem } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { FeedGrid, FeedGridSkeleton } from '@/components/feed/feed-grid';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';


/** RECIENTES sorts by publish time; TOP SEMANA sorts the last 7 days by likes
 * (falling back to the full list when nothing falls in that window, so a
 * short-lived seed/mock dataset never renders an empty grid). */
const FEED_SORTS = [
  { value: 'recent', label: 'Recientes', description: 'Más recientes' },
  { value: 'top-week', label: 'Top semana', description: 'Top de la semana' },
] as const;

type FeedSort = (typeof FEED_SORTS)[number]['value'];

function isFeedSort(value: string): value is FeedSort {
  return FEED_SORTS.some((entry) => entry.value === value);
}

const WEEK_MS = 7 * 24 * 60 * 60 * 1000;

function sortFeed(items: FeedItem[], sort: FeedSort): FeedItem[] {
  if (sort === 'recent') {
    return [...items].sort((a, b) => b.createdAt - a.createdAt);
  }
  const cutoff = Date.now() - WEEK_MS;
  const thisWeek = items.filter((item) => item.createdAt >= cutoff);
  const pool = thisWeek.length > 0 ? thisWeek : items;
  return [...pool].sort((a, b) => b.likes - a.likes);
}

/** True when an API error means the local analysis service is unreachable. */
function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

export default function FeedPage() {
  const [items, setItems] = useState<FeedItem[] | null>(null);
  const [loadError, setLoadError] = useState<{ offline: boolean } | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [sort, setSort] = useState<FeedSort>('recent');

  useEffect(() => {
    let active = true;
    // The rejection handler is the point: without it an offline orchestrator
    // left `items` null forever and the skeleton animated for as long as the tab
    // stayed open, with the failure swallowed.
    api.listFeed().then(
      (feed) => {
        if (!active) return;
        setItems(feed);
        setLoadError(null);
      },
      (err: unknown) => {
        if (active) setLoadError({ offline: isServiceUnavailable(err) });
      },
    );
    return () => {
      active = false;
    };
  }, [attempt]);

  const retry = useCallback(() => {
    setItems(null);
    setLoadError(null);
    setAttempt((n) => n + 1);
  }, []);

  const visible = useMemo(() => sortFeed(items ?? [], sort), [items, sort]);

  let content: ReactNode;
  if (loadError !== null) {
    content = <FeedUnavailable offline={loadError.offline} onRetry={retry} />;
  } else if (items === null) {
    content = <FeedGridSkeleton />;
  } else if (items.length === 0) {
    content = <FeedEmptyState />;
  } else {
    content = <FeedGrid items={visible} showRank={sort === 'top-week'} />;
  }

  return (
    <div className="flex flex-col gap-8 sm:gap-10">
      <StudioPageHeader
        title="LA COMUNIDAD FORJA"
        description="Reels forjados en los rigs de la comunidad. Mira uno, deja un like."
        actions={
          items !== null && items.length > 0 ? (
            <div className="-mx-1 max-w-full overflow-x-auto px-1 pb-1">
              <ToggleGroup
                type="single"
                variant="filter"
                value={sort}
                onValueChange={(value) => {
                  if (isFeedSort(value)) setSort(value);
                }}
                aria-label="Ordenar feed"
              >
                {FEED_SORTS.map((entry) => (
                  <ToggleGroupItem key={entry.value} value={entry.value} aria-label={entry.description}>
                    {entry.label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </div>
          ) : null
        }
      />

      {content}
    </div>
  );
}

/**
 * `api.listFeed()` rejected. The community feed is a remote surface, so an
 * unreachable orchestrator is the expected failure and gets its own copy.
 */
function FeedUnavailable({ offline, onRetry }: { offline: boolean; onRetry: () => void }) {
  return (
    <div role="alert">
      <StudioEmptyState
        icon={PlugZap}
        title={offline ? 'Servicio local sin responder' : 'No se pudo cargar el feed'}
        description={
          offline
            ? 'TickCut no ha podido contactar con el servicio local para traer el feed. Arráncalo y reintenta.'
            : 'El feed de la comunidad no respondió. Reintenta en un momento.'
        }
        accent="magenta"
        compact
        actions={
          <Button type="button" variant="outline-primary" onClick={onRetry}>
            <RefreshCw aria-hidden />
            REINTENTAR
          </Button>
        }
      />
    </div>
  );
}

function FeedEmptyState() {
  return (
    <StudioEmptyState
      icon={Compass}
      title="Todavía no hay nada publicado"
      description="Sé el primero en publicar un highlight — tus reels aparecerán aquí para todos."
      accent="magenta"
      compact
      actions={
        <>
          <Button asChild variant="hero">
            <Link href="/videos">
              <Film aria-hidden />
              PUBLICAR UN REEL
            </Link>
          </Button>
          <Button asChild variant="outline" className="font-display tracking-wide uppercase">
            <Link href="/upload">
              <UploadCloud aria-hidden />
              CREAR UN REEL
            </Link>
          </Button>
        </>
      }
    />
  );
}
