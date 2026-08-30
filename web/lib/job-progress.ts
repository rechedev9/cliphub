import type { JobProgress } from './api/types.ts';

export type JobProgressRaw = {
  done?: number;
  total?: number;
  percent?: number;
  unit?: string;
  label?: string;
  stage?: string;
};

export function jobProgressPercent(progress: JobProgress): number {
  let raw = 0;
  if (progress.percent !== undefined) {
    raw = progress.percent;
  } else if (progress.total > 0) {
    raw = (progress.done / progress.total) * 100;
  }
  return Math.min(100, Math.max(0, Math.round(raw)));
}

export function jobProgressCount(progress: JobProgress): string {
  const unit = (progress.label ?? progress.unit ?? '').trim();
  const count = `${progress.done} / ${progress.total}`;
  return unit ? `${count} ${unit}` : count;
}

/** Percent and count only when a real snapshot exists. Never invent 0 / 0. */
export function jobProgressDisplay(progress?: JobProgress): { percent?: string; count?: string } {
  if (!progress) {
    return {};
  }
  return {
    percent: `${jobProgressPercent(progress)}%`,
    count: jobProgressCount(progress),
  };
}

export function parseJobProgress(raw: JobProgressRaw | undefined): JobProgress | undefined {
  if (raw === undefined || typeof raw.done !== 'number' || typeof raw.total !== 'number') {
    return undefined;
  }
  if (!Number.isFinite(raw.done) || !Number.isFinite(raw.total) || raw.total < 0 || raw.done < 0) {
    return undefined;
  }
  if (raw.total === 0 && raw.done !== 0) {
    return undefined;
  }
  const progress: JobProgress = { done: raw.done, total: raw.total };
  if (typeof raw.percent === 'number' && Number.isFinite(raw.percent)) {
    progress.percent = Math.min(100, Math.max(0, Math.round(raw.percent)));
  }
  if (typeof raw.unit === 'string' && raw.unit) progress.unit = raw.unit;
  if (typeof raw.label === 'string' && raw.label) progress.label = raw.label;
  if (typeof raw.stage === 'string' && raw.stage) progress.stage = raw.stage;
  return progress;
}
