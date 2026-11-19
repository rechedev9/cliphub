import type { EditConfig, RenderMode, VideoStatus } from './types.ts';
import { DEFAULT_EDIT_CONFIG, DEFAULT_VARIANT, FULL_DEMO_REEL_SUFFIX, type ReelIntent } from './reel-store.ts';
import { editConfigsEqual } from './edit-request.ts';
import { isLandscapeHighlights, isLandscapeRecap } from '../reel-brief.ts';

export type ReelIdentityInput = {
  matchId: string;
  playIds: string[];
  variant?: string;
  songId?: string;
  musicVolume?: number;
  gameVolume?: number;
  editConfig?: EditConfig;
};

/** Auto-Shorts reels carry their own prefix, so a batch never replaces a compiled reel of the same plays. */
const AUTO_SHORTS_REEL_PREFIX = 'auto';
/** A 16:9 summary of chosen plays has its own slot, so Crear horizontal never replaces the Short of the same plays. */
const LANDSCAPE_REEL_PREFIX = 'landscape';

/** The plays part of a reel id, shared with the reels recovered from disk. */
export function reelPlaysSuffix(playIds: readonly string[], edit?: Pick<EditConfig, 'separateShorts' | 'format' | 'matchRecap'>): string {
  const plays = playIds.join('_');
  if (edit?.separateShorts) return `${AUTO_SHORTS_REEL_PREFIX}__${plays}`;
  return edit !== undefined && isLandscapeHighlights(edit) ? `${LANDSCAPE_REEL_PREFIX}__${plays}` : plays;
}

/** Mis vídeos title of an Auto-Shorts batch: one Short per play. */
export function autoShortsTitle(shorts: number): string {
  return `Auto-Shorts: ${shorts} ${shorts === 1 ? 'Short' : 'Shorts'}`;
}

export function reelIdentity(input: ReelIdentityInput): string {
  if (input.editConfig && isLandscapeRecap(input.editConfig)) {
    return `${input.matchId}__${FULL_DEMO_REEL_SUFFIX}`;
  }
  return `${input.matchId}__${reelPlaysSuffix(input.playIds, input.editConfig)}`;
}

export function reelContractMatches(
  existing: Pick<ReelIntent, 'variant' | 'songId' | 'musicVolume' | 'gameVolume' | 'editConfig' | 'mode'>,
  input: ReelIdentityInput & { mode: RenderMode },
): boolean {
  const variant = input.variant ?? DEFAULT_VARIANT;
  const edit = input.editConfig ?? DEFAULT_EDIT_CONFIG;
  return (
    (existing.variant ?? DEFAULT_VARIANT) === variant &&
    (existing.songId ?? undefined) === (input.songId ?? undefined) &&
    existing.musicVolume === (input.songId ? input.musicVolume : undefined) &&
    existing.gameVolume === (input.songId ? input.gameVolume : undefined) &&
    existing.mode === input.mode &&
    editConfigsEqual(existing.editConfig, edit)
  );
}

const IN_FLIGHT: ReadonlySet<VideoStatus> = new Set(['queued', 'recording', 'composing']);

/** Full Demo identity is one slot per job. A source change must not replace an in-flight capture. */
export function shouldReuseReelIntent(
  existing: { status: VideoStatus },
  existingIntent: Pick<ReelIntent, 'variant' | 'songId' | 'musicVolume' | 'gameVolume' | 'editConfig' | 'mode'>,
  input: ReelIdentityInput & { mode: RenderMode },
): boolean {
  if (existing.status === 'failed') return false;
  if (reelContractMatches(existingIntent, input)) return true;
  if (!IN_FLIGHT.has(existing.status)) return false;
  const incoming = input.editConfig ?? DEFAULT_EDIT_CONFIG;
  if (incoming.fullDemo || existingIntent.editConfig.fullDemo) return false;
  return isLandscapeRecap(existingIntent.editConfig) && isLandscapeRecap(incoming);
}

export function fullDemoIntentConflict(
  existing: { status: VideoStatus }, existingIntent: Parameters<typeof reelContractMatches>[0], input: ReelIdentityInput & { mode: RenderMode },
): boolean {
  return IN_FLIGHT.has(existing.status) && !!(existingIntent.editConfig.fullDemo || input.editConfig?.fullDemo) && !reelContractMatches(existingIntent, input);
}
