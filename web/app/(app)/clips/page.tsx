'use client';

import { Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { streamsApi, type StreamJob } from '@/lib/api/streams';
import { HUB_ORPHANS_HINT, HUB_ORPHANS_TITLE } from '@/lib/clips/copy';
import {
  activeJobCount,
  buildHubModel,
  HUB_ROW_STAGE,
  hubTransitions,
  isWorking,
  settleHubSnapshot,
  type HubModel,
  type HubSnapshot,
} from '@/lib/clips/hub';
import { HUB_LENS, HUB_QUERY, hubHref, isHubLens, ORPHAN_MATCH_SEGMENT, type HubLens } from '@/lib/clips/routes';
import { isDemoServiceUnavailable } from '@/lib/demo-parse-flow';
import { prettyMapName } from '@/lib/format';
import { startPollLoop } from '@/lib/poll-loop';
import { collectShellJobs, publishShellJobs } from '@/lib/shell-activity';
import { Skeleton } from '@/components/ui/skeleton';
import { ClipsLens } from '@/components/clips-hub/clips-lens';
import { CreationPaths } from '@/components/studio/creation-paths';
import { HubBanner } from '@/components/clips-hub/hub-banner';
import { HubEmpty } from '@/components/clips-hub/hub-empty';
import { HubHeader } from '@/components/clips-hub/hub-header';
import { LensToggle } from '@/components/clips-hub/lens-toggle';
import { MatchRow, matchRowId } from '@/components/clips-hub/match-row';
import { OutputItem } from '@/components/clips-hub/output-item';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

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
  /** Last accepted poll; a rejected source falls back to it. */
  const snapshotRef = useRef<HubSnapshot | null>(null);
  const scrolledTo = useRef<string | null>(null);
  const inFlight = useRef(false);

  const accept = useCallback((snapshot: HubSnapshot) => {
    const next = buildHubModel(snapshot.matches, snapshot.videos);
    if (modelRef.current !== null) announceTransitions(modelRef.current, next);
    modelRef.current = next;
    snapshotRef.current = snapshot;
    setModel(next);
    setStreams(snapshot.streams);
    setLoadError(snapshot.failure === null ? null : { offline: isDemoServiceUnavailable(snapshot.failure) });
    // A partial poll carries stale sources; the shell monitor fetches for itself instead.
    if (snapshot.failure === null) publishShellJobs(collectShellJobs(snapshot), Date.now());
    return next;
  }, []);

  const refresh = useCallback(async (): Promise<HubModel | null> => {
    if (inFlight.current) return null;
    inFlight.current = true;
    try {
      return accept(await fetchSnapshot(snapshotRef.current));
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
      <div className="measure-list flex flex-col gap-5">
        <HubBanner offline={loadError.offline} onRetry={onChange} />
        <HubEmpty />
      </div>
    );
  }

  if (model.rows.length === 0 && model.clips.length === 0) {
    return (
      <div className="measure-list flex flex-col gap-5">
        {loadError !== null ? <HubBanner offline={loadError.offline} onRetry={onChange} /> : null}
        <HubEmpty />
      </div>
    );
  }

  const counts: Record<HubLens, number> = { partidas: model.rows.length, clips: model.clips.length };
  const jobs = activeJobCount(model, streams);

  return (
    <div className="measure-list flex flex-col gap-6">
      <HubHeader lens={lens} />
      <CreationPaths />

      <div className="flex flex-wrap items-center justify-between gap-4">
        <LensToggle lens={lens} counts={counts} />
        {/* The lens already counts partidas; this line only reports work in flight. */}
        <span role="status" className="font-mono text-meta uppercase tracking-wider text-fg-3">
          {jobs === 0 ? 'Nada en marcha' : `${jobs} ${jobs === 1 ? 'trabajo en marcha' : 'trabajos en marcha'}`}
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
        <div className="flex flex-col gap-5" aria-busy={loadError !== null || undefined}>
          <div className="flex flex-col gap-3">
            {model.rows.map((row) => (
              <MatchRow
                key={row.match.id}
                row={row}
                open={open === row.match.id}
                onToggle={onToggle}
                onChange={onChange}
              />
            ))}
          </div>
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
      <Skeleton className="h-10 w-56" />
      <div className="flex flex-col gap-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-[85px] w-full rounded-[10px]" />
        ))}
      </div>
    </div>
  );
}
