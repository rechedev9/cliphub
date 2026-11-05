import type { EditConfig, JobStatusView } from './types.ts';
import type { RenderStatus } from './reel-reconcile.ts';
import { parseEffectiveEditConfig, parseEffectiveRenderMusic, type EffectiveRenderMusic } from './render-hydration.ts';
import { parseCaptureProgress } from '../capture-progress.ts';

/** `/status` and batch-status job half, as the proxy whitelists it. */
export type RawStatusView = {
  status: string;
  failure_reason?: string;
  failure_code?: string;
  progress?: { done?: number; total?: number; percent?: number; stage?: string };
};

/** `/renders/{variant}` and batch-status render half. */
export type RawRenderView = {
  status?: string;
  /** `renderplan.RenderVariantState.Error`: the render failure text. */
  error?: string;
  warnings?: string[];
  videos?: string[];
  covers?: string[];
  artifact_prefix?: string;
  segment_ids?: unknown;
  music?: unknown;
  edit?: {
    format?: EditConfig['format'];
    killEffect?: EditConfig['killEffect'];
    transition?: EditConfig['transition'];
    intro?: boolean;
    outro?: boolean;
    hook_text?: boolean;
    kill_counter?: boolean;
    centered_killfeed?: boolean;
    cover_strategy?: EditConfig['coverStrategy'];
    intro_text?: string;
    outro_text?: string;
  };
};

export type RenderView = {
  status: RenderStatus;
  failureReason?: string;
  warnings?: string[];
  videoName?: string;
  coverName?: string;
  coverNames?: string[];
  artifactPrefix?: string;
  segmentIds?: string[];
  editConfig?: EditConfig;
  effectiveMusic?: EffectiveRenderMusic;
};

export function parseStatusView(data: RawStatusView): JobStatusView {
  const full: JobStatusView = { status: data.status };
  if (data.failure_reason) full.failureReason = data.failure_reason;
  if (data.failure_code) full.failureCode = data.failure_code;
  const parsed = parseCaptureProgress(data.progress);
  if (parsed) full.captureProgress = parsed;
  return full;
}

export const KNOWN_RENDER_STATUSES = new Set<RenderStatus>(['queued', 'rendering', 'ready', 'review_required', 'failed']);

export function parseRenderView(data: RawRenderView): RenderView {
  const status: RenderStatus =
    data.status && KNOWN_RENDER_STATUSES.has(data.status as RenderStatus) ? (data.status as RenderStatus) : 'none';
  const coverNames = Array.isArray(data.covers)
    ? data.covers.filter((name): name is string => typeof name === 'string' && name.length > 0)
    : undefined;
  const segmentIds = Array.isArray(data.segment_ids)
    ? data.segment_ids.filter((id): id is string => typeof id === 'string' && id.length > 0)
    : undefined;
  return {
    status,
    failureReason: data.error,
    warnings: data.warnings,
    videoName: data.videos?.[0],
    coverName: coverNames?.[0],
    coverNames,
    artifactPrefix: data.artifact_prefix,
    segmentIds,
    editConfig: parseEffectiveEditConfig(data.edit),
    effectiveMusic: parseEffectiveRenderMusic(data.music),
  };
}

/**
 * The item-level read failure of one `/batch-status` row, or undefined when the
 * row was read cleanly. The server sends a message string; an object body is
 * still read as a failure so a shape change degrades to "unknown", never to
 * "job gone" (which latches the reel) or "no render yet" (which re-drives it).
 */
export function batchItemError(raw: unknown, code: unknown): string | undefined {
  if (typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'boolean') {
    const text = String(raw).trim();
    return text === '' || text === 'false' ? undefined : text;
  }
  if (typeof raw === 'object' && raw !== null) {
    const fields = raw as { message?: unknown; error?: unknown; code?: unknown };
    for (const field of [fields.message, fields.error, fields.code]) {
      if (typeof field === 'string' && field.trim() !== '') return field.trim();
    }
    return 'batch-status could not read this reel';
  }
  if (typeof code === 'string' && code.trim() !== '') return code.trim();
  return undefined;
}
