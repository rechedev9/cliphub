import type { StreamJob } from './api/streams.ts';
import type { Match, Video } from './api/types.ts';

/** Live jobs for the command-strip transport and the html capture-active gate. */

/** Mirrors `capturing` onto <html>; the CSS depth/animation gates key off it. */
export const CAPTURE_ACTIVE_ATTRIBUTE = 'data-capture-active';

/** A push is authoritative for this long before the shell polls for itself. */
const SHELL_ACTIVITY_MAX_AGE_MS = 4000;

/**
 * Non-terminal stages across the three job kinds, in the order the pipeline
 * runs them. `recording` is CS2 + HLAE and always sorts first: it is the one
 * stage that owns the machine.
 */
export type ShellJobStage = 'queued' | 'recording' | 'composing' | 'parsing' | 'acquiring';

/** reel = demo output; parse = a partida being parsed; stream = a stream job. */
export type ShellJobKind = 'reel' | 'parse' | 'stream';

export interface ShellJob {
  readonly id: string;
  readonly kind: ShellJobKind;
  readonly title: string;
  readonly stage: ShellJobStage;
  /** Epoch ms the job was created — real API data, used for elapsed time. */
  readonly startedAt: number;
  /** Capture done/total/percent while recording; null on every other stage. */
  readonly progress: { readonly done: number; readonly total: number; readonly percent?: number } | null;
  /** Where the job lives; the transport's row links there. */
  readonly href: string;
}

export interface ShellActivity {
  /** Non-terminal jobs, most advanced first. */
  readonly jobs: readonly ShellJob[];
  /** True while a reel is on the GPU (recording or composing), not merely queued. */
  readonly capturing: boolean;
  /** Epoch ms of the last push; 0 means nothing has ever published. */
  readonly publishedAt: number;
}

const EMPTY: ShellActivity = { jobs: [], capturing: false, publishedAt: 0 };

const STAGE_RANK: Record<ShellJobStage, number> = {
  recording: 0,
  composing: 1,
  acquiring: 2,
  parsing: 3,
  queued: 4,
};

/** Orchestrator statuses that mean "the partida is still being parsed". */
const PARSING_STATUSES: ReadonlySet<string> = new Set(['queued', 'scanning', 'parsing']);

let current: ShellActivity = EMPTY;
const listeners = new Set<() => void>();

/** Full push from a page that knows every source. */
export function publishShellJobs(input: readonly ShellJob[], now: number): void {
  const jobs = [...input].sort(byPipelineOrder);
  const capturing = jobs.some((job) => job.stage === 'recording' || job.stage === 'composing');
  if (
    capturing === current.capturing &&
    jobs.length === current.jobs.length &&
    jobs.every((job, index) => sameJob(job, current.jobs[index]))
  ) {
    // Same pixels: refresh freshness so the monitor skips its fetch.
    current = { ...current, publishedAt: now };
    return;
  }
  current = { jobs, capturing, publishedAt: now };
  for (const listener of listeners) listener();
}

/** Every live job from the three sources, unsorted. */
export function collectShellJobs(input: {
  videos: readonly Video[];
  matches?: readonly Match[];
  streams?: readonly StreamJob[];
}): ShellJob[] {
  return [
    ...input.videos.flatMap(reelJob),
    ...(input.matches ?? []).flatMap(parseJob),
    ...(input.streams ?? []).flatMap(streamJob),
  ];
}

export function subscribeToShellActivity(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function shellActivitySnapshot(): ShellActivity {
  return current;
}

/** SSR snapshot. Constant by construction: the server knows no job state. */
export function serverShellActivitySnapshot(): ShellActivity {
  return EMPTY;
}

export function shellActivityIsStale(now: number): boolean {
  // publishedAt 0 is stale: the monitor must fetch on its first tick.
  if (current.publishedAt === 0) return true;
  return now - current.publishedAt >= SHELL_ACTIVITY_MAX_AGE_MS;
}

/** Test seam and unmount cleanup: drops every job without notifying. */
export function resetShellActivity(): void {
  current = EMPTY;
}

function reelJob(video: Video): ShellJob[] {
  const stage = reelStage(video.status);
  if (stage === null) return [];
  const capture = video.captureProgress;
  let progress: ShellJob['progress'] = null;
  if (stage === 'recording' && capture !== undefined && capture.total > 0) {
    progress = { done: capture.done, total: capture.total };
    if (capture.percent !== undefined) {
      progress = { ...progress, percent: capture.percent };
    }
  }
  const href = video.jobId === undefined ? '/clips?vista=clips' : `/clips?partida=${encodeURIComponent(video.jobId)}`;
  return [{ id: video.id, kind: 'reel', title: video.title, stage, startedAt: video.createdAt, progress, href }];
}

function reelStage(status: Video['status']): ShellJobStage | null {
  if (status === 'queued' || status === 'recording' || status === 'composing') return status;
  return null;
}

function parseJob(match: Match): ShellJob[] {
  if (match.status === undefined || !PARSING_STATUSES.has(match.status)) return [];
  const title = match.player ? `${match.map} · parseo POV ${match.player}` : `${match.map} · parseo`;
  const startedAt = Date.parse(match.playedAt);
  return [
    {
      id: `parse:${match.id}`,
      kind: 'parse',
      title,
      stage: 'parsing',
      startedAt: Number.isNaN(startedAt) ? 0 : startedAt,
      progress: null,
      href: `/clips?partida=${encodeURIComponent(match.id)}`,
    },
  ];
}

function streamJob(job: StreamJob): ShellJob[] {
  let stage: ShellJobStage;
  if (job.status === 'acquiring') stage = 'acquiring';
  else if (job.status === 'rendering') stage = 'composing';
  else return [];
  const startedAt = Date.parse(job.updated_at ?? job.created_at);
  return [
    {
      id: `stream:${job.id}`,
      kind: 'stream',
      title: job.title?.trim() || 'Clip de stream',
      stage,
      startedAt: Number.isNaN(startedAt) ? 0 : startedAt,
      progress: null,
      href: `/streams/${encodeURIComponent(job.id)}`,
    },
  ];
}

/** Most advanced stage first, oldest first within a stage. */
function byPipelineOrder(a: ShellJob, b: ShellJob): number {
  const rank = STAGE_RANK[a.stage] - STAGE_RANK[b.stage];
  return rank !== 0 ? rank : a.startedAt - b.startedAt;
}

function sameJob(job: ShellJob, other: ShellJob | undefined): boolean {
  if (other === undefined) return false;
  return (
    job.id === other.id &&
    job.kind === other.kind &&
    job.stage === other.stage &&
    job.title === other.title &&
    job.progress?.done === other.progress?.done &&
    job.progress?.total === other.progress?.total &&
    job.progress?.percent === other.progress?.percent
  );
}
