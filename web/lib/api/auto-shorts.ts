// Auto-Shorts: one Short per great play, chosen by the orchestrator (GET /api/jobs/{id}/shorts-batch).
import { dataPlane } from './dataplane.ts';
import { pinRenderRevision, renderRevisionFromPrefix } from './render-revision.ts';
import { apiFetch } from './edition-fetch.ts';
import { reelPlaysSuffix, withAutoShortsCount } from './reel-identity.ts';
import { DEFAULT_VARIANT } from './reel-store.ts';
import type { RenderView } from './status-view.ts';
import type { Video } from './types.ts';

/** Mirrors internal/moments: the default bar and limit, and the hard cap. */
export const AUTO_SHORTS_DEFAULT_MIN_SCORE = 0.6;
export const AUTO_SHORTS_DEFAULT_LIMIT = 5;
export const AUTO_SHORTS_MAX_LIMIT = 10;

export const AUTO_SHORTS_LEVELS = [
  { minScore: 0.7, label: 'Solo las mejores', hint: 'Sobre todo cuádruples, aces y clutches ganados.' },
  { minScore: 0.6, label: 'Destacadas', hint: 'Triples, o dobles con algo más: clutch, headshots o peso en la ronda.' },
  { minScore: 0.5, label: 'Más jugadas', hint: 'También la mayoría de las dobles.' },
] as const;

export const AUTO_SHORTS_LIMITS = [3, 5, 8, AUTO_SHORTS_MAX_LIMIT] as const;

export type AutoShortsOptions = { minScore: number; limit: number };
export type AutoShortsBatch = AutoShortsOptions & { segmentIds: string[] };

const ARTIFACT_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function strings(value: unknown, pattern: RegExp): string[] | null {
  if (!Array.isArray(value)) return null;
  return value.filter((item): item is string => typeof item === 'string' && pattern.test(item));
}

/** The batch answer, or null when its shape is not the orchestrator's. */
export function parseShortsBatch(raw: unknown): AutoShortsBatch | null {
  if (!record(raw) || typeof raw.min_score !== 'number' || typeof raw.limit !== 'number') return null;
  const segmentIds = strings(raw.segment_ids, ARTIFACT_NAME);
  if (segmentIds === null) return null;
  return { minScore: raw.min_score, limit: raw.limit, segmentIds };
}

export function shortsBatchUrl(jobId: string, options: AutoShortsOptions): string {
  const query = new URLSearchParams({ min_score: String(options.minScore), limit: String(options.limit) });
  return `/api/demos/${encodeURIComponent(jobId)}/shorts-batch?${query.toString()}`;
}

export async function fetchShortsBatch(jobId: string, options: AutoShortsOptions & { signal?: AbortSignal }): Promise<AutoShortsBatch> {
  const response = await apiFetch(shortsBatchUrl(jobId, options), { cache: 'no-store', signal: options.signal });
  if (!response.ok) throw new Error(`shorts-batch ${response.status}`);
  const batch = parseShortsBatch(await response.json());
  if (batch === null) throw new Error('shorts-batch: unexpected answer');
  return batch;
}

/** One Short of a finished batch, addressed in the render revision that made it. */
export type AutoShortsFile = { name: string; url: string };

/** A batch render's Shorts; replaced when the style's newest render is another video's. */
export type AutoShortsFiles = { files: AutoShortsFile[]; replaced: boolean };

export function loteVideoNames(render: RenderView, lote: { jobId: string; id: string }): string[] {
  if (render.status !== 'ready' || render.editConfig?.separateShorts !== true || !render.segmentIds) return [];
  const current = `${lote.jobId}__${reelPlaysSuffix(render.segmentIds, render.editConfig)}`;
  return current === lote.id ? (render.videoNames ?? []) : [];
}

export function loteShortVideo(video: Video, short: string | null): Video {
  if (short === null || !ARTIFACT_NAME.test(short) || video.editConfig?.separateShorts !== true || video.jobId === undefined) return video;
  const url = dataPlane().videoUrl(video.jobId, video.variant ?? DEFAULT_VARIANT, short);
  return { ...video, downloadUrl: pinRenderRevision(url, video.artifactRevision), thumbnailUrl: undefined };
}

export function loteCardAfterDelete(video: Video, change: { deleted: string; firstUrl: string; left: number }): Video {
  const next: Video = { ...video, title: withAutoShortsCount(video.title, change.left), downloadUrl: pinRenderRevision(change.firstUrl, video.artifactRevision) };
  if (video.thumbnailUrl?.split('?', 1)[0]?.endsWith(`/covers/${change.deleted}`)) delete next.thumbnailUrl;
  return next;
}

/** Reads the Shorts of a batch from its GET /renders/{variant} state; none while it is not ready. */
export function autoShortsFiles(raw: unknown, target: { jobId: string; variant: string; artifactPrefix?: string }): AutoShortsFiles {
  if (!record(raw) || raw.status !== 'ready') return { files: [], replaced: false };
  const prefix = typeof raw.artifact_prefix === 'string' ? raw.artifact_prefix : undefined;
  if (target.artifactPrefix !== undefined && prefix !== target.artifactPrefix) return { files: [], replaced: true };
  const revision = renderRevisionFromPrefix(prefix, target.jobId, target.variant);
  const dp = dataPlane();
  const names = strings(raw.videos, ARTIFACT_NAME) ?? [];
  return { files: names.map((name) => ({ name, url: pinRenderRevision(dp.videoUrl(target.jobId, target.variant, name), revision) })), replaced: false };
}
