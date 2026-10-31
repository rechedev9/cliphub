'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Film } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { streamPlaybackItem } from '@/lib/api/playback';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import {
  activeJobCount,
  buildHubModel,
  hubPollClearsLoadError,
  hubPollUnchanged,
  hubTransitions,
  HUB_ROW_STAGE,
  isWorking,
  settleHubSnapshot,
  type HubModel,
  type HubSnapshot,
} from '@/lib/clips/hub';
import { queuedOutputCount, resultsActivityLabel } from '@/lib/clips/results';
import { ORPHAN_MATCH_SEGMENT, publishHref } from '@/lib/clips/routes';
import { createCoalescedCall } from '@/lib/coalesced-call';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { collectShellJobs, publishShellJobs } from '@/lib/shell-activity';
import { ClipsLens } from '@/components/clips-hub/clips-lens';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { useWebMCPLibrary } from '@/components/clips-hub/use-webmcp-library';
import { EmptyState } from '@/components/records/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Skeleton } from '@/components/ui/skeleton';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

const PAGE_TITLE = 'Mis vídeos';
const PAGE_DESCRIPTION = 'Todos tus vídeos, en preparación o listos para descargar.';

type LoadError = { offline: boolean };

async function fetchSnapshot(prev: HubSnapshot | null): Promise<HubSnapshot> {
  return settleHubSnapshot(
    await Promise.allSettled([api.listMatches(), api.listVideos(), streamsApi.listJobs()]),
    prev,
  );
}

function anyoneWorking(model: HubModel): boolean {
  if (model.rows.some((row) => row.stage === HUB_ROW_STAGE.parsing)) return true;
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

  const onChange = useCallback(() => {
    void refresh();
  }, [refresh]);

  useWebMCPLibrary({ model, failed: loadError !== null });

  const header = <StudioPageHeader title={PAGE_TITLE} description={PAGE_DESCRIPTION} />;
  // Same slot in every state: under the header, before anything that depends on the service.
  const banner = loadError === null ? null : <HubBanner offline={loadError.offline} onRetry={onChange} />;

  if (model === null) {
    if (loadError === null) return <ResultsSkeleton />;
    return (
      <div className="measure-list flex flex-col gap-6">
        {header}
        {banner}
      </div>
    );
  }

  if (model.clips.length + streamLibraryCount(streams) === 0) {
    return (
      <div className="measure-list flex flex-col gap-6">
        {header}
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
    <div className="measure-list flex flex-col gap-6">
      {header}
      {banner}
      <span role="status" className="self-end text-meta text-fg-3">
        {resultsActivityLabel({ running: activeJobCount(model, streams), queued: queuedOutputCount(model) })}
      </span>
      <div aria-busy={loadError !== null || undefined}>
        <ClipsLens clips={model.clips} streams={streams} onChange={onChange} />
      </div>
    </div>
  );
}

/** Same blocks and heights as the loaded page, so the first poll does not shift the list. */
function ResultsSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando tus vídeos" className="measure-list flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="measure-read h-5 w-full" />
      </div>
      <Skeleton className="h-4 w-32 self-end" />
      <div className="grid gap-3 @[48rem]/content:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-56 w-full" />
        ))}
      </div>
    </div>
  );
}
