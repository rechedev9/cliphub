import type { EditConfig, RenderMode, VideoStatus } from './types.ts';
import { DEFAULT_EDIT_CONFIG, DEFAULT_VARIANT, FULL_DEMO_REEL_SUFFIX, type ReelIntent } from './reel-store.ts';
import { editConfigsEqual } from './edit-request.ts';
import { isLandscapeRecap } from '../reel-brief.ts';

export type ReelIdentityInput = {
  matchId: string;
  playIds: string[];
  variant?: string;
  songId?: string;
  musicVolume?: number;
  gameVolume?: number;
  editConfig?: EditConfig;
};

export function reelIdentity(input: ReelIdentityInput): string {
  if (input.editConfig && isLandscapeRecap(input.editConfig)) {
    return `${input.matchId}__${FULL_DEMO_REEL_SUFFIX}`;
  }
  return `${input.matchId}__${input.playIds.join('_')}`;
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
  return isLandscapeRecap(existingIntent.editConfig) && isLandscapeRecap(incoming);
}
