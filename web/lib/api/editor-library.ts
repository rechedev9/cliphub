import type { StreamEditPlan, StreamJob } from './streams.ts';
import type { Video } from './types.ts';

export const IMPORT_SOURCE = {
  demo: 'demo',
  stream: 'stream',
} as const;

export type ImportSource = (typeof IMPORT_SOURCE)[keyof typeof IMPORT_SOURCE];

export type ImportableRender = {
  source: ImportSource;
  job_id: string;
  variant?: string;
  name?: string;
  title: string;
};

const DEMO_IMPORTABLE_STATUS = {
  ready: 'ready',
  reviewRequired: 'review_required',
} as const;

const STREAM_IMPORTABLE_STATUS = {
  rendered: 'rendered',
} as const;

const DEMO_VIDEO_REF_RE =
  /^\/api\/demos\/([0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12})\/renders\/([^/?#]+)\/videos\/([^/?#]+)/i;

export type DemoLibraryVideo = Pick<Video, 'jobId' | 'title' | 'variant' | 'status' | 'downloadUrl'>;
export type StreamLibraryJob = Pick<StreamJob, 'id' | 'status' | 'title' | 'edit_plan'>;

export function parseDemoVideoRef(downloadUrl: string): { job_id: string; variant: string; name: string } | null {
  const match = DEMO_VIDEO_REF_RE.exec(downloadUrl);
  if (!match) return null;
  const jobId = match[1];
  const variant = match[2];
  const name = match[3];
  if (!jobId || !variant || !name) return null;
  return { job_id: jobId, variant, name };
}

export function mapImportableRenders(input: {
  videos?: readonly DemoLibraryVideo[];
  streamJobs?: readonly StreamLibraryJob[];
}): ImportableRender[] {
  const out: ImportableRender[] = [];
  for (const video of input.videos ?? []) {
    const mapped = mapDemoVideo(video);
    if (mapped) out.push(mapped);
  }
  for (const job of input.streamJobs ?? []) {
    out.push(...mapStreamJob(job));
  }
  return out;
}

export async function listImportableRenders(): Promise<ImportableRender[]> {
  const [{ api }, { streamsApi }] = await Promise.all([import('./index.ts'), import('./streams.ts')]);
  const [videos, streamJobs] = await Promise.all([api.listVideos(), streamsApi.listJobs()]);
  return mapImportableRenders({ videos, streamJobs });
}

function isDemoImportable(status: string): boolean {
  return status === DEMO_IMPORTABLE_STATUS.ready || status === DEMO_IMPORTABLE_STATUS.reviewRequired;
}

function mapDemoVideo(video: DemoLibraryVideo): ImportableRender | null {
  if (!isDemoImportable(video.status)) return null;
  const parsed = video.downloadUrl ? parseDemoVideoRef(video.downloadUrl) : null;
  const jobId = parsed?.job_id ?? video.jobId;
  const variant = parsed?.variant ?? video.variant;
  const name = parsed?.name;
  if (!jobId || !name) return null;
  const item: ImportableRender = {
    source: IMPORT_SOURCE.demo,
    job_id: jobId,
    name,
    title: video.title,
  };
  if (variant) item.variant = variant;
  return item;
}

function mapStreamJob(job: StreamLibraryJob): ImportableRender[] {
  if (job.status !== STREAM_IMPORTABLE_STATUS.rendered) return [];
  const plan = job.edit_plan;
  const clips = plan?.clips ?? [];
  if (clips.length === 0) return [];
  return clips.flatMap((clip) => mapStreamClip(job, plan, clip));
}

function mapStreamClip(
  job: StreamLibraryJob,
  plan: StreamEditPlan | undefined,
  clip: { id: string; title?: string },
): ImportableRender[] {
  if (!clip.id) return [];
  const item: ImportableRender = {
    source: IMPORT_SOURCE.stream,
    job_id: job.id,
    name: clip.id,
    title: clip.title || job.title || clip.id,
  };
  if (plan?.variant) item.variant = plan.variant;
  return [item];
}
