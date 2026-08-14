import type { ReelIntent } from './reel-store';

/** Music the user wants on the next render. An empty songId means no music. */
export type MusicChoice = {
  songId?: string;
  /**
   * Track gain in (0,1]. Absent means the default full volume, which renders
   * byte-identically to a reel that never set a volume.
   */
  musicVolume?: number;
};

export const MUSIC_VOLUME_MIN_PERCENT = 5;
export const MUSIC_VOLUME_MAX_PERCENT = 100;
export const MUSIC_VOLUME_STEP_PERCENT = 5;
export const MUSIC_VOLUME_DEFAULT_PERCENT = 100;

/** True when both choices describe the same mix (including "no music"). */
export function musicChoicesEqual(left: MusicChoice, right: MusicChoice): boolean {
  const leftKey = left.songId ?? '';
  const rightKey = right.songId ?? '';
  if (leftKey !== rightKey) return false;
  if (leftKey === '') return true;
  return (left.musicVolume ?? 1) === (right.musicVolume ?? 1);
}

/** UI percent → render request volume. Full volume stays unset (legacy default). */
export function musicVolumePercentToRequest(percent: number): number | undefined {
  return percent < MUSIC_VOLUME_MAX_PERCENT ? percent / 100 : undefined;
}

/** Stored request volume → UI percent. Absent volume is the full-volume default. */
export function musicVolumeRequestToPercent(volume: number | undefined): number {
  return volume === undefined ? MUSIC_VOLUME_DEFAULT_PERCENT : Math.round(volume * 100);
}

/**
 * Writes a music choice onto a reel intent and drops the approved cover so the
 * Library thumbnail gate reopens on the new revision.
 */
export function applyMusicChoice(intent: ReelIntent, choice: MusicChoice): void {
  if (!choice.songId) {
    intent.mode = 'clean';
    delete intent.songId;
    delete intent.musicVolume;
  } else {
    intent.mode = 'music';
    intent.songId = choice.songId;
    if (choice.musicVolume !== undefined && choice.musicVolume < 1) {
      intent.musicVolume = choice.musicVolume;
    } else {
      delete intent.musicVolume;
    }
  }
  delete intent.selectedCoverName;
}

/**
 * Swaps the ` - {variant}` / ` - {variant} + Music` suffix createVideo writes.
 * A title that does not carry either suffix is left unchanged so a hand-edited
 * or legacy name is not rewritten.
 */
export function titleWithMusicSuffix(title: string, variantLabel: string, hasMusic: boolean): string {
  const musicSuffix = `${variantLabel} + Music`;
  const cleanSuffix = variantLabel;
  if (title.endsWith(` - ${musicSuffix}`)) {
    return hasMusic ? title : `${title.slice(0, -(musicSuffix.length + 3))} - ${cleanSuffix}`;
  }
  if (title.endsWith(` - ${cleanSuffix}`)) {
    return hasMusic ? `${title.slice(0, -(cleanSuffix.length + 3))} - ${musicSuffix}` : title;
  }
  return title;
}
