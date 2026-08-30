import type { Video } from './api/types.ts';

/** Live jobs for the command-strip transport and the html capture-active gate. */

/** Mirrors `capturing` onto <html>; the CSS depth/animation gates key off it. */
export const CAPTURE_ACTIVE_ATTRIBUTE = 'data-capture-active';

/** A push is authoritative for this long before the shell polls for itself. */
const SHELL_ACTIVITY_MAX_AGE_MS = 4000;

/** The non-terminal reel states, in the order the pipeline runs them. */
export type ShellJobStage = 'queued' | 'recording' | 'composing';

export interface ShellJob {
  readonly id: string;
  readonly title: string;
  readonly stage: ShellJobStage;
  /** Epoch ms the reel was created — real API data, used for elapsed time. */
  readonly startedAt: number;
  /** Worker done/total/percent while recording or composing; null when none yet. */
  readonly progress: { readonly done: number; readonly total: number; readonly percent?: number } | null;
}

export interface ShellActivity {
  /** Non-terminal reels, most advanced first. */
  readonly jobs: readonly ShellJob[];
  /** True while a reel is on the GPU (recording or composing), not merely queued. */
  readonly capturing: boolean;
  /** Epoch ms of the last push; 0 means nothing has ever published. */
  readonly publishedAt: number;
}

const EMPTY: ShellActivity = { jobs: [], capturing: false, publishedAt: 0 };

const STAGE_RANK: Record<ShellJobStage, number> = { recording: 0, composing: 1, queued: 2 };

let current: ShellActivity = EMPTY;
const listeners = new Set<() => void>();

export function publishShellActivity(videos: readonly Video[], now: number): void {
  const jobs = videos.flatMap(toShellJob).sort(byPipelineOrder);
  const capturing = jobs.some((job) => job.stage !== 'queued');
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

function toShellJob(video: Video): ShellJob[] {
  const stage = toStage(video.status);
  if (stage === null) return [];
  const capture = video.captureProgress;
  let progress: ShellJob['progress'] = null;
  if ((stage === 'recording' || stage === 'composing') && capture !== undefined && capture.total > 0) {
    progress = { done: capture.done, total: capture.total };
    if (capture.percent !== undefined) {
      progress = { ...progress, percent: capture.percent };
    }
  }
  return [{ id: video.id, title: video.title, stage, startedAt: video.createdAt, progress }];
}

function toStage(status: Video['status']): ShellJobStage | null {
  if (status === 'queued' || status === 'recording' || status === 'composing') return status;
  return null;
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
    job.stage === other.stage &&
    job.title === other.title &&
    job.progress?.done === other.progress?.done &&
    job.progress?.total === other.progress?.total &&
    job.progress?.percent === other.progress?.percent
  );
}
