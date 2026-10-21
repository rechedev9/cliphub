import type { StorageUsage } from './api/storage.ts';
import { SERVICE_UNAVAILABLE_CODE, type Match } from './api/types.ts';
import { DELETE_OFFLINE_MESSAGE } from './delete-error.ts';
import { prettyMapName } from './format.ts';

export const STORAGE_LOAD_ERROR_MESSAGE = 'No se pudo leer el espacio usado. Inténtalo de nuevo.';

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;
const BYTE_NUMBER = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 1 });

/** "31,2 GB": 1024-based like Windows Explorer, one decimal at most. */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  let value = bytes;
  let unit = 0;
  // Compare the rounded value so 1023,96 MB reads "1 GB", never "1024 MB".
  while (unit < BYTE_UNITS.length - 1 && Math.round(value * 10) / 10 >= 1024) {
    value /= 1024;
    unit += 1;
  }
  return `${BYTE_NUMBER.format(unit === 0 ? Math.round(value) : value)} ${BYTE_UNITS[unit]}`;
}

export type FreeableJob = {
  jobId: string;
  label: string;
  captureBytes: number;
  renderBytes: number;
};

function jobLabel(jobId: string, match: Pick<Match, 'map' | 'player'> | undefined): string {
  if (match === undefined) return `Vídeo ${jobId.slice(0, 8)}`;
  const map = prettyMapName(match.map);
  return match.player ? `${map} · ${match.player}` : map;
}

/** Jobs whose captures a ready render covers, biggest first, named after the demo list's match. */
export function freeableJobs(
  usage: StorageUsage,
  matches: ReadonlyArray<Pick<Match, 'id' | 'map' | 'player'>>,
): FreeableJob[] {
  const byId = new Map(matches.map((match) => [match.id, match]));
  return usage.jobs
    .filter((job) => job.freeable)
    .sort((a, b) => b.captureBytes - a.captureBytes)
    .map((job) => ({
      jobId: job.jobId,
      label: jobLabel(job.jobId, byId.get(job.jobId)),
      captureBytes: job.captureBytes,
      renderBytes: job.renderBytes,
    }));
}

export function storageLoadErrorMessage(err: unknown): string {
  const code: unknown = typeof err === 'object' && err !== null ? Reflect.get(err, 'code') : undefined;
  return code === SERVICE_UNAVAILABLE_CODE ? DELETE_OFFLINE_MESSAGE : STORAGE_LOAD_ERROR_MESSAGE;
}
