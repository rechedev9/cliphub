import type { StreamJob } from '../api/streams.ts';

/** Never guess characters already lost by an older import; use known project metadata. */
export function streamTitle(job: Pick<StreamJob, 'title' | 'source_url' | 'edit_plan'>): string {
  const title = readableTitle(job.title);
  if (title) return title;
  const clipTitle = job.edit_plan?.clips.map((clip) => readableTitle(clip.title)).find(Boolean);
  return clipTitle || 'Clip de stream';
}

function readableTitle(raw: string | undefined): string {
  const title = raw?.trim() ?? '';
  // U+FFFD means that the original byte was discarded. Keep persisted data
  // untouched so a user can rename it, instead of silently inventing a title.
  return title.includes('\uFFFD') ? '' : title;
}
