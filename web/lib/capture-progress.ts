import type { CaptureProgress } from '@/lib/api/types';

export function captureProgressPercent(progress: CaptureProgress): number {
  let raw = 0;
  if (progress.percent !== undefined) {
    raw = progress.percent;
  } else if (progress.total > 0) {
    raw = (progress.done / progress.total) * 100;
  }
  return Math.min(100, Math.max(0, Math.round(raw)));
}

export function captureProgressDetail(progress: CaptureProgress): string {
  if (progress.total <= 0) return 'Preparando captura local';
  if (progress.done >= progress.total) {
    return 'Validando captura local';
  }
  if (captureProgressPercent(progress) === 0 && progress.done === 0) {
    return 'Preparando captura local';
  }
  return `Capturando ${progress.done + 1}/${progress.total}`;
}

export function parseCaptureProgress(raw: {
  done?: number;
  total?: number;
  percent?: number;
} | undefined): CaptureProgress | undefined {
  if (raw === undefined || typeof raw.done !== 'number' || typeof raw.total !== 'number' || raw.total <= 0) {
    return undefined;
  }
  const progress: CaptureProgress = { done: raw.done, total: raw.total };
  if (typeof raw.percent === 'number' && Number.isFinite(raw.percent)) {
    progress.percent = Math.min(100, Math.max(0, Math.round(raw.percent)));
  }
  return progress;
}
