import type { EditConfig, Play } from '../api/types.ts';
import { coerceEditConfig, DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import { GAME_VOLUME_DEFAULT_PERCENT, MUSIC_VOLUME_DEFAULT_PERCENT, MUSIC_VOLUME_MIN_PERCENT } from '../api/reel-music.ts';
import { constrainEditConfig } from '../reel-brief.ts';
import { autoPickBestPlays } from './short-selection.ts';

export const SHORT_DRAFT_VERSION = 1;
export type ShortSettings = {
  selectedIds: string[];
  variant: string | null;
  songId: string | null;
  songTitle: string | null;
  musicDecided: boolean;
  musicVolume: number;
  gameVolume: number;
  editConfig: EditConfig;
};
type ShortDraft = ShortSettings & { version: typeof SHORT_DRAFT_VERSION; savedAt: number };

export function shortDraftKey(matchId: string): string {
  return `cliphub.short-draft.v1.${matchId}`;
}

export function defaultShortSettings(plays: Play[]): ShortSettings {
  return {
    selectedIds: [...autoPickBestPlays(plays)], variant: null, songId: null, songTitle: null, musicDecided: false,
    musicVolume: MUSIC_VOLUME_DEFAULT_PERCENT, gameVolume: GAME_VOLUME_DEFAULT_PERCENT,
    editConfig: constrainEditConfig({ ...DEFAULT_EDIT_CONFIG, format: 'short-9x16' }),
  };
}

function nonemptyString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() !== '' ? value : null;
}

function volume(value: unknown, min: number, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= min && value <= 100 ? value : fallback;
}

/** Storage is untyped. Keep valid decisions even when none of the selected plays still exist. */
export function parseShortDraft(raw: unknown, validPlayIds: ReadonlySet<string>): ShortSettings | null {
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) return null;
  const draft = raw as Record<string, unknown>;
  if (draft.version !== SHORT_DRAFT_VERSION || !Array.isArray(draft.selectedIds)) return null;
  const selectedIds = [...new Set(draft.selectedIds.filter((id): id is string => typeof id === 'string' && validPlayIds.has(id)))];
  const songId = nonemptyString(draft.songId);
  const songTitle = nonemptyString(draft.songTitle);
  const completeSong = songId !== null && songTitle !== null;
  const musicDecided = draft.musicDecided === true && (completeSong || (songId === null && songTitle === null));
  // Full Demo snapshots have their own persistence and must never enter a Short request.
  const rawEdit = typeof draft.editConfig === 'object' && draft.editConfig !== null && !Array.isArray(draft.editConfig)
    ? { ...draft.editConfig } as Record<string, unknown> : {};
  delete rawEdit.fullDemo;
  const editConfig = constrainEditConfig(coerceEditConfig({ ...rawEdit, format: 'short-9x16' }));
  return {
    selectedIds, variant: nonemptyString(draft.variant),
    songId: musicDecided && completeSong ? songId : null,
    songTitle: musicDecided && completeSong ? songTitle : null,
    musicDecided,
    musicVolume: volume(draft.musicVolume, MUSIC_VOLUME_MIN_PERCENT, MUSIC_VOLUME_DEFAULT_PERCENT),
    gameVolume: volume(draft.gameVolume, 0, GAME_VOLUME_DEFAULT_PERCENT),
    editConfig,
  };
}

export function loadShortDraft(matchId: string, validPlayIds: ReadonlySet<string>): ShortSettings | null {
  try {
    const raw = window.sessionStorage.getItem(shortDraftKey(matchId));
    return raw === null ? null : parseShortDraft(JSON.parse(raw), validPlayIds);
  } catch {
    return null;
  }
}

export function saveShortDraft(matchId: string, settings: ShortSettings): void {
  try {
    const draft: ShortDraft = { ...settings, version: SHORT_DRAFT_VERSION, savedAt: Date.now() };
    window.sessionStorage.setItem(shortDraftKey(matchId), JSON.stringify(draft));
  } catch {
    // Editing remains available when storage is full or disabled.
  }
}

export function clearShortDraft(matchId: string): void {
  try {
    window.sessionStorage.removeItem(shortDraftKey(matchId));
  } catch {
    // Storage is optional; the in-memory reset still applies.
  }
}
