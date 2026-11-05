import type { StorageFile, StorageFileKind } from './api/storage.ts';
import type { Match, Video } from './api/types.ts';
import { prettyMapName } from './format.ts';

export const STORAGE_FILE_KIND_LABEL = {
  demo: 'Demo',
  studio_demo: 'Copia de Studio',
  short: 'Short',
  full_demo: 'Demo completa',
  stream: 'Clip de stream',
  editor: 'Editor',
  cloud: 'Nube',
} as const satisfies Record<StorageFileKind, string>;

/** Splits a Windows or POSIX path into its folder and file name. */
export function splitPath(path: string): { folder: string; name: string } {
  const cut = Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/'));
  if (cut < 0) return { folder: '', name: path };
  return { folder: path.slice(0, cut), name: path.slice(cut + 1) };
}

export type FileGroup = { key: string; label: string; totalBytes: number; files: StorageFile[] };

function groupFiles(files: readonly StorageFile[], keyOf: (file: StorageFile) => string): FileGroup[] {
  const groups = new Map<string, FileGroup>();
  for (const file of files) {
    const key = keyOf(file);
    const group = groups.get(key) ?? { key, label: key, totalBytes: 0, files: [] };
    group.totalBytes += file.sizeBytes;
    group.files.push(file);
    groups.set(key, group);
  }
  return [...groups.values()].sort((a, b) => b.totalBytes - a.totalBytes);
}

/** Demos grouped by folder, the heaviest folder first, so the user sees where they pile up. */
export function groupByFolder(files: readonly StorageFile[]): FileGroup[] {
  return groupFiles(files, (file) => splitPath(file.path).folder);
}

/** Studio videos grouped by kind (Shorts, Demo completa...), the heaviest first. */
export function groupByKind(files: readonly StorageFile[]): FileGroup[] {
  return groupFiles(files, (file) => file.kind).map((group) => ({
    ...group,
    label: STORAGE_FILE_KIND_LABEL[group.files[0].kind],
  }));
}

export function totalBytes(files: readonly StorageFile[]): number {
  return files.reduce((sum, file) => sum + file.sizeBytes, 0);
}

/** "Mirage · s1mple" for a file of a known match, else null. */
export function matchLabel(jobId: string | undefined, matches: ReadonlyMap<string, Pick<Match, 'map' | 'player'>>): string | null {
  const match = jobId === undefined ? undefined : matches.get(jobId);
  if (match === undefined) return null;
  const map = prettyMapName(match.map);
  return match.player ? `${map} · ${match.player}` : map;
}

const DATE = new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', year: 'numeric' });

/** "5 nov 2026", or '' for a missing or bad timestamp. */
export function formatFileDate(iso: string): string {
  const time = Date.parse(iso);
  return Number.isNaN(time) ? '' : DATE.format(time);
}

/** The Mis vídeos entry that plays this render MP4, so deleting the file also drops the card. */
export function libraryVideoFor(videos: readonly Pick<Video, 'id' | 'jobId' | 'downloadUrl'>[], file: StorageFile): string | null {
  if (file.jobId === undefined || (file.kind !== 'short' && file.kind !== 'full_demo')) return null;
  const suffix = `/videos/${splitPath(file.path).name}`;
  const video = videos.find((entry) => entry.jobId === file.jobId && entry.downloadUrl?.split('?')[0]?.endsWith(suffix));
  return video?.id ?? null;
}
