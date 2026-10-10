'use client';

import { Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { createCoalescedCall } from '@/lib/coalesced-call';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { cloudApi } from '@/lib/api/cloud';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import { streamPlaybackItem } from '@/lib/api/playback';
import { HUB_ORPHANS_HINT, HUB_ORPHANS_TITLE } from '@/lib/clips/copy';
import {
  activeJobCount,
  buildHubModel,
  dismissFirstRunGuide,
  firstRunComplete,
  firstRunGuideDismissed,
  firstRunProgress,
  HUB_ROW_STAGE,
  hubTransitions,
  isWorking,
  sameHubProps,
  hubPollClearsLoadError,
  hubPollUnchanged,
  settleHubSnapshot,
  type HubModel,
  type HubSnapshot,
} from '@/lib/clips/hub';
import { cloudAccountSnapshot } from '@/lib/cloud/account-store';
import { cloudDeviceLinked } from '@/lib/cloud/account-view';
import type { CloudJob } from '@/lib/cloud/parse';
import { anyCloudJobActive, cloudJobsByMatch, cloudJobsNewlyReady, type CloudDevice } from '@/lib/cloud/view';
import { HUB_LENS, HUB_QUERY, hubHref, isHubLens, ORPHAN_MATCH_SEGMENT, publishHref, type HubLens } from '@/lib/clips/routes';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { collectShellJobs, publishShellJobs } from '@/lib/shell-activity';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { Skeleton } from '@/components/ui/skeleton';
import { ClipsLens } from '@/components/clips-hub/clips-lens';
import { CloudOutputItem } from '@/components/clips-hub/cloud-output-item';
import { FirstRunGuide } from '@/components/clips-hub/first-run-guide';
import { CreationPaths } from '@/components/studio/creation-paths';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { HubEmpty } from '@/components/clips-hub/hub-empty';
import { HubHeader } from '@/components/clips-hub/hub-header';
import { LensToggle } from '@/components/clips-hub/lens-toggle';
import { MatchRow, matchRowId } from '@/components/clips-hub/match-row';
import { OutputItem } from '@/components/clips-hub/output-item';
import { useWebMCPLibrary } from '@/components/clips-hub/use-webmcp-library';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

type LoadError = { offline: boolean };

const NO_CLOUD_JOBS: readonly CloudJob[] = [];
const CLOUD_SECTION_TITLE = 'En la nube';

async function fetchSnapshot(prev: HubSnapshot | null): Promise<HubSnapshot> {
  return settleHubSnapshot(
    await Promise.allSettled([api.listMatches(), api.listVideos(), streamsApi.listJobs()]),
    prev,
  );
}

/** This PC as cloud jobs see it, read at call time because the polls outlive a render. */
function cloudDevice(): CloudDevice {
  return { now: Date.now(), linked: cloudDeviceLinked(cloudAccountSnapshot()) };
}

function cloudShell(cloud: readonly CloudJob[]): { cloud: readonly CloudJob[]; cloudLinked: boolean } {
  return { cloud, cloudLinked: cloudDevice().linked };
}

function anyoneWorking(model: HubModel): boolean {
  if (model.rows.some((row) => row.stage === HUB_ROW_STAGE.parsing)) return true;
  return model.clips.some((clip) => isWorking(clip.state));
}

function browserStorage(): Storage | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

function streamLibraryCount(streams: readonly StreamJob[]): number {
  return streams.reduce(
    (count, job) => count + (job.rendered_outputs ?? []).filter((output) => streamPlaybackItem(job, output) !== null).length,
    0,
  );
}

type HubDestination = { matchId: string } | { clipId: string };

function announceTransitions(prev: HubModel, next: HubModel, openResult: (destination: HubDestination) => void): void {
  const changes = hubTransitions(prev, next);
  for (const row of changes.parsed) {
    toast('Partida analizada', {
      action: { label: 'Abrir', onClick: () => openResult({ matchId: row.match.id }) },
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
    toast(`${clip.title} listo`, {
      description: `${prettyMapName(clip.video.map)} · Revisa el vídeo y descárgalo`,
      action: { label: 'Abrir', onClick: () => openResult(clip.match ? { matchId: clip.match.id } : { clipId: clip.id }) },
    });
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
  const [cloudJobs, setCloudJobs] = useState<readonly CloudJob[]>(NO_CLOUD_JOBS);
  const cloudLinked = cloudDeviceLinked(useCloudAccount());
  /** Last cloud poll, for the shell push and the "listo" toast. */
  const cloudRef = useRef<readonly CloudJob[]>(NO_CLOUD_JOBS);
  const [loadError, setLoadError] = useState<LoadError | null>(null);
  const modelRef = useRef<HubModel | null>(null);
  /** Last accepted poll; a rejected source falls back to it. */
  const snapshotRef = useRef<HubSnapshot | null>(null);
  const scrolledTo = useRef<string | null>(null);
  // One poll at a time. A delete (or any other refresh) during that poll must
  // not be dropped, and the poll must not publish the list it started with.
  const loadSnapshot = useRef(createCoalescedCall(() => fetchSnapshot(snapshotRef.current)));

  const openResult = useCallback((destination: HubDestination) => {
    if ('matchId' in destination) {
      scrolledTo.current = null;
      router.replace(hubHref({ open: destination.matchId }), { scroll: false });
    } else {
      // The publishing page also previews/downloads outputs whose demo was deleted.
      router.push(publishHref(ORPHAN_MATCH_SEGMENT, destination.clipId));
    }
  }, [router]);

  const accept = useCallback((snapshot: HubSnapshot) => {
    const next = buildHubModel(snapshot.matches, snapshot.videos);
    if (hubPollUnchanged(snapshotRef.current, modelRef.current, next, snapshot)) {
      if (hubPollClearsLoadError(snapshot)) {
        publishShellJobs(collectShellJobs({ ...snapshot, ...cloudShell(cloudRef.current) }), Date.now());
        setLoadError(null);
      }
      return modelRef.current ?? next;
    }
    if (modelRef.current !== null) announceTransitions(modelRef.current, next, openResult);
    modelRef.current = next;
    snapshotRef.current = snapshot;
    setModel(next);
    setStreams(snapshot.streams);
    setLoadError(snapshot.failure === null ? null : { offline: isDemoServiceUnavailable(snapshot.failure) });
    // A partial poll carries stale sources; the shell monitor fetches for itself instead.
    if (snapshot.failure === null) publishShellJobs(collectShellJobs({ ...snapshot, ...cloudShell(cloudRef.current) }), Date.now());
    return next;
  }, [openResult]);

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

  // Cloud jobs stay out of the hub model and the reel store: their own poll, their own cadence.
  const refreshCloud = useCallback(async (): Promise<readonly CloudJob[]> => {
    const next = await cloudApi.jobs();
    for (const job of cloudJobsNewlyReady(cloudRef.current, next)) {
      toast(`${job.title} listo`, {
        description: 'Grabado en la nube y descargado en este PC',
        action: job.localJobId === '' ? undefined : { label: 'Abrir', onClick: () => openResult({ matchId: job.localJobId }) },
      });
    }
    cloudRef.current = next;
    // The cloud beat is the faster one while a cloud job moves: keep the transport pill in step with it.
    const snapshot = snapshotRef.current;
    if (snapshot !== null && snapshot.failure === null) {
      publishShellJobs(collectShellJobs({ ...snapshot, ...cloudShell(next) }), Date.now());
    }
    setCloudJobs((prev) => (sameHubProps(prev, next) ? prev : next));
    return next;
  }, [openResult]);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        const next = await refreshCloud();
        return active && anyCloudJobActive(next, cloudDevice()) ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [refreshCloud]);

  const onCloudChange = useCallback(() => {
    void refreshCloud().catch(() => {});
  }, [refreshCloud]);

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

  // Hidden until storage is read, so a dismissed guide never flashes in on hydration.
  const [guideDismissed, setGuideDismissed] = useState(true);
  useEffect(() => {
    setGuideDismissed(firstRunGuideDismissed(browserStorage()));
  }, []);
  const hideGuide = useCallback(() => {
    dismissFirstRunGuide(browserStorage());
    setGuideDismissed(true);
  }, []);

  useWebMCPLibrary({ model, lens, open, failed: loadError !== null, navigate });

  /** One stable callback for every row: a fresh closure per row would defeat their memo. */
  const onToggle = useCallback(
    (matchId: string) => {
      navigate(open === matchId ? {} : { open: matchId });
    },
    [navigate, open],
  );

  if (model === null) {
    if (loadError === null) return <HubSkeleton />;
    return (
      <div className="measure-list flex flex-col gap-6">
        <HubEmpty banner={<HubBanner offline={loadError.offline} onRetry={onChange} />} />
      </div>
    );
  }

  if (model.rows.length === 0 && model.clips.length === 0 && streamLibraryCount(streams) === 0 && cloudJobs.length === 0) {
    return (
      <div className="measure-list flex flex-col gap-6">
        <HubEmpty banner={loadError !== null ? <HubBanner offline={loadError.offline} onRetry={onChange} /> : null} />
      </div>
    );
  }

  const counts: Record<HubLens, number> = { partidas: model.rows.length, clips: model.clips.length + streamLibraryCount(streams) };
  const device: CloudDevice = { now: Date.now(), linked: cloudLinked };
  const jobs = activeJobCount(model, streams) + cloudJobs.filter((job) => anyCloudJobActive([job], device)).length;
  const cloudByMatch = cloudJobsByMatch(cloudJobs);
  const rowIds = new Set(model.rows.map((row) => row.match.id));
  // A cloud job outlives its partida: it still has to be followed, played and removed.
  const looseCloudJobs = cloudJobs.filter((job) => !rowIds.has(job.localJobId));
  const progress = firstRunProgress(model);

  return (
    <div className="measure-list flex flex-col gap-6">
      <HubHeader lens={lens} />
      {/* Same slot in every hub state: under the header, before anything that depends on the service. */}
      {loadError !== null ? <HubBanner offline={loadError.offline} onRetry={onChange} /> : null}
      {!guideDismissed && !firstRunComplete(progress) ? (
        <FirstRunGuide progress={progress} onDismiss={hideGuide} />
      ) : null}
      <CreationPaths />

      <div className="flex flex-wrap items-center justify-between gap-4">
        <LensToggle lens={lens} counts={counts} />
        {/* The lens already counts partidas; this line only reports work in flight. */}
        <span role="status" className="font-mono text-meta uppercase tracking-wider text-fg-3">
          {jobs === 0 ? 'Nada en marcha' : `${jobs} ${jobs === 1 ? 'trabajo en marcha' : 'trabajos en marcha'}`}
        </span>
      </div>

      {lens === HUB_LENS.clips ? (
        <ClipsLens
          clips={model.clips}
          streams={streams}
          onChange={onChange}
          onOpenMatch={(matchId) => {
            scrolledTo.current = null;
            navigate({ lens: HUB_LENS.matches, open: matchId });
          }}
        />
      ) : (
        <div className="flex flex-col gap-5" aria-busy={loadError !== null || undefined}>
          <div className="flex flex-col gap-3">
            {model.rows.map((row) => (
              <MatchRow
                key={row.match.id}
                row={row}
                open={open === row.match.id}
                onToggle={onToggle}
                onChange={onChange}
                cloud={cloudByMatch.get(row.match.id) ?? NO_CLOUD_JOBS}
                onCloudChange={onCloudChange}
              />
            ))}
          </div>
          {looseCloudJobs.length > 0 ? (
            <section aria-label={CLOUD_SECTION_TITLE} className="flex flex-col gap-2">
              <h2 className="font-mono text-meta uppercase tracking-widest text-fg-3">
                {CLOUD_SECTION_TITLE} · {looseCloudJobs.length}
              </h2>
              <div className="flex flex-col gap-2">
                {looseCloudJobs.map((job) => (
                  <CloudOutputItem key={job.id} job={job} onChange={onCloudChange} />
                ))}
              </div>
            </section>
          ) : null}
          {/* No row and a source down: "no partida" may only mean the jobs index never listed. */}
          {model.orphans.length > 0 && (model.rows.length > 0 || loadError === null) ? (
            <section aria-label={HUB_ORPHANS_TITLE} className="flex flex-col gap-2">
              <header className="flex flex-col gap-0.5">
                <h2 className="font-mono text-meta uppercase tracking-widest text-fg-3">
                  {HUB_ORPHANS_TITLE} · {model.orphans.length}
                </h2>
                <p className="text-meta text-fg-3">{HUB_ORPHANS_HINT}</p>
              </header>
              <div className="flex flex-col gap-2">
                {model.orphans.map((output) => (
                  <OutputItem key={output.id} output={output} matchId={ORPHAN_MATCH_SEGMENT} onChange={onChange} />
                ))}
              </div>
            </section>
          ) : null}
        </div>
      )}
    </div>
  );
}

/** Same blocks and heights as the loaded hub, so the first poll does not shift the list. */
function HubSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando partidas" className="measure-list flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="measure-read h-5 w-full" />
      </div>
      <div className="grid gap-3 @[48rem]/content:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-36 w-full" />
        ))}
      </div>
      <Skeleton className="h-10 w-56" />
      <div className="flex flex-col gap-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-[85px] w-full" />
        ))}
      </div>
    </div>
  );
}
