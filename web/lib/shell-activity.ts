import type { Video } from './api/types.ts';

/**
 * The shell's view of what the machine is doing right now.
 *
 * FragForge's whole premise is long-running local capture and render jobs, and
 * until this module existed the chrome never showed one: `videos/page.tsx`
 * computed `hasActiveReel` every 1.5s and threw it away, so navigating to Feed
 * mid-render told the user nothing, and the `html[data-capture-active]` gate in
 * globals.css had no writer at all.
 *
 * Two consumers, one store:
 *   - the command strip's transport renders the leading job;
 *   - `ShellActivityMonitor` mirrors `capturing` onto <html>, which is what
 *     stops every ambient effect from contending with cs2.exe for the GPU.
 *
 * PUSHING FROM A PAGE THAT ALREADY POLLS. Any page holding fresh `Video[]` from
 * `api.listVideos()` should call `publishShellActivity(videos, Date.now())`
 * right where it calls `setVideos(...)`. The monitor treats a recent push as
 * authoritative and skips its own fetch, so the Library's existing 1.5s loop is
 * reused rather than duplicated. Nothing breaks if a page never pushes — the
 * monitor just polls for itself.
 */

/** Mirrors `capturing` onto <html>; the CSS depth/animation gates key off it. */
export const CAPTURE_ACTIVE_ATTRIBUTE = 'data-capture-active';

/** A push is authoritative for this long before the shell polls for itself. */
export const SHELL_ACTIVITY_MAX_AGE_MS = 4000;

/** The non-terminal reel states, in the order the pipeline runs them. */
export type ShellJobStage = 'queued' | 'recording' | 'composing';

export interface ShellJob {
  readonly id: string;
  readonly title: string;
  readonly stage: ShellJobStage;
  /** Epoch ms the reel was created — real API data, used for elapsed time. */
  readonly startedAt: number;
  /**
   * Real capture progress, or null. The orchestrator only reports segments
   * while a reel is `recording`; every other stage is genuinely indeterminate
   * and must render as such rather than as an invented percentage
   * (design.md "Never fabricate progress").
   */
  readonly progress: { readonly done: number; readonly total: number } | null;
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
    // Identical payload: refresh the freshness stamp so the monitor keeps
    // skipping its own fetch, but do not wake every subscriber 40 times a
    // minute for a snapshot that renders the same pixels.
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
  // publishedAt 0 means nothing has ever pushed, which is stale by definition
  // rather than by arithmetic — the monitor must fetch on its very first tick.
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
  const progress =
    stage === 'recording' && capture !== undefined && capture.total > 0
      ? { done: capture.done, total: capture.total }
      : null;
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
    job.progress?.total === other.progress?.total
  );
}
