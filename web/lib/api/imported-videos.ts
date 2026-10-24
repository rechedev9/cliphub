// Shorts an older Studio downloaded from the retired ClipHub cloud, listed in Resultados as ready local videos.

import { DEFAULT_EDIT_CONFIG } from './reel-store.ts';
import type { IndexedJob } from './jobs-index.ts';
import type { Video } from './types.ts';

export type ImportedVideo = {
  cloudJobId: string;
  name: string;
  title: string;
  localJobId?: string;
  createdAt: number;
};

const ID_PREFIX = 'imported__';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** `GET /api/imported-videos`; malformed rows are dropped, never guessed. */
export function parseImportedVideos(raw: unknown): ImportedVideo[] {
  const list = isRecord(raw) ? raw.videos : undefined;
  if (!Array.isArray(list)) return [];
  const out: ImportedVideo[] = [];
  for (const row of list) {
    if (!isRecord(row)) continue;
    const { cloud_job_id: cloudJobId, name, title, local_job_id: localJobId, created_at: createdAt } = row;
    if (typeof cloudJobId !== 'string' || typeof name !== 'string' || typeof title !== 'string') continue;
    const created = typeof createdAt === 'string' ? Date.parse(createdAt) : Number.NaN;
    out.push({
      cloudJobId,
      name,
      title,
      ...(typeof localJobId === 'string' && localJobId !== '' ? { localJobId } : {}),
      createdAt: Number.isFinite(created) ? created : 0,
    });
  }
  return out;
}

export function importedVideoId(video: Pick<ImportedVideo, 'cloudJobId' | 'name'>): string {
  return `${ID_PREFIX}${video.cloudJobId}__${video.name}`;
}

export function isImportedVideo(video: Pick<Video, 'id'>): boolean {
  return video.id.startsWith(ID_PREFIX);
}

/** A ready, read-only Short. It takes the map of the demo it came from while that demo is still on this PC. */
export function importedVideo(video: ImportedVideo, downloadUrl: string, jobs: readonly IndexedJob[]): Video {
  const job = video.localJobId === undefined ? undefined : jobs.find((candidate) => candidate.jobId === video.localJobId);
  const out: Video = {
    id: importedVideoId(video),
    title: video.title,
    map: job?.summary?.match?.map ?? 'Unknown',
    score: '',
    mode: 'clean',
    editConfig: { ...DEFAULT_EDIT_CONFIG, format: 'short-9x16', matchRecap: false },
    status: 'ready',
    recovered: true,
    publishable: false,
    createdAt: video.createdAt,
    downloadUrl,
  };
  if (job?.summary?.target?.name) out.targetName = job.summary.target.name;
  return out;
}
