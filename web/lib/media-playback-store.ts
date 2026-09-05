import type { MediaPlaybackItem } from './api/playback.ts';

export const MEDIA_PLAYBACK_STORAGE_KEY = 'cliphub.media-playback.v1';
const MAX_ENTRIES = 50;
const MAX_STORAGE_CHARS = 128 * 1024;

export type MediaPlaybackProgress = {
  position: number;
  volume: number;
  muted: boolean;
  loop: boolean;
  playbackRate: number;
  updatedAt: number;
};

type PlaybackStore = { version: 1; entries: Record<string, MediaPlaybackProgress> };

export function mediaPlaybackKey(item: MediaPlaybackItem): string {
  return `${item.source}:${item.jobId}:${item.variant}:${item.artifactName}:${item.revision}`;
}

function parseProgress(value: unknown): MediaPlaybackProgress | null {
  if (typeof value !== 'object' || value === null) return null;
  const candidate = value as Partial<MediaPlaybackProgress>;
  if (
    typeof candidate.position !== 'number' || !Number.isFinite(candidate.position) || candidate.position < 0 ||
    typeof candidate.volume !== 'number' || !Number.isFinite(candidate.volume) || candidate.volume < 0 || candidate.volume > 1 ||
    typeof candidate.muted !== 'boolean' || typeof candidate.loop !== 'boolean' ||
    typeof candidate.updatedAt !== 'number' || !Number.isFinite(candidate.updatedAt)
  ) return null;
  const playbackRate = candidate.playbackRate ?? 1;
  if (!Number.isFinite(playbackRate) || playbackRate < 0.25 || playbackRate > 3) return null;
  return {
    position: candidate.position,
    volume: candidate.volume,
    muted: candidate.muted,
    loop: candidate.loop,
    playbackRate,
    updatedAt: candidate.updatedAt,
  };
}

export function parseMediaPlaybackStore(raw: string | null): PlaybackStore {
  if (raw === null) return { version: 1, entries: {} };
  if (raw.length > MAX_STORAGE_CHARS) return { version: 1, entries: {} };
  try {
    const parsed = JSON.parse(raw) as { version?: unknown; entries?: unknown };
    if (parsed.version !== 1 || typeof parsed.entries !== 'object' || parsed.entries === null) {
      return { version: 1, entries: {} };
    }
    const validEntries: [string, MediaPlaybackProgress][] = [];
    for (const [key, value] of Object.entries(parsed.entries)) {
      const progress = parseProgress(value);
      if (key.length <= 512 && progress !== null) validEntries.push([key, progress]);
    }
    validEntries.sort((left, right) => right[1].updatedAt - left[1].updatedAt);
    return { version: 1, entries: Object.fromEntries(validEntries.slice(0, MAX_ENTRIES)) };
  } catch {
    return { version: 1, entries: {} };
  }
}

export function updateMediaPlaybackStore(
  raw: string | null,
  key: string,
  progress: MediaPlaybackProgress,
): string {
  const store = parseMediaPlaybackStore(raw);
  store.entries[key] = progress;
  const newest = Object.entries(store.entries)
    .sort((left, right) => right[1].updatedAt - left[1].updatedAt)
    .slice(0, MAX_ENTRIES);
  return JSON.stringify({ version: 1, entries: Object.fromEntries(newest) });
}
