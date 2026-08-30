import type { CaptureProgress } from '@/lib/api/types';
import { parseJobProgress, type JobProgressRaw } from './job-progress.ts';

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

export function parseCaptureProgress(raw: JobProgressRaw | undefined): CaptureProgress | undefined {
  return parseJobProgress(raw);
}
