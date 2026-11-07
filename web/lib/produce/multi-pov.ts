import type { EditConfig, Play } from '../api/types.ts';
import { FULL_DEMO_VOICE_VOLUME } from '../full-demo.ts';

/** Two players' plays need this much demo time between them: one capture films one POV at a time (killplan.POVClash). */
export const POV_SWITCH_SECONDS = 0.5;

type TimedPlay = Pick<Play, 'player' | 'startSeconds' | 'endSeconds'>;

/** True when two plays film different players too close together to share one capture. */
export function playsClash(a: TimedPlay, b: TimedPlay): boolean {
  if (!a.player || !b.player || a.player.steamId === b.player.steamId) return false;
  const { startSeconds: aStart, endSeconds: aEnd } = a;
  const { startSeconds: bStart, endSeconds: bEnd } = b;
  if (aStart === undefined || aEnd === undefined || bStart === undefined || bEnd === undefined) return false;
  return aStart < bEnd + POV_SWITCH_SECONDS && bStart < aEnd + POV_SWITCH_SECONDS;
}

/** The first chosen play that clashes with `play`, if any. */
export function clashingPlay<T extends TimedPlay>(play: TimedPlay, chosen: Iterable<T>): T | undefined {
  for (const other of chosen) if (other !== play && playsClash(play, other)) return other;
  return undefined;
}

/** `kept` plus every play of `candidates`, in order, that clashes with nothing already in. */
export function addWithoutClashes<T extends TimedPlay & Pick<Play, 'id'>>(kept: readonly T[], candidates: readonly T[]): T[] {
  const out = [...kept];
  for (const play of candidates) {
    if (out.some((other) => other.id === play.id) || clashingPlay(play, out)) continue;
    out.push(play);
  }
  return out;
}

/** "ropz", "ropz y b1t" or "ropz, b1t y 3 más", as the editor names a multi-POV Short. */
export function povPlayersLabel(names: readonly string[]): string {
  if (names.length > 2) return `${names[0]}, ${names[1]} y ${names.length - 2} más`;
  return names.join(' y ');
}

/** A 16:9 best-moments video runs longer than a Short: about three minutes. */
export const BEST_MOMENTS_TARGET_SECONDS = 180;

/** The 16:9 best-moments look: the game's own HUD and team comms, no recap overlays or 9:16 garnish. */
export function bestMomentsEdit(edit: EditConfig): EditConfig {
  const next: EditConfig = { ...edit, format: 'landscape-16x9', matchRecap: false, nativeHud: true, voiceComms: true, voiceVolume: edit.voiceVolume ?? FULL_DEMO_VOICE_VOLUME, deathReel: false, killCounter: false };
  delete next.centeredKillfeed;
  delete next.demoSource;
  delete next.overlayTheme;
  return next;
}

/** One multi-POV job films 2 to 5 players of one team (job.MinMultiPOVPlayers, job.MaxMultiPOVPlayers). */
export const MIN_POV_PLAYERS = 2;
export const MAX_POV_PLAYERS = 5;

export type PovCandidate = { steamId: string; team: string };

/** Adds or removes a player; one of the other team starts a new pick, and a full pick stays as it is. */
export function togglePovPick<T extends PovCandidate>(picked: readonly T[], player: T): T[] {
  if (picked.some((p) => p.steamId === player.steamId)) return picked.filter((p) => p.steamId !== player.steamId);
  if (picked.length > 0 && picked[0].team !== player.team) return [player];
  if (picked.length >= MAX_POV_PLAYERS) return [...picked];
  return [...picked, player];
}
