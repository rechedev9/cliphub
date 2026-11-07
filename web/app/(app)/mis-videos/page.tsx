'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { ChevronDown, Film, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { streamPlaybackItem } from '@/lib/api/playback';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import {
  buildHubModel,
  OUTPUT_STATE,
  hubPollClearsLoadError,
  hubPollUnchanged,
  hubTransitions,
  HUB_ROW_STAGE,
  isWorking,
  settleHubSnapshot,
  type HubModel,
  type HubSnapshot,
} from '@/lib/clips/hub';
import { librarySummary, outputStateCount, streamActivity } from '@/lib/clips/results';
import { LONG_VIDEOS_HREF, ORPHAN_MATCH_SEGMENT, SHORTS_HREF, STREAMS_HREF, publishHref } from '@/lib/clips/routes';
import { createCoalescedCall } from '@/lib/coalesced-call';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { collectShellJobs, publishShellJobs } from '@/lib/shell-activity';
import { cn } from '@/lib/utils';
import { ClipsLens } from '@/components/clips-hub/clips-lens';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { useWebMCPLibrary } from '@/components/clips-hub/use-webmcp-library';
import { EmptyState } from '@/components/records/empty-state';
import { FORMAT_TONE, type FormatTone } from '@/components/studio/format-tone';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

const PAGE_TITLE = 'Mis vídeos';
const PAGE_DESCRIPTION = 'Todos tus vídeos, en preparación o listos para descargar.';
const NEW_VIDEO_OPTIONS = [
  { tone: 'short', label: 'Short', href: SHORTS_HREF },
  { tone: 'full', label: 'Demo completa', href: LONG_VIDEOS_HREF },
  { tone: 'stream', label: 'Clips de stream', href: STREAMS_HREF },
] as const satisfies ReadonlyArray<{ tone: FormatTone; label: string; href: string }>;

type LoadError = { offline: boolean };

async function fetchSnapshot(prev: HubSnapshot | null): Promise<HubSnapshot> {
  return settleHubSnapshot(
    await Promise.allSettled([api.listMatches(), api.listVideos(), streamsApi.listJobs()]),
    prev,
  );
}

function parsingDemoCount(model: HubModel): number {
  return model.rows.filter((row) => row.stage === HUB_ROW_STAGE.parsing).length;
}

function anyoneWorking(model: HubModel, streams: readonly StreamJob[]): boolean {
  if (parsingDemoCount(model) > 0 || streamActivity(streams).length > 0) return true;
  return model.clips.some((clip) => isWorking(clip.state));
}

function streamLibraryCount(streams: readonly StreamJob[]): number {
  return streams.reduce(
    (count, job) => count + (job.rendered_outputs ?? []).filter((output) => streamPlaybackItem(job, output) !== null).length,
    0,
  );
}

export default function ResultsPage(): ReactNode {
  const router = useRouter();

  const [model, setModel] = useState<HubModel | null>(null);
  const [streams, setStreams] = useState<StreamJob[]>([]);
  const [loadError, setLoadError] = useState<LoadError | null>(null);
  const modelRef = useRef<HubModel | null>(null);
  /** Last accepted poll; a rejected source falls back to it. */
  const snapshotRef = useRef<HubSnapshot | null>(null);
  // One poll at a time. A delete (or any other refresh) during that poll must
  // not be dropped, and the poll must not publish the list it started with.
  const loadSnapshot = useRef(createCoalescedCall(() => fetchSnapshot(snapshotRef.current)));

  const announceReady = useCallback(
    (prev: HubModel, next: HubModel) => {
      for (const clip of hubTransitions(prev, next).ready) {
        toast(`${clip.title} listo`, {
          description: `${prettyMapName(clip.video.map)} · Revisa el vídeo y descárgalo`,
          action: {
            label: 'Abrir',
            // The publishing page also previews and downloads outputs whose demo was deleted.
            onClick: () => router.push(publishHref(clip.match?.id ?? ORPHAN_MATCH_SEGMENT, clip.id)),
          },
        });
      }
    },
    [router],
  );

  const accept = useCallback((snapshot: HubSnapshot) => {
    const next = buildHubModel(snapshot.matches, snapshot.videos);
    if (hubPollUnchanged(snapshotRef.current, modelRef.current, next, snapshot)) {
      if (hubPollClearsLoadError(snapshot)) {
        publishShellJobs(collectShellJobs(snapshot), Date.now());
        setLoadError(null);
      }
      return modelRef.current ?? next;
    }
    if (modelRef.current !== null) announceReady(modelRef.current, next);
    modelRef.current = next;
    snapshotRef.current = snapshot;
    setModel(next);
    setStreams(snapshot.streams);
    setLoadError(snapshot.failure === null ? null : { offline: isDemoServiceUnavailable(snapshot.failure) });
    // A partial poll carries stale sources; the shell monitor fetches for itself instead.
    if (snapshot.failure === null) publishShellJobs(collectShellJobs(snapshot), Date.now());
    return next;
  }, [announceReady]);

  const refresh = useCallback(async (): Promise<HubModel | null> => {
    try {
      return accept(await loadSnapshot.current());
    } catch (err) {
      setLoadError({ offline: isDemoServiceUnavailable(err) });
      return null;
    }
  }, [accept]);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        const next = await refresh();
        if (!active) return 'idle';
        return next === null || anyoneWorking(next, snapshotRef.current?.streams ?? []) ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [refresh]);

  const onChange = useCallback(() => {
    void refresh();
  }, [refresh]);

  useWebMCPLibrary({ model, failed: loadError !== null });

  const banner = loadError === null ? null : <HubBanner offline={loadError.offline} onRetry={onChange} />;

  if (model === null) {
    if (loadError === null) return <ResultsSkeleton />;
    return (
      <div className="measure-list flex flex-col gap-6">
        <LibraryHeader summary={PAGE_DESCRIPTION} />
        {banner}
      </div>
    );
  }

  const streamWork = streamActivity(streams);
  const analysing = parsingDemoCount(model);
  const summary = librarySummary({
    ready: outputStateCount(model, OUTPUT_STATE.ready) + streamLibraryCount(streams),
    recording: outputStateCount(model, OUTPUT_STATE.rec),
    working: outputStateCount(model, OUTPUT_STATE.render),
    queued: outputStateCount(model, OUTPUT_STATE.queue),
    failed: outputStateCount(model, OUTPUT_STATE.failed),
    analysing,
    streamsDownloading: streamWork.filter((activity) => activity.stage === 'download').length,
    streamsExporting: streamWork.filter((activity) => activity.stage === 'export').length,
  });

  // A stream project at work has a row below, so it never falls into the empty state.
  if (model.clips.length + streamLibraryCount(streams) + streamWork.length === 0) {
    return (
      <div className="measure-list flex flex-col gap-6">
        <LibraryHeader summary={analysing > 0 ? summary : PAGE_DESCRIPTION} live={analysing > 0} />
        {banner}
        <EmptyState
          icon={Film}
          title="Aún no hay vídeos"
          description="Los Shorts, demos completas y clips de stream que crees aparecerán aquí, ordenados por fecha."
        />
      </div>
    );
  }

  return (
    <div className="measure-list flex flex-col gap-8">
      <LibraryHeader summary={summary} live />
      {banner}
      <div aria-busy={loadError !== null || undefined}>
        <ClipsLens clips={model.clips} streams={streams} onChange={onChange} />
      </div>
    </div>
  );
}

/** Big title, the one-line count and the Nuevo vídeo menu. `live` announces count changes. */
function LibraryHeader({ summary, live = false }: { summary: string; live?: boolean }): ReactNode {
  return (
    <header className="flex flex-wrap items-end gap-4">
      <div className="flex min-w-0 flex-[1_1_22rem] flex-col gap-2">
        <h1 className="font-display text-display font-extrabold text-fg-1">{PAGE_TITLE}</h1>
        <p role={live ? 'status' : undefined} className="text-body text-fg-2">
          {summary}
        </p>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button">
            <Plus aria-hidden />
            Nuevo vídeo
            <ChevronDown aria-hidden />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-48">
          {NEW_VIDEO_OPTIONS.map((option) => (
            <DropdownMenuItem key={option.href} asChild>
              <Link href={option.href}>
                <span aria-hidden className={cn('size-2.5 rounded-[3px]', FORMAT_TONE[option.tone].bg)} />
                {option.label}
              </Link>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  );
}

/** Same blocks and heights as the loaded page, so the first poll does not shift the list. */
function ResultsSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando tus vídeos" className="measure-list flex flex-col gap-8">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="measure-read h-5 w-full" />
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(17rem,1fr))] gap-4.5">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-[22rem] w-full rounded-[18px]" />
        ))}
      </div>
    </div>
  );
}
