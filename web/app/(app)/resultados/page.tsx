'use client';

import { Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Cloud, Monitor, type LucideIcon } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { cloudApi } from '@/lib/api/cloud';
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
  sameHubProps,
  settleHubSnapshot,
  type HubModel,
  type HubSnapshot,
} from '@/lib/clips/hub';
import {
  queuedOutputCount,
  resultsActivityLabel,
  resultsCounts,
  resultsOrder,
  resultsSections,
  type ResultsCounts,
} from '@/lib/clips/results';
import {
  CLOUD_HREF,
  LONG_VIDEOS_HREF,
  ORPHAN_MATCH_SEGMENT,
  publishHref,
  RESULTS_ORIGIN,
  RESULTS_QUERY,
  resultsHref,
  resultsOriginParam,
  SHORTS_HREF,
  type ResultsOrigin,
} from '@/lib/clips/routes';
import { createCoalescedCall } from '@/lib/coalesced-call';
import { cloudAccountSnapshot } from '@/lib/cloud/account-store';
import { cloudDeviceLinked } from '@/lib/cloud/account-view';
import { CAPTURE_TARGET_LABEL } from '@/lib/cloud/capture-target';
import type { CloudJob } from '@/lib/cloud/parse';
import { anyCloudJobActive, cloudJobsNewlyReady, type CloudDevice } from '@/lib/cloud/view';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { collectShellJobs, publishShellJobs } from '@/lib/shell-activity';
import { cn } from '@/lib/utils';
import { useCloudAccount } from '@/hooks/use-cloud-account';
import { ClipsLens } from '@/components/clips-hub/clips-lens';
import { CloudOutputItem } from '@/components/clips-hub/cloud-output-item';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { useWebMCPLibrary } from '@/components/clips-hub/use-webmcp-library';
import { CreationPaths } from '@/components/studio/creation-paths';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

const PAGE_TITLE = 'Resultados';
const PAGE_DESCRIPTION =
  'Todos tus vídeos, en preparación o listos para descargar. Cada uno indica si se grabó en este PC o en la nube.';

const ORIGIN_COPY = {
  pc: { title: 'Grabados en este PC', hint: 'CS2 graba en este ordenador y el vídeo se guarda aquí.' },
  nube: { title: 'Grabados en la nube', hint: 'Los graba un servidor de ClipHub y se descargan en este PC al terminar.' },
} as const satisfies Record<ResultsOrigin, { title: string; hint: string }>;

type LoadError = { offline: boolean };

const NO_CLOUD_JOBS: readonly CloudJob[] = [];

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

function streamLibraryCount(streams: readonly StreamJob[]): number {
  return streams.reduce(
    (count, job) => count + (job.rendered_outputs ?? []).filter((output) => streamPlaybackItem(job, output) !== null).length,
    0,
  );
}

export default function ResultsPage(): ReactNode {
  return (
    <Suspense fallback={<ResultsSkeleton />}>
      <Results />
    </Suspense>
  );
}

function Results(): ReactNode {
  const router = useRouter();
  const origin = resultsOriginParam(useSearchParams().get(RESULTS_QUERY.origin));

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
  // One poll at a time. A delete (or any other refresh) during that poll must
  // not be dropped, and the poll must not publish the list it started with.
  const loadSnapshot = useRef(createCoalescedCall(() => fetchSnapshot(snapshotRef.current)));

  const announceReady = useCallback(
    (prev: HubModel, next: HubModel) => {
      for (const clip of hubTransitions(prev, next).ready) {
        toast(`${clip.title} listo`, {
          description: `${prettyMapName(clip.video.map)} · Grabado en este PC`,
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
        publishShellJobs(collectShellJobs({ ...snapshot, ...cloudShell(cloudRef.current) }), Date.now());
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
    if (snapshot.failure === null) publishShellJobs(collectShellJobs({ ...snapshot, ...cloudShell(cloudRef.current) }), Date.now());
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

  // Cloud jobs stay out of the hub model and the reel store: their own poll, their own cadence.
  const refreshCloud = useCallback(async (): Promise<readonly CloudJob[]> => {
    const next = await cloudApi.jobs();
    for (const job of cloudJobsNewlyReady(cloudRef.current, next)) {
      toast(`${job.title} listo`, { description: 'Grabado en la nube y descargado en este PC' });
    }
    cloudRef.current = next;
    // The cloud beat is the faster one while a cloud job moves: keep the transport pill in step with it.
    const snapshot = snapshotRef.current;
    if (snapshot !== null && snapshot.failure === null) {
      publishShellJobs(collectShellJobs({ ...snapshot, ...cloudShell(next) }), Date.now());
    }
    setCloudJobs((prev) => (sameHubProps(prev, next) ? prev : next));
    return next;
  }, []);

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

  const onChange = useCallback(() => {
    void refresh();
  }, [refresh]);

  useWebMCPLibrary({ model, origin, failed: loadError !== null });

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

  const counts = resultsCounts({ local: model.clips.length + streamLibraryCount(streams), cloud: cloudJobs.length });

  if (counts.all === 0) {
    return (
      <div className="measure-list flex flex-col gap-6">
        {header}
        {banner}
        <p className="rounded-[10px] border border-dashed border-border-subtle p-6 text-body text-fg-2">
          Aún no has creado ningún vídeo. Empieza por uno de estos flujos: lo que crees aparecerá aquí, indicando si se grabó
          en este PC o en la nube.
        </p>
        <CreationPaths />
      </div>
    );
  }

  const device: CloudDevice = { now: Date.now(), linked: cloudLinked };
  const localRunning = activeJobCount(model, streams);
  const cloudRunning = cloudJobs.filter((job) => anyCloudJobActive([job], device)).length;
  const running = localRunning + cloudRunning;
  const sections = resultsSections(origin, counts);
  const order = resultsOrder({
    local: { active: localRunning > 0, newest: Math.max(0, ...model.clips.map((clip) => clip.video.createdAt)) },
    cloud: { active: cloudRunning > 0, newest: Math.max(0, ...cloudJobs.map((job) => job.createdAt)) },
  });
  const sectionOf: Record<ResultsOrigin, ReactNode> = {
    [RESULTS_ORIGIN.cloud]: !sections.cloud ? null : (
      <OriginSection key={RESULTS_ORIGIN.cloud} icon={Cloud} origin={RESULTS_ORIGIN.cloud} count={counts.cloud}>
        {cloudJobs.length === 0 ? (
          <CloudEmpty linked={cloudLinked} />
        ) : (
          <div className="grid gap-2 @[60rem]/content:grid-cols-2">
            {cloudJobs.map((job) => (
              <CloudOutputItem key={job.id} job={job} onChange={onCloudChange} />
            ))}
          </div>
        )}
      </OriginSection>
    ),
    [RESULTS_ORIGIN.local]: !sections.local ? null : (
      <OriginSection key={RESULTS_ORIGIN.local} icon={Monitor} origin={RESULTS_ORIGIN.local} count={counts.local}>
        {counts.local === 0 ? (
          <div className={cn(EMPTY_NOTE_CLASS, 'flex flex-wrap items-center justify-between gap-4')}>
            <p className="min-w-0 flex-1 basis-64">Aún no has grabado ningún vídeo en este PC.</p>
            <div className="flex flex-wrap gap-2">
              <Button asChild size="sm" variant="outline-primary">
                <Link href={SHORTS_HREF}>Crear un Short</Link>
              </Button>
              <Button asChild size="sm" variant="outline">
                <Link href={LONG_VIDEOS_HREF}>Crear un vídeo largo</Link>
              </Button>
            </div>
          </div>
        ) : (
          <div aria-busy={loadError !== null || undefined}>
            <ClipsLens clips={model.clips} streams={streams} onChange={onChange} />
          </div>
        )}
      </OriginSection>
    ),
  };

  return (
    <div className="measure-list flex flex-col gap-6">
      {header}
      {banner}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <OriginFilter origin={origin} counts={counts} />
        <span role="status" className="font-mono text-meta uppercase tracking-wider text-fg-3">
          {resultsActivityLabel({ running, queued: queuedOutputCount(model) })}
        </span>
      </div>

      {order.map((key) => sectionOf[key])}
    </div>
  );
}

const EMPTY_NOTE_CLASS = 'rounded-[10px] border border-dashed border-border-subtle p-6 text-body-sm text-fg-2';

const CHIP_CLASS =
  'inline-flex h-10 items-center gap-2 border px-3 font-mono text-meta uppercase tracking-wider transition-colors duration-(--dur-fast) hover:text-fg-1';

/** Narrows the page to what one place recorded; the choice lives in the URL so it can be linked. */
function OriginFilter({ origin, counts }: { origin: ResultsOrigin | null; counts: ResultsCounts }): ReactNode {
  const options = [
    { key: null, label: 'Todos', count: counts.all, icon: null },
    { key: RESULTS_ORIGIN.local, label: CAPTURE_TARGET_LABEL.local, count: counts.local, icon: Monitor },
    { key: RESULTS_ORIGIN.cloud, label: 'Nube', count: counts.cloud, icon: Cloud },
  ] as const;
  return (
    <nav aria-label="Dónde se grabó" className="flex flex-wrap items-center gap-2">
      <span aria-hidden className="mr-1 font-mono text-meta uppercase tracking-widest text-fg-3">
        Grabado en
      </span>
      {options.map(({ key, label, count, icon: Icon }) => (
        <Link
          key={label}
          href={resultsHref(key === null ? {} : { origin: key })}
          replace
          scroll={false}
          aria-current={origin === key ? 'true' : undefined}
          className={cn(CHIP_CLASS, FOCUS_RING, origin === key ? 'border-primary text-primary' : 'border-border-strong text-fg-3')}
        >
          {Icon === null ? null : <Icon aria-hidden className="size-4" />}
          {label} · {count}
        </Link>
      ))}
    </nav>
  );
}

type OriginSectionProps = {
  icon: LucideIcon;
  origin: ResultsOrigin;
  count: number;
  children: ReactNode;
};

function OriginSection({ icon: Icon, origin, count, children }: OriginSectionProps): ReactNode {
  const { title, hint } = ORIGIN_COPY[origin];
  return (
    <section aria-label={title} data-origin={origin} className="flex flex-col gap-3">
      <header className="flex flex-col gap-1 border-b border-border pb-3">
        <h2 className="flex items-center gap-2.5 font-display text-title font-bold uppercase text-fg-1">
          <Icon aria-hidden className="size-5 shrink-0 text-primary" />
          {title}
          <span className="font-mono text-body-sm font-normal text-fg-3">· {count}</span>
        </h2>
        <p className="text-body-sm text-fg-2">{hint}</p>
      </header>
      {children}
    </section>
  );
}

function CloudEmpty({ linked }: { linked: boolean }): ReactNode {
  return (
    <div className={cn(EMPTY_NOTE_CLASS, 'flex flex-wrap items-center justify-between gap-4')}>
      <p className="min-w-0 flex-1 basis-64">
        {linked
          ? 'Aún no has grabado nada en la nube. Al preparar un Short, elige Nube ClipHub en "Dónde se graba".'
          : 'Este PC no está conectado a la nube. Conecta tu cuenta para grabar Shorts sin usar este ordenador.'}
      </p>
      <Button asChild size="sm" variant="outline-primary">
        <Link href={linked ? SHORTS_HREF : CLOUD_HREF}>{linked ? 'Crear un Short' : 'Conectar con la nube'}</Link>
      </Button>
    </div>
  );
}

/** Same blocks and heights as the loaded page, so the first poll does not shift the list. */
function ResultsSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando resultados" className="measure-list flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="measure-read h-5 w-full" />
      </div>
      <Skeleton className="h-10 w-72" />
      <div className="grid gap-3 @[48rem]/content:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-56 w-full" />
        ))}
      </div>
    </div>
  );
}
