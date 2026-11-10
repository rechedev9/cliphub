/**
 * Which of the library's jobs have a finished video, for the POV chip and "Con POV".
 * It reads the batch status of each job and preset, the same rows the MCP reads, and
 * never the Videos list, whose reconcile would run every time Partidas opens.
 */

/** Pairs per request; the batch-status route rejects more than 100. */
export const POV_BATCH_ITEMS = 100;

type StatusRow = { job_id?: unknown; render?: unknown; error?: unknown };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

/** "job:preset" pairs for every job and preset. */
export function povItems(jobIds: readonly string[], presets: readonly string[]): string[] {
  return jobIds.flatMap((jobId) => presets.map((preset) => `${jobId}:${preset}`));
}

/** The jobs with a ready render that wrote at least one video. */
export function readyVideoJobs(rows: readonly unknown[]): Set<string> {
  const jobs = new Set<string>();
  for (const raw of rows) {
    if (!isRecord(raw)) continue;
    const row: StatusRow = raw;
    const render = row.render;
    if (typeof row.job_id !== 'string' || row.error || !isRecord(render)) continue;
    const ready = render.status === 'ready' || render.status === 'review_required';
    if (ready && Array.isArray(render.videos) && render.videos.length > 0) jobs.add(row.job_id);
  }
  return jobs;
}

async function readJson(path: string): Promise<unknown> {
  const res = await fetch(path, { cache: 'no-store' });
  if (!res.ok) throw new Error(`${path} answered ${res.status}`);
  return res.json();
}

function listOf(body: unknown, key: string): unknown[] {
  if (!isRecord(body)) return [];
  const list = body[key];
  return Array.isArray(list) ? list : [];
}

/** The jobs among `jobIds` that have a finished video; an unreadable answer reads as none. */
export async function loadPovJobs(jobIds: readonly string[]): Promise<Set<string>> {
  if (jobIds.length === 0) return new Set();
  try {
    const presets = listOf(await readJson('/api/presets'), 'presets').flatMap((preset) =>
      isRecord(preset) && typeof preset.name === 'string' ? [preset.name] : [],
    );
    const items = povItems(jobIds, presets);
    const found = new Set<string>();
    for (let from = 0; from < items.length; from += POV_BATCH_ITEMS) {
      const chunk = items.slice(from, from + POV_BATCH_ITEMS).join(',');
      const body = await readJson(`/api/demos/batch-status?items=${encodeURIComponent(chunk)}`);
      for (const job of readyVideoJobs(listOf(body, 'items'))) found.add(job);
    }
    return found;
  } catch {
    return new Set();
  }
}
