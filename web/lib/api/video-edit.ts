import { currentFullDemoOptions, responseJSON, type FullDemoOptions } from '../full-demo-plan.ts';
import type { CaptureCheck, SpaceCheck } from '../produce/edit-capture.ts';
import { projectHorizontalDraft, type HorizontalDraft } from '../produce/horizontal.ts';
import { apiFetch } from './edition-fetch.ts';

/** One finished render of a video (Versiones in Mis vídeos); `videos` are MP4 names without the extension. */
export type VideoVersion = {
  revision: string;
  createdAt: string;
  current: boolean;
  videos: string[];
  segmentIds: string[];
  edit: unknown;
  music: unknown;
  planId: string | null;
  rounds: number[];
};

const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;
const NAME = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;
const VARIANT = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;

function field(body: unknown, key: string): unknown {
  return typeof body === 'object' && body !== null ? Reflect.get(body, key) : undefined;
}

function strings(value: unknown, pattern: RegExp): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && pattern.test(item)) : [];
}

function count(value: unknown): number | null {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : null;
}

/** Orchestrator `GET .../versions` body, newest first; a malformed version drops. */
export function projectVideoVersions(body: unknown): VideoVersion[] {
  const raw = field(body, 'versions');
  const versions: VideoVersion[] = [];
  for (const row of Array.isArray(raw) ? raw : []) {
    const revision = field(row, 'revision');
    const createdAt = field(row, 'created_at');
    if (typeof revision !== 'string' || !UUID.test(revision) || typeof createdAt !== 'string') continue;
    const planId = field(row, 'plan_id');
    const rounds = field(row, 'rounds');
    versions.push({
      revision,
      createdAt,
      current: field(row, 'current') === true,
      videos: strings(field(row, 'videos'), NAME),
      segmentIds: strings(field(row, 'segment_ids'), NAME),
      edit: field(row, 'edit'),
      music: field(row, 'music'),
      planId: typeof planId === 'string' && UUID.test(planId) ? planId : null,
      rounds: Array.isArray(rounds) ? rounds.filter((round): round is number => count(round) !== null) : [],
    });
  }
  return versions;
}

export function projectSpace(body: unknown): SpaceCheck | null {
  const freeBytes = count(field(body, 'free_bytes'));
  const neededBytes = count(field(body, 'needed_bytes'));
  if (freeBytes === null || neededBytes === null) return null;
  return { freeBytes, neededBytes, short: field(body, 'short') === true };
}

function cameraIds(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((id): id is string => typeof id === 'string') : [];
}

/** Orchestrator capture check (also `capture` in the plan envelope); null when malformed. */
export function projectCaptureCheck(body: unknown): CaptureCheck | null {
  const needed = field(body, 'needed');
  const missing = field(body, 'missing_rounds');
  if (typeof needed !== 'boolean' || !Array.isArray(missing)) return null;
  const missingCameras = cameraIds(field(body, 'missing_cameras'));
  const omittedCameras = cameraIds(field(body, 'omitted_cameras'));
  return {
    needed,
    missingRounds: missing.filter((round): round is number => count(round) !== null),
    ...(missingCameras.length > 0 ? { missingCameras } : {}),
    ...(omittedCameras.length > 0 ? { omittedCameras } : {}),
    space: projectSpace(field(body, 'space')),
  };
}

function versionsUrl(jobId: string, variant: string): string {
  if (!UUID.test(jobId) || !VARIANT.test(variant)) throw new Error('Vídeo desconocido.');
  return `/api/demos/${jobId}/renders/${variant}/versions`;
}

type VersionRef = { jobId: string; variant: string };

export async function listVideoVersions({ jobId, variant, signal }: VersionRef & { signal?: AbortSignal }): Promise<VideoVersion[]> {
  return projectVideoVersions(await responseJSON(await apiFetch(versionsUrl(jobId, variant), { cache: 'no-store', signal })));
}

/** Crear horizontal: the 16:9 summary of a Short's current version; it saves nothing. */
export async function fetchHorizontalDraft({ jobId, variant, signal }: VersionRef & { signal?: AbortSignal }): Promise<HorizontalDraft | null> {
  if (!UUID.test(jobId) || !VARIANT.test(variant)) throw new Error('Vídeo desconocido.');
  return projectHorizontalDraft(await responseJSON(await apiFetch(`/api/demos/${jobId}/renders/${variant}/horizontal`, { cache: 'no-store', signal })));
}

/** The current version cannot be deleted alone: the server answers 409 `version_current`. */
export async function deleteVideoVersion({ jobId, variant, revision }: VersionRef & { revision: string }): Promise<void> {
  if (!UUID.test(revision)) throw new Error('Versión desconocida.');
  await responseJSON(await apiFetch(`${versionsUrl(jobId, variant)}/${revision}`, { method: 'DELETE' }));
}

/** The MP4 of one version, pinned to its revision. */
export function versionVideoUrl({ jobId, variant, revision, name }: VersionRef & { revision: string; name: string }): string {
  return `/api/demos/${encodeURIComponent(jobId)}/renders/${encodeURIComponent(variant)}/revisions/${encodeURIComponent(revision)}/videos/${encodeURIComponent(name)}`;
}

/** Plans the options without saving them and says which rounds the edit would record. */
export async function checkFullDemoCapture({ jobId, options, signal }: { jobId: string; options: FullDemoOptions; signal?: AbortSignal }): Promise<CaptureCheck | null> {
  if (!UUID.test(jobId)) throw new Error('Partida desconocida.');
  const response = await apiFetch(`/api/demos/${jobId}/full-demo/capture-check`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ options: currentFullDemoOptions(options) }),
    signal,
  });
  return projectCaptureCheck(await responseJSON(response));
}

/** Free space against a capture and render of this length; null when Studio cannot read the disk. */
export async function fetchStorageSpace({ captureSeconds, renderSeconds, signal }: { captureSeconds: number; renderSeconds: number; signal?: AbortSignal }): Promise<SpaceCheck | null> {
  const params = new URLSearchParams({ capture_seconds: String(Math.max(0, Math.round(captureSeconds))), render_seconds: String(Math.max(0, Math.round(renderSeconds))) });
  return projectSpace(field(await responseJSON(await apiFetch(`/api/demos/storage/space?${params.toString()}`, { cache: 'no-store', signal })), 'space'));
}

/** Sets how many rendered Full Demos keep their captures; returns the bytes freed from the older ones. */
export async function setCaptureRetention(keep: number): Promise<number> {
  const response = await apiFetch('/api/demos/storage/capture-retention', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keep }),
  });
  return count(field(await responseJSON(response), 'freed_bytes')) ?? 0;
}
