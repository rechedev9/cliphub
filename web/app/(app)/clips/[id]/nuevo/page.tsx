'use client';

import { use, useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, SearchX, Unplug, Users } from 'lucide-react';
import { api } from '@/lib/api';
import { DEMO_CREATION_STEPS } from '@/lib/clips/copy';
import type { Match, Play } from '@/lib/api/types';
import { HUB_ROW_STAGE, matchRowStage } from '@/lib/clips/hub';
import {
  hubHref,
  isProduceFormat,
  newDemoHref,
  PRODUCE_FORMAT,
  PRODUCE_QUERY,
  produceHref,
  seriesHref,
  type ProduceFormat,
} from '@/lib/clips/routes';
import { classifyFullDemoLoadFailure, fullDemoEmptyState, type FullDemoLoadFailure } from '@/lib/full-demo';
import {
  MATCH_PLAYS_ANALYZING_DESCRIPTION,
  MATCH_PLAYS_ANALYZING_TITLE,
  MATCH_PLAYS_EMPTY_DESCRIPTION,
  MATCH_PLAYS_EMPTY_TITLE,
  MATCH_PLAYS_ERROR_DESCRIPTION,
  MATCH_PLAYS_ERROR_TITLE,
} from '@/lib/match-plays-empty';
import { startPollLoop } from '@/lib/poll-loop';
import {
  PRODUCE_MATCH_MISSING,
  PRODUCE_MATCH_NO_POV,
  PRODUCE_PICK_POV_CTA,
  PRODUCE_POLL_ERROR,
  PRODUCE_POLL_OFFLINE,
} from '@/lib/produce/copy';
import { isSeriesId } from '@/lib/series-status';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { StudioBackLink } from '@/components/studio/back-link';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { WorkflowProgress } from '@/components/studio/workflow-progress';
import { ProduceFormatBar } from '@/components/produce/format-bar';
import { ShortProducer } from '@/components/produce/short-producer';
import { FullPovProducer } from '@/components/produce/full-pov-producer';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

type RecapFailure = Exclude<FullDemoLoadFailure, null> | null;

export default function ProducePage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}): ReactNode {
  const { id } = use(params);
  const query = use(searchParams);
  const formato = query[PRODUCE_QUERY.format];
  const series = query[PRODUCE_QUERY.series];
  const format: ProduceFormat = typeof formato === 'string' && isProduceFormat(formato) ? formato : PRODUCE_FORMAT.short;
  const seriesId = typeof series === 'string' && isSeriesId(series) ? series : null;
  const router = useRouter();

  const [match, setMatch] = useState<Match | null>(null);
  const [plays, setPlays] = useState<Play[]>([]);
  const [playsError, setPlaysError] = useState(false);
  const [shortPlanJobId, setShortPlanJobId] = useState<string | null>(null);
  const [rounds, setRounds] = useState<Play[]>([]);
  const [recapFailure, setRecapFailure] = useState<RecapFailure>(null);
  const [loaded, setLoaded] = useState(false);
  const [loadFailure, setLoadFailure] = useState<FullDemoLoadFailure>(null);
  /** Last tick failed while a partida is already on screen: a banner, not an empty state. */
  const [pollError, setPollError] = useState<FullDemoLoadFailure>(null);
  // The poll cadence depends on the current format without restarting the loop.
  const formatRef = useRef(format);
  formatRef.current = format;
  /** The last partida the API confirmed; a failing tick must keep it on screen. */
  const matchRef = useRef<Match | null>(null);

  const activity = useSyncExternalStore(subscribeToShellActivity, shellActivitySnapshot, serverShellActivitySnapshot);
  const recBusy = activity.jobs.some((job) => job.stage === 'recording');

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        // One wave: the client shares this beat's status and kill-plan reads
        // across the three calls, so asking for everything at once costs one
        // status request plus one document wave instead of a waterfall.
        const [matchResult, planResult, recapResult] = await Promise.allSettled([
          api.getMatch(id),
          api.findClips(id),
          api.findRecapClips(id),
        ]);
        if (!active) return 'idle';
        if (matchResult.status === 'rejected') {
          // A transient tick must not wipe a loaded partida: only a definitive
          // `null` from the API (handled below) turns this into "no encontrada".
          const failure = classifyFullDemoLoadFailure(matchResult.reason);
          if (matchRef.current === null) setLoadFailure(failure);
          else setPollError(failure);
          setLoaded(true);
          return 'idle';
        }
        const nextMatch = matchResult.value;
        matchRef.current = nextMatch;
        setMatch(nextMatch);
        setLoadFailure(null);
        setPollError(null);
        if (!nextMatch || matchRowStage(nextMatch.status) !== HUB_ROW_STAGE.ready) {
          setPlays([]);
          setPlaysError(false);
          setRounds([]);
          setRecapFailure(null);
          setLoaded(true);
          // `scanned` never advances without a POV pick, so only a real parse polls fast.
          return nextMatch !== null && matchRowStage(nextMatch.status) === HUB_ROW_STAGE.parsing ? 'fast' : 'idle';
        }
        if (planResult.status === 'fulfilled') {
          setPlays(planResult.value);
          if (planResult.value.length > 0) setShortPlanJobId(id);
          setPlaysError(false);
        } else {
          setPlays([]);
          setPlaysError(true);
        }
        let recapPending = false;
        if (recapResult.status === 'fulfilled') {
          setRounds(recapResult.value);
          setRecapFailure(null);
          recapPending = recapResult.value.length === 0;
        } else {
          setRounds([]);
          setRecapFailure(classifyFullDemoLoadFailure(recapResult.reason));
        }
        setLoaded(true);
        return recapPending && formatRef.current === PRODUCE_FORMAT.full ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [id]);

  function changeFormat(next: ProduceFormat): void {
    if (next === format) return;
    router.replace(produceHref(id, next, seriesId ?? undefined));
  }

  if (!loaded) return <LoadingState />;

  const backHref = seriesId ? seriesHref(seriesId) : hubHref({ open: id });

  if (!match) {
    const empty = matchEmptyState(loadFailure);
    return (
      <div className="measure-work flex flex-col gap-8">
        <StudioBackLink href={backHref}>{seriesId ? 'Serie' : 'Clips y vídeos'}</StudioBackLink>
        <StudioEmptyState
          icon={empty.icon}
          title={empty.title}
          description={empty.description}
          actions={<Button onClick={() => router.push(backHref)}>Volver</Button>}
        />
      </div>
    );
  }

  const stage = matchRowStage(match.status);
  let body: ReactNode;
  if (stage === HUB_ROW_STAGE.unpicked) {
    body = (
      <StudioEmptyState
        icon={Users}
        title={PRODUCE_MATCH_NO_POV.title}
        description={PRODUCE_MATCH_NO_POV.description}
        compact
        actions={
          <>
            <Button onClick={() => router.push(newDemoHref({ job: id, format }))}>{PRODUCE_PICK_POV_CTA}</Button>
            <Button variant="outline" onClick={() => router.push(backHref)}>
              Volver
            </Button>
          </>
        }
      />
    );
  } else if (stage === HUB_ROW_STAGE.parsing) {
    body = (
      <StudioEmptyState
        icon={SearchX}
        title={MATCH_PLAYS_ANALYZING_TITLE}
        description={MATCH_PLAYS_ANALYZING_DESCRIPTION}
        note={
          <span className="inline-flex items-center gap-2">
            <span className="studio-spinner text-primary" aria-hidden />
            Analizando jugadas{match.player ? ` de ${match.player}` : ''}
          </span>
        }
        compact
      />
    );
  } else {
    let shortContent: ReactNode = null;
    if (playsError) {
      shortContent = <StudioEmptyState icon={AlertTriangle} title={MATCH_PLAYS_ERROR_TITLE}
        description={MATCH_PLAYS_ERROR_DESCRIPTION} compact actions={<Button onClick={() => router.push(backHref)}>Volver</Button>} />;
    } else if (plays.length === 0) {
      shortContent = <StudioEmptyState icon={SearchX} title={MATCH_PLAYS_EMPTY_TITLE}
        description={MATCH_PLAYS_EMPTY_DESCRIPTION} compact actions={<Button onClick={() => router.push(backHref)}>Volver</Button>} />;
    }
    const shortUnavailable = playsError || plays.length === 0;
    // Keep each format's choices mounted while switching; their approval and configuration stay independent.
    body = (
      <>
        <div hidden={format !== PRODUCE_FORMAT.short}
          className={format === PRODUCE_FORMAT.short ? 'flex flex-1 flex-col' : 'hidden'}>
          {shortContent}
          {shortPlanJobId === id ? (
            <div hidden={shortUnavailable} className={shortUnavailable ? 'hidden' : 'flex flex-1 flex-col'}>
              <ShortProducer matchId={id} match={match} plays={plays} seriesId={seriesId} />
            </div>
          ) : null}
        </div>
        <div hidden={format !== PRODUCE_FORMAT.full}
          className={format === PRODUCE_FORMAT.full ? 'flex flex-1 flex-col' : 'hidden'}>
          <FullPovProducer matchId={id} match={match} rounds={rounds} recapFailure={recapFailure}
            recBusy={recBusy} seriesId={seriesId} />
        </div>
      </>
    );
  }

  return (
    <div className="measure-work flex min-h-[calc(100vh-9rem)] flex-col">
      <div className="mb-5">
        <WorkflowProgress steps={DEMO_CREATION_STEPS}
          current={stage === HUB_ROW_STAGE.unpicked || stage === HUB_ROW_STAGE.parsing ? 1 : 2} />
      </div>
      <ProduceFormatBar value={format} onChange={changeFormat} />
      <div className="flex flex-1 flex-col gap-6 pt-6">
        {pollError !== null ? (
          <p
            role="alert"
            className="flex items-center gap-2.5 border border-warning/40 bg-warning/10 px-4 py-3 text-body-sm text-warning"
          >
            <AlertTriangle className="size-4 shrink-0" aria-hidden />
            {pollError === 'offline' ? PRODUCE_POLL_OFFLINE : PRODUCE_POLL_ERROR}
          </p>
        ) : null}
        {body}
      </div>
    </div>
  );
}

function matchEmptyState(failure: FullDemoLoadFailure): { icon: typeof SearchX; title: string; description: string } {
  if (failure === 'offline') return { icon: Unplug, ...fullDemoEmptyState(failure) };
  if (failure === 'error') return { icon: AlertTriangle, ...fullDemoEmptyState(failure) };
  return { icon: SearchX, ...PRODUCE_MATCH_MISSING };
}

function LoadingState(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-6" role="status" aria-label="Cargando la partida">
      <Skeleton className="h-12 w-full" />
      <div className="grid items-start gap-6 @[56rem]/content:grid-cols-[minmax(0,1fr)_320px]">
        <div className="flex flex-col gap-4">
          <Skeleton className="h-8 w-72" />
          <div className="flex flex-col gap-px overflow-hidden border border-border">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-[86px] w-full" />
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-3">
          <Skeleton className="mx-auto h-[302px] w-[170px]" />
          <Skeleton className="h-20" />
          <Skeleton className="h-20" />
        </div>
      </div>
    </div>
  );
}
