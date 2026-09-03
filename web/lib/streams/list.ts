import type { StreamJob } from '../api/streams.ts';
import type { PollCadence } from '../poll-loop.ts';

export type StreamJobTagTone = 'warning' | 'success' | 'stream' | 'danger';

export type StreamJobTag = { label: string; tone: StreamJobTagTone; busy: boolean };

/**
 * Row state chip for the /streams list; `busy` adds the activity ring.
 * The list only knows job status: whether a render is stale against its plan is
 * shown in the editor, which is the only place that holds both.
 */
export function streamJobTag(job: StreamJob): StreamJobTag {
  switch (job.status) {
    case 'rendered':
      return { label: 'Renderizado', tone: 'success', busy: false };
    case 'rendering':
      return { label: 'Render', tone: 'stream', busy: true };
    case 'failed':
      return { label: 'Falló', tone: 'danger', busy: false };
    case 'acquiring':
      return { label: 'Trayendo el vídeo', tone: 'stream', busy: true };
    default:
      return { label: 'Borrador', tone: 'warning', busy: false };
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
