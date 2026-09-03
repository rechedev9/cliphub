'use client';

import { Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import type { Match, Video } from '@/lib/api/types';
import { activeJobCount, buildHubModel, hubTransitions, isWorking, type HubModel } from '@/lib/clips/hub';
import { HUB_LENS, HUB_QUERY, hubHref, isHubLens, type HubLens } from '@/lib/clips/routes';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { collectShellJobs, publishShellJobs } from '@/lib/shell-activity';
import { Skeleton } from '@/components/ui/skeleton';
import { ClipsLens } from '@/components/clips-hub/clips-lens';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { HubEmpty } from '@/components/clips-hub/hub-empty';
import { LensToggle } from '@/components/clips-hub/lens-toggle';
import { MatchRow, matchRowId } from '@/components/clips-hub/match-row';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

type LoadError = { offline: boolean };

type Snapshot = { matches: Match[]; videos: Video[]; streams: StreamJob[]; failure: unknown };

/** One source failing keeps the other on screen; both failing throws. */
async function fetchSnapshot(): Promise<Snapshot> {
  const [matches, videos, streams] = await Promise.allSettled([
    api.listMatches(),
    api.listVideos(),
    streamsApi.listJobs(),
  ]);
  if (matches.status === 'rejected' && videos.status === 'rejected') throw matches.reason;
  let failure: unknown = null;
  if (matches.status === 'rejected') failure = matches.reason;
  else if (videos.status === 'rejected') failure = videos.reason;
  else if (streams.status === 'rejected') failure = streams.reason;
  return {
    matches: matches.status === 'fulfilled' ? matches.value : [],
    videos: videos.status === 'fulfilled' ? videos.value : [],
    streams: streams.status === 'fulfilled' ? streams.value : [],
    failure,
  };
}

function anyoneWorking(model: HubModel): boolean {
  if (model.rows.some((row) => row.parsing)) return true;
  return model.clips.some((clip) => isWorking(clip.state));
}

function announceTransitions(prev: HubModel, next: HubModel): void {
  const changes = hubTransitions(prev, next);
  for (const row of changes.parsed) {
    toast('Partida parseada', {
      description: [
        prettyMapName(row.match.map),
        row.match.decentPlays > 0 ? `${row.match.decentPlays} highlights` : null,
        `POV de ${row.match.player ?? '—'}`,
      ]
        .filter(Boolean)
        .join(' · '),
    });
  }
  for (const clip of changes.ready) {
    toast(`${clip.title} listo`, { description: `${prettyMapName(clip.video.map)} · MP4 y portada en la fila` });
  }
}

export default function ClipsHubPage(): ReactNode {
  return (
    <Suspense fallback={<HubSkeleton />}>
      <ClipsHub />
    </Suspense>
  );
}

function ClipsHub(): ReactNode {
  const router = useRouter();
  const searchParams = useSearchParams();
  const lensParam = searchParams.get(HUB_QUERY.lens);
  const lens: HubLens = isHubLens(lensParam) ? lensParam : HUB_LENS.matches;
  const open = searchParams.get(HUB_QUERY.open);

  const [model, setModel] = useState<HubModel | null>(null);
  const [streams, setStreams] = useState<StreamJob[]>([]);
  const [loadError, setLoadError] = useState<LoadError | null>(null);
  const modelRef = useRef<HubModel | null>(null);
  const scrolledTo = useRef<string | null>(null);
  const inFlight = useRef(false);

  const accept = useCallback((snapshot: Snapshot) => {
    const next = buildHubModel(snapshot.matches, snapshot.videos);
    if (modelRef.current !== null) announceTransitions(modelRef.current, next);
    modelRef.current = next;
    setModel(next);
    setStreams(snapshot.streams);
    setLoadError(snapshot.failure === null ? null : { offline: isDemoServiceUnavailable(snapshot.failure) });
    publishShellJobs(collectShellJobs(snapshot), Date.now());
    return next;
  }, []);

  const refresh = useCallback(async (): Promise<HubModel | null> => {
    if (inFlight.current) return null;
    inFlight.current = true;
    try {
      return accept(await fetchSnapshot());
    } catch (err) {
      setLoadError({ offline: isDemoServiceUnavailable(err) });
      return null;
    } finally {
      inFlight.current = false;
    }
  }, [accept]);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        const next = await refresh();
        if (!active) return 'idle';
        return next === null || anyoneWorking(next) ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [refresh]);

  useEffect(() => {
    if (open === null || model === null || scrolledTo.current === open) return;
    const el = document.getElementById(matchRowId(open));
    if (el === null) return;
    scrolledTo.current = open;
    el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, [open, model]);

  const navigate = useCallback(
    (next: { lens?: HubLens; open?: string }) => {
      router.replace(hubHref(next), { scroll: false });
    },
    [router],
  );

  const onChange = useCallback(() => {
    void refresh();
  }, [refresh]);

  if (model === null) {
    if (loadError === null) return <HubSkeleton />;
    return (
      <div className="flex max-w-[1080px] flex-col gap-3">
        <HubBanner offline={loadError.offline} onRetry={onChange} />
        <HubEmpty />
      </div>
    );
  }

  if (model.rows.length === 0 && model.clips.length === 0) {
    return (
      <div className="flex max-w-[1080px] flex-col gap-3">
        {loadError !== null ? <HubBanner offline={loadError.offline} onRetry={onChange} /> : null}
        <HubEmpty />
      </div>
    );
  }

  const counts: Record<HubLens, number> = { partidas: model.rows.length, clips: model.clips.length };
  const jobs = activeJobCount(model, streams);

  return (
    <div className="studio-enter flex max-w-[1080px] flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <LensToggle lens={lens} counts={counts} />
        <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
          {model.rows.length} {model.rows.length === 1 ? 'partida' : 'partidas'} · {jobs}{' '}
          {jobs === 1 ? 'trabajo en marcha' : 'trabajos en marcha'}
        </span>
      </div>

      {loadError !== null ? <HubBanner offline={loadError.offline} onRetry={onChange} /> : null}

      {lens === HUB_LENS.clips ? (
        <ClipsLens
          clips={model.clips}
          onChange={onChange}
          onOpenMatch={(matchId) => {
            scrolledTo.current = null;
            navigate({ lens: HUB_LENS.matches, open: matchId });
          }}
        />
      ) : (
        <div className="flex flex-col gap-3" aria-busy={loadError !== null || undefined}>
          {model.rows.map((row) => (
            <MatchRow
              key={row.match.id}
              row={row}
              open={open === row.match.id}
              onToggle={() => navigate(open === row.match.id ? {} : { open: row.match.id })}
              onChange={onChange}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function HubSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando partidas" className="flex max-w-[1080px] flex-col gap-3">
      <Skeleton className="h-9 w-56" />
      {Array.from({ length: 3 }).map((_, index) => (
        <Skeleton key={index} className="h-[72px] w-full rounded-[10px]" />
      ))}
    </div>
  );
}
