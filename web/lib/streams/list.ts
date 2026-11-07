import type { StreamJob } from '../api/streams.ts';
import type { PollCadence } from '../poll-loop.ts';

export type StreamJobTagTone = 'neutral' | 'success' | 'stream' | 'danger';

export type StreamJobTag = { label: string; tone: StreamJobTagTone; busy: boolean };

/**
 * Row state chip for the /streams list; `busy` adds the activity ring.
 * The list only knows job status: whether a render is stale against its plan is
 * shown in the editor, which is the only place that holds both.
 */
export function streamJobTag(job: StreamJob): StreamJobTag {
  switch (job.status) {
    case 'rendered':
      return { label: 'Listo', tone: 'success', busy: false };
    case 'rendering':
      return { label: 'Exportando', tone: 'stream', busy: true };
    case 'failed':
      return { label: 'Error', tone: 'danger', busy: false };
    case 'acquiring':
      return { label: 'Descargando', tone: 'stream', busy: true };
    default:
      return { label: 'En edición', tone: 'neutral', busy: false };
  }
}

/** Poll quickly while any job is still moving. */
export function streamListCadence(jobs: readonly StreamJob[]): PollCadence {
  return jobs.some((job) => job.status === 'acquiring' || job.status === 'rendering') ? 'fast' : 'idle';
}

/** Newest first, so a job just created lands at the top of the list. */
export function sortStreamJobs(jobs: readonly StreamJob[]): StreamJob[] {
  return [...jobs].sort(
    (a, b) => Date.parse(b.updated_at ?? b.created_at) - Date.parse(a.updated_at ?? a.created_at),
  );
}

/** Cuts in a listed job: the list row's `clip_count`, else the plan a full job carries. */
export function streamClipCount(job: Pick<StreamJob, 'clip_count' | 'edit_plan'>): number {
  if (typeof job.clip_count === 'number') return job.clip_count;
  return job.edit_plan?.clips.length ?? 0;
}

/** One cut of a project, as percents of the source so a poster bar can place it. */
export type StreamCutMark = { id: string; leftPercent: number; widthPercent: number };

/** Null when the job lacks the plan or the source length: the list endpoint sends only `clip_count`. */
export function streamCutMarks(job: Pick<StreamJob, 'edit_plan' | 'probe'>): StreamCutMark[] | null {
  const duration = job.probe?.duration_seconds;
  const clips = job.edit_plan?.clips;
  if (clips === undefined || clips.length === 0 || duration === undefined || !Number.isFinite(duration) || duration <= 0) {
    return null;
  }
  const percent = (seconds: number): number => Math.min(100, Math.max(0, (seconds / duration) * 100));
  return clips.map((clip) => {
    const leftPercent = percent(clip.start_seconds);
    return { id: clip.id, leftPercent, widthPercent: Math.max(0, percent(clip.end_seconds) - leftPercent) };
  });
}
