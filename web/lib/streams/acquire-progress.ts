import type { StreamDownloadProgress } from '../api/streams.ts';
import { formatBytes } from '../storage.ts';

export type AcquireProgressView = {
  stage: string;
  /** Whole percent for `LongOperation`; absent keeps the bar indeterminate. */
  percent?: number;
  detail: string;
};

const EDITOR_OPENS = 'El editor se abre al terminar';

/** "quedan 3 min", "queda menos de 1 min", "quedan 1 h 05 min". */
export function formatTimeLeft(seconds: number): string {
  const minutes = Math.ceil(seconds / 60);
  if (minutes <= 1) return 'queda menos de 1 min';
  if (minutes < 60) return `quedan ${minutes} min`;
  return `quedan ${Math.floor(minutes / 60)} h ${String(minutes % 60).padStart(2, '0')} min`;
}

/** What the "Trayendo el vídeo" card shows for the latest yt-dlp update. */
export function acquireProgressView(progress: StreamDownloadProgress | undefined): AcquireProgressView {
  if (!progress) return { stage: 'Descargando', detail: EDITOR_OPENS };
  // Floor, so 99,6 never reads as a finished download.
  const percent = progress.percent === undefined ? undefined : Math.floor(progress.percent);
  // A finished track: yt-dlp merges and the worker checks the file, with no number to show.
  if (percent !== undefined && percent >= 100) return { stage: 'Preparando el vídeo', detail: EDITOR_OPENS };

  const total = progress.total_bytes ?? 0;
  const size =
    total > 0
      ? `${formatBytes(progress.downloaded_bytes)} de ${progress.total_estimated ? '~' : ''}${formatBytes(total)}`
      : `${formatBytes(progress.downloaded_bytes)} descargados`;
  const eta = percent !== undefined && progress.eta_seconds ? formatTimeLeft(progress.eta_seconds) : null;
  return {
    stage: progress.audio_only ? 'Descargando audio' : 'Descargando',
    percent,
    detail: eta ? `${size} · ${eta}` : size,
  };
}
