'use client';

import { use, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ChevronRight, Layers } from 'lucide-react';
import { api } from '@/lib/api';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import type { RosterMatch, SeriesDemo, Video } from '@/lib/api/types';
import {
  isSeriesId,
  seriesReelIsActive,
  seriesReelLabel,
  seriesReelTone,
  seriesStatusLabel,
  seriesStatusTone,
  seriesStatusIsPending,
  seriesStatusIsForgeable,
  summarizeSeriesStatuses,
  seriesTitle,
  type SeriesStatusTone,
} from '@/lib/series-status';
import { prettyMapName } from '@/lib/format';
import { groupSeriesDemos, representativeSeriesStatus, type SeriesGroup } from '@/lib/series-grouping';
import { startPollLoop } from '@/lib/poll-loop';
import { StudioDataRow } from '@/components/studio/data-row';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { LongOperation } from '@/components/studio/long-operation';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

// A series detail belongs to the demo-upload journey, so it shares its number.

/** Fast while any map is still working, relaxed once the series has settled. */
const FAST_MS = 2500;
const IDLE_MS = 8000;

/** True when an API error means the local analysis service is unreachable. */
function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

/** Each demo's headline: prettified map name, else file name, else its position. */
function demoTitle(demo: SeriesDemo, index: number): string {
  if (demo.match) return prettyMapName(demo.match.map);
  if (demo.fileName) return demo.fileName;
  return `Mapa ${index + 1}`;
}

/**
 * Header description built from the real status buckets, omitting empty ones:
 * "2 mapas con jugadas listas · 1 analizando · 1 fallido · 1 sin jugador".
 * The ready bucket spans every forgeable status (parsing done through done), so
 * its copy stays true whether a map is grabando, renderizando or completada.
 * Only genuinely pending maps are ever described as being analyzed; settled
 * ones (failed, or scanned without the chosen player) get their own bucket.
 */
function seriesDescription(statuses: readonly string[]): string {
  const { ready, pending, failed, skipped } = summarizeSeriesStatuses(statuses);
  const parts: string[] = [];
  if (ready > 0) parts.push(ready === 1 ? '1 mapa con jugadas listas' : `${ready} mapas con jugadas listas`);
  if (pending > 0) parts.push(`${pending} analizando`);
  if (failed > 0) parts.push(failed === 1 ? '1 fallido' : `${failed} fallidos`);
  if (skipped > 0) parts.push(`${skipped} sin jugador`);
  if (parts.length === 0) return 'Sin demos en la serie.';
  return `${parts.join(' · ')}.`;
}

/**
 * Each map's newest reel: the Library reels that belong to this series' jobs,
 * keyed by job. listVideos returns reels newest-first, so the first hit per
 * job is the one the map card should describe.
 */
function latestReelPerJob(demos: readonly SeriesDemo[], videos: readonly Video[]): ReadonlyMap<string, Video> {
  const jobIds = new Set(demos.map((d) => d.jobId));
  const byJob = new Map<string, Video>();
  for (const video of videos) {
    if (video.jobId === undefined || !jobIds.has(video.jobId)) continue;
    if (!byJob.has(video.jobId)) byJob.set(video.jobId, video);
  }
  return byJob;
}

/**
 * The five series tones expressed in the kit's vocabulary. v3 painted these with
 * raw `amber-400`/`emerald-400`, which neither tracked the theme nor matched the
 * warning/success roles that already exist as tokens.
 */
const TAG_TONE: Record<SeriesStatusTone, StatusTagTone> = {
  pending: 'neutral',
  ready: 'primary',
  progress: 'warning',
  done: 'success',
  failed: 'danger',
};

/**
 * The spine node's lit edge. A node is a square plate on the well surface whose
 * border and glyph carry the map's state, so a bo3 reads as a lit bracket down
 * the left margin rather than as three unrelated boxes.
 */
const NODE_TONE_CLASS: Record<SeriesStatusTone, string> = {
  pending: 'border-border-strong text-fg-2',
  ready: 'border-primary/55 text-primary shadow-[var(--elev-1),var(--glow-primary-sm)]',
  progress: 'border-warning/55 text-warning shadow-[var(--elev-1)]',
  done: 'border-success/55 text-success shadow-[var(--elev-1)]',
  failed: 'border-destructive/55 text-destructive shadow-[var(--elev-1)]',
};

/** The rail leaving a node, tinted by that node's state. */
const RAIL_TONE_CLASS: Record<SeriesStatusTone, string> = {
  pending: 'bg-border-subtle',
  ready: 'bg-primary/45',
  progress: 'bg-warning/45',
  done: 'bg-success/45',
  failed: 'bg-destructive/45',
};

/** A map's live state, reel-aware: a queued reel outranks the settled job status. */
function demoTone(demo: SeriesDemo, reel: Video | undefined): SeriesStatusTone {
  return reel ? seriesReelTone(reel.status) : seriesStatusTone(demo.status);
}

/** The Spanish label matching {@link demoTone}. */
function demoLabel(demo: SeriesDemo, reel: Video | undefined): string {
  return reel ? seriesReelLabel(reel.status) : seriesStatusLabel(demo.status);
}

/**
 * The part whose status stands for the whole map, matching the bucket the page
 * header counts it in, so the spine node and the header never contradict.
 */
function representativeDemo(demos: readonly SeriesDemo[]): SeriesDemo {
  const status = representativeSeriesStatus(demos.map((d) => d.status));
  return demos.find((d) => d.status === status) ?? demos[0];
}

/**
 * Series view (/series/[id]) — the demos uploaded together as one bo3/bo5. It
 * lists every map with its map/score and live status, links each ready map into
 * its highlight picker, and polls the local orchestrator until every map has
 * settled. Reached from the /upload series flow after the picked player is
 * parsed on each map.
 */
export default function SeriesPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const valid = isSeriesId(id);

  const [demos, setDemos] = useState<SeriesDemo[] | null>(null);
  const [reelByJob, setReelByJob] = useState<ReadonlyMap<string, Video>>(new Map());
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<'offline' | 'generic' | null>(null);
  // Read the latest demos inside the poll tick without re-subscribing the loop:
  // a transient poll failure must not wipe an already-loaded list.
  const demosRef = useRef<SeriesDemo[] | null>(null);

  useEffect(() => {
    if (!valid) return;
    // The App Router reuses this page instance across dynamic-param changes, so
    // the effect must reset every piece of series state before polling the new
    // id; otherwise the previous series' demos linger and the loading state
    // never re-renders when switching series.
    setDemos(null);
    setReelByJob(new Map());
    setLoaded(false);
    setLoadError(null);
    demosRef.current = null;

    let active = true;
    let stopLoop: (() => void) | undefined;
    const stop = startPollLoop({
      tick: async () => {
        try {
          // listVideos also runs the reel reconcile tick, so keeping this page
          // open is enough to drive every queued reel through record → render;
          // the user never has to visit the Library for the queue to advance.
          const [list, videos] = await Promise.all([api.getSeries(id), api.listVideos()]);
          if (!active) return 'idle';
          const reels = latestReelPerJob(list, videos);
          demosRef.current = list;
          setDemos(list);
          setReelByJob(reels);
          setLoadError(null);
          setLoaded(true);
          const pending =
            list.some((d) => seriesStatusIsPending(d.status)) ||
            Array.from(reels.values()).some((v) => seriesReelIsActive(v.status));
          // A settled series (no map still working, no reel still forging) does
          // one fetch, renders, and stops: keep polling only while something is
          // pending. stopLoop is assigned before this async tick can reach here
          // (the tick suspends on the awaits above), so the call is safe.
          if (!pending) stopLoop?.();
          return pending ? 'fast' : 'idle';
        } catch (err) {
          if (!active) return 'idle';
          setLoaded(true);
          // Only surface an error screen before the first successful load; once
          // demos are on screen, keep them and let the next tick recover.
          if (demosRef.current === null) {
            setLoadError(isServiceUnavailable(err) ? 'offline' : 'generic');
          }
          return 'idle';
        }
      },
      fastMs: FAST_MS,
      idleMs: IDLE_MS,
    });
    stopLoop = stop;
    return () => {
      active = false;
      stop();
    };
  }, [id, valid]);

  if (!valid) {
    return (
      <StudioEmptyState
        icon={Layers}
        title="Serie no encontrada"
        description="Ese enlace de serie no es válido. Sube tus demos para empezar una serie nueva."
        note="Las series se identifican con el id que acuña /upload al soltar varias demos."
        actions={
          <Button asChild variant="hero">
            <Link href="/upload">SUBIR DEMOS</Link>
          </Button>
        }
      />
    );
  }

  if (!loaded) {
    return <LoadingState />;
  }

  if (demos === null && loadError) {
    const offline = loadError === 'offline';
    return (
      <StudioEmptyState
        icon={Layers}
        title={offline ? 'Servicio de análisis offline' : 'No se pudo cargar la serie'}
        description={
          offline
            ? 'Arranca el servicio de análisis local y vuelve a intentarlo.'
            : 'Hubo un problema al cargar esta serie. Recarga la página para reintentar.'
        }
        note={offline ? 'ClipHub no envía tus demos a ningún servidor: todo el análisis es local.' : undefined}
        actions={
          <Button asChild variant="outline">
            <Link href="/upload">SUBIR DEMOS</Link>
          </Button>
        }
      />
    );
  }

  const list = demos ?? [];
  if (list.length === 0) {
    return (
      <StudioEmptyState
        icon={Layers}
        title="Esta serie está vacía"
        description="No hay demos en esta serie. Sube las demos de tu bo3/bo5 para forjar sus highlights."
        actions={
          <Button asChild variant="hero">
            <Link href="/upload">SUBIR DEMOS</Link>
          </Button>
        }
      />
    );
  }

  // HLTV-style downloads split one map into several .dem parts; fold them back
  // into one logical map card so a bo3 reads "SERIE DE 3 MAPAS", not 4.
  const groups = groupSeriesDemos(list);
  const mapStatuses = groups.map((g) => representativeSeriesStatus(g.demos.map((d) => d.status)));
  // The maps the poll loop is still waiting on, named, so the live region says
  // which ones rather than just that something is happening.
  const workingTitles = groups
    .map((group, index) => ({ group, index }))
    .filter(({ group }) =>
      group.demos.some((d) => {
        const reel = reelByJob.get(d.jobId);
        return seriesStatusIsPending(d.status) || (reel !== undefined && seriesReelIsActive(reel.status));
      }),
    )
    .map(({ group, index }) => demoTitle(group.demos.find((d) => d.match) ?? group.demos[0], index));

  return (
    <div className="flex flex-col gap-8 sm:gap-10">
      <StudioPageHeader
        title={seriesTitle(groups.length)}
        description={seriesDescription(mapStatuses)}
      />

      {workingTitles.length > 0 ? (
        // No percentage exists for a demo parse, so the bar stays indeterminate
        // rather than inventing one. This is also the page's polite live region.
        <LongOperation
          className="studio-panel px-5 py-4"
          stage={workingTitles.length === 1 ? 'ANALIZANDO 1 MAPA' : `ANALIZANDO ${workingTitles.length} MAPAS`}
          detail={workingTitles.join(' · ')}
        />
      ) : null}

      <ol className="flex flex-col gap-4">
        {groups.map((group, i) => (
          <SeriesSpineItem
            key={group.demos.length === 1 ? group.demos[0].jobId : group.key}
            group={group}
            index={i}
            last={i === groups.length - 1}
            seriesId={id}
            reelByJob={reelByJob}
          />
        ))}
      </ol>
    </div>
  );
}

/**
 * One rung of the series spine: the numbered status node, the rail that carries
 * its tone down to the next map, and the map card itself. The rail is what turns
 * the list into a bracket — it is decorative, so it is `aria-hidden` and the
 * ordered list carries the sequence for assistive tech.
 */
function SeriesSpineItem({
  group,
  index,
  last,
  seriesId,
  reelByJob,
}: {
  group: SeriesGroup<SeriesDemo>;
  index: number;
  last: boolean;
  seriesId: string;
  reelByJob: ReadonlyMap<string, Video>;
}): ReactNode {
  const lead = representativeDemo(group.demos);
  const tone = demoTone(lead, reelByJob.get(lead.jobId));

  return (
    <li className="flex gap-4 sm:gap-5">
      <div className="relative flex w-11 shrink-0 justify-center">
        <span
          aria-hidden
          className={cn(
            'z-10 grid size-11 place-items-center border bg-surface-0 font-mono text-body tabular-nums',
            NODE_TONE_CLASS[tone],
          )}
        >
          {String(index + 1).padStart(2, '0')}
        </span>
        {last ? null : (
          <span aria-hidden className={cn('absolute inset-x-0 top-12 -bottom-4 mx-auto w-px', RAIL_TONE_CLASS[tone])} />
        )}
      </div>
      <SeriesMapCard group={group} index={index} seriesId={seriesId} reelByJob={reelByJob} />
    </li>
  );
}

/**
 * One logical map. A map downloaded as a single .dem carries its status tag and
 * forge CTA in the header; an HLTV `-pN` split renders the same header over an
 * indented sub-list, one row per part, each with its own state and link. v3
 * shipped these as two components with byte-identical pill/CTA logic.
 */
function SeriesMapCard({
  group,
  index,
  seriesId,
  reelByJob,
}: {
  group: SeriesGroup<SeriesDemo>;
  index: number;
  seriesId: string;
  reelByJob: ReadonlyMap<string, Video>;
}): ReactNode {
  // Title from the first part that has a roster match; fall back to the first
  // part's own title so a still-scanning map still names itself.
  const head = group.demos.find((d) => d.match) ?? group.demos[0];
  const split = group.demos.length > 1;
  const single = split ? null : group.demos[0];
  const singleReel = single ? reelByJob.get(single.jobId) : undefined;
  const lead = representativeDemo(group.demos);
  const leadTone = demoTone(lead, reelByJob.get(lead.jobId));

  return (
    <article className="studio-panel studio-panel-interactive flex min-w-0 flex-1 flex-col gap-4 p-4 sm:p-5">
      <div className="flex flex-col gap-4 @[42rem]/content:flex-row @[42rem]/content:items-center @[42rem]/content:justify-between @[42rem]/content:gap-6">
        <div className="flex min-w-0 flex-col gap-1.5">
          <h2 className="truncate font-display text-title font-bold uppercase text-fg-1">{demoTitle(head, index)}</h2>
          <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
            {split ? `${group.demos.length} partes` : null}
            {split && head.match ? ' · ' : null}
            {head.match ? `${head.match.rounds} rondas` : null}
            {!split && !head.match ? 'sin marcador todavía' : null}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-x-5 gap-y-3 @[42rem]/content:shrink-0 @[42rem]/content:justify-end">
          {head.match ? <MapScore match={head.match} /> : null}
          {split ? (
            <StatusTag tone={TAG_TONE[leadTone]} dot>
              {demoLabel(lead, reelByJob.get(lead.jobId))}
            </StatusTag>
          ) : null}
          {single ? (
            <>
              <StatusTag tone={TAG_TONE[demoTone(single, singleReel)]} dot>
                {demoLabel(single, singleReel)}
              </StatusTag>
              <ForgeLink demo={single} seriesId={seriesId} reel={singleReel} />
            </>
          ) : null}
        </div>
      </div>

      {single && single.status === 'failed' && single.failureReason ? (
        <FailureNote>{single.failureReason}</FailureNote>
      ) : null}

      {split ? (
        <ol className="flex flex-col gap-2 border-l-2 border-border-subtle pl-3 sm:pl-4">
          {group.demos.map((demo, partIndex) => (
            <li key={demo.jobId}>
              <SeriesPart
                demo={demo}
                partIndex={partIndex}
                seriesId={seriesId}
                reel={reelByJob.get(demo.jobId)}
              />
            </li>
          ))}
        </ol>
      ) : null}
    </article>
  );
}

/**
 * One part of a split map. `StudioDataRow` supplies the shared label/value/state
 * geometry; the part only decides what goes in each slot, so a part row and an
 * upload scan row are the same object.
 */
function SeriesPart({
  demo,
  partIndex,
  seriesId,
  reel,
}: {
  demo: SeriesDemo;
  partIndex: number;
  seriesId: string;
  reel: Video | undefined;
}): ReactNode {
  const tone = demoTone(demo, reel);
  const failed = demo.status === 'failed';
  const reason = failed ? demo.failureReason : undefined;

  return (
    <>
      <StudioDataRow
        className={cn('bg-surface-1', failed && 'border-destructive/45', reason !== undefined && 'border-b-0')}
        label={`Parte ${partIndex + 1}`}
        value={demo.match ? `${demo.match.scoreT}-${demo.match.scoreCt}` : undefined}
        status={
          <>
            <StatusTag tone={TAG_TONE[tone]} dot>
              {demoLabel(demo, reel)}
            </StatusTag>
            <ForgeLink demo={demo} seriesId={seriesId} reel={reel} />
          </>
        }
      />
      {reason !== undefined ? <FailureNote attached>{reason}</FailureNote> : null}
    </>
  );
}

/** The forge CTA, present only once a map actually has a kill plan. */
function ForgeLink({
  demo,
  seriesId,
  reel,
}: {
  demo: SeriesDemo;
  seriesId: string;
  reel: Video | undefined;
}): ReactNode {
  if (!seriesStatusIsForgeable(demo.status)) return null;
  return (
    <Button asChild size="sm" variant={reel ? 'outline' : 'hero'}>
      <Link href={`/matches/${demo.jobId}?series=${seriesId}`}>
        {reel ? 'OTRO REEL' : 'ELEGIR JUGADAS'}
        <ChevronRight className="size-4" />
      </Link>
    </Button>
  );
}

/** The orchestrator's own words about why a demo failed, kept verbatim. */
function FailureNote({ children, attached = false }: { children: ReactNode; attached?: boolean }): ReactNode {
  return (
    <p
      className={cn(
        'border border-destructive/45 bg-destructive/8 px-3.5 py-2 text-body-sm text-destructive',
        attached && 'border-t-0',
      )}
    >
      {children}
    </p>
  );
}

/**
 * The map scoreline, typeset as a scoreboard rather than as body text: mono,
 * tabular, and the largest number on the card. The winning half stays in --fg-1
 * and the losing half drops to --fg-3, with a hairline rule between them — the
 * literal spaces v3 used rendered as digit-width figure spaces inside a
 * tabular-nums run and jittered against the figures.
 */
function MapScore({ match }: { match: RosterMatch }): ReactNode {
  const tWon = match.scoreT > match.scoreCt;
  const ctWon = match.scoreCt > match.scoreT;
  return (
    <span
      className="inline-flex items-center gap-2.5 font-mono text-title tabular-nums @[34rem]/content:text-stat"
      aria-label={`Marcador ${match.scoreT} a ${match.scoreCt}`}
    >
      <span className={tWon ? 'text-fg-1' : 'text-fg-3'}>{match.scoreT}</span>
      <span aria-hidden className="h-6 w-px bg-border-strong" />
      <span className={ctWon ? 'text-fg-1' : 'text-fg-3'}>{match.scoreCt}</span>
    </span>
  );
}

/**
 * Skeleton while the first series poll is in flight. It reproduces the spine —
 * node column, rail, card — so the first paint does not reflow into a different
 * shape once the demos land.
 */
function LoadingState(): ReactNode {
  return (
    <div className="flex flex-col gap-8 sm:gap-10" role="status" aria-label="Cargando la serie">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-10 w-72 max-w-full" />
        <Skeleton className="h-5 w-96 max-w-full" />
      </div>
      <div className="flex flex-col gap-4">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex gap-4 sm:gap-5">
            <div className="relative flex w-11 shrink-0 justify-center">
              <Skeleton className="size-11 rounded-none" />
              {i < 2 ? <span aria-hidden className="absolute inset-x-0 top-12 -bottom-4 mx-auto w-px bg-border-subtle" /> : null}
            </div>
            <Skeleton className="h-[104px] flex-1" />
          </div>
        ))}
      </div>
    </div>
  );
}
