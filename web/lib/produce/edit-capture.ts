import { DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import { GAME_VOLUME_DEFAULT_PERCENT, MUSIC_VOLUME_DEFAULT_PERCENT } from '../api/reel-music.ts';
import { parseEffectiveEditConfig, parseEffectiveRenderMusic } from '../api/render-hydration.ts';
import type { EditConfig, Play } from '../api/types.ts';
import { constrainEditConfig } from '../reel-brief.ts';
import { formatBytes } from '../storage.ts';
import type { ShortSettings } from './short-draft.ts';
import type { VideoVersion } from '../api/video-edit.ts';

/** The server's answer to "does this plan open CS2?" (`POST /full-demo/capture-check`). */
export type CaptureCheck = { needed: boolean; missingRounds: readonly number[]; space?: SpaceCheck | null };
export type SpaceCheck = { freeBytes: number; neededBytes: number; short: boolean };

export const EDIT_RENDER_CTA = 'Volver a montar';
export const EDIT_RECORD_CTA = 'Volver a grabar y montar';

/** The create button while editing a finished video; unknown capture keeps the usual label. */
export function editCtaLabel({ capture, editing, fallback }: { capture: CaptureCheck | null; editing: boolean; fallback: string }): string {
  if (!editing || capture === null) return fallback;
  return capture.needed ? EDIT_RECORD_CTA : EDIT_RENDER_CTA;
}

function roundList(rounds: readonly number[]): string {
  const sorted = [...rounds].sort((a, b) => a - b).map(String);
  return sorted.length <= 1 ? sorted.join('') : `${sorted.slice(0, -1).join(', ')} y ${sorted[sorted.length - 1]}`;
}

/** What an edit of a Full Demo will record, next to its button. */
export function captureNote(capture: CaptureCheck, roundCount: number): string {
  if (!capture.needed) return 'Las capturas de todas las rondas están guardadas: solo se vuelve a montar.';
  if (capture.missingRounds.length >= roundCount) return 'Las capturas de este vídeo ya no están guardadas: se grabarán todas sus rondas.';
  const one = capture.missingRounds.length === 1;
  return `Se grabará${one ? '' : 'n'} otra vez ${one ? 'la ronda' : 'las rondas'} ${roundList(capture.missingRounds)}; las demás ya están capturadas.`;
}

/** A warning, never a block: the estimate comes from typical bitrates. */
export function spaceWarning(space: SpaceCheck | null | undefined): string | null {
  if (!space?.short) return null;
  return `Puede que no haya espacio suficiente: hacen falta unos ${formatBytes(space.neededBytes)} y quedan ${formatBytes(space.freeBytes)} libres.`;
}

/** Seconds of play in the chosen source rounds (every round when none is chosen), from the recap's clock. */
export function recapSeconds(recap: readonly Pick<Play, 'round' | 'startSeconds' | 'endSeconds'>[], rounds: readonly number[]): number {
  const chosen = new Set(rounds);
  let total = 0;
  for (const play of recap) {
    if (chosen.size > 0 && !chosen.has(play.round)) continue;
    if (play.startSeconds !== undefined && play.endSeconds !== undefined) total += Math.max(0, play.endSeconds - play.startSeconds);
  }
  return total;
}

/** What a Short's capture depends on: its plays, the preset's HUD and the edit options CS2 records with. */
export type ShortCaptureChoice = { playIds: readonly string[]; hudMode: string | undefined; edit: EditConfig };

function captureHud(choice: ShortCaptureChoice): string {
  return choice.edit.nativeHud ? 'gameplay' : choice.hudMode ?? '';
}

function portraitFraming(edit: EditConfig): boolean {
  return edit.format === 'short-9x16' && edit.centeredKillfeed !== true;
}

/** Mirrors the record worker's reuse (`sameCapture` in internal/httpapi); the worker still decides. */
export function shortEditNeedsCapture(recorded: ShortCaptureChoice, next: ShortCaptureChoice): boolean {
  if (captureHud(recorded) !== captureHud(next) || portraitFraming(recorded.edit) !== portraitFraming(next.edit)) return true;
  if ((recorded.edit.deathReel === true) !== (next.edit.deathReel === true)) return true;
  if (next.edit.victimReplay === true && recorded.edit.victimReplay !== true) return true;
  const have = new Set(recorded.playIds);
  return next.playIds.some((id) => !have.has(id));
}

/** A Short or resumen version reopened by Editar: its producer settings and when it was rendered. */
export type ShortEditSeed = { settings: ShortSettings; date: string };

/** A version as the versions API returns it, enough to reopen a Short in its producer. */
export type ShortVersionSeed = { variant: string; segmentIds: readonly string[]; edit: unknown; music?: unknown };

/** The producer settings of a Short version; null when its edit is not a Short's. */
export function shortSettingsFromVersion(version: ShortVersionSeed): ShortSettings | null {
  const parsed = parseEffectiveEditConfig(version.edit);
  if (parsed === undefined || parsed.fullDemo) return null;
  const music = parseEffectiveRenderMusic(version.music);
  const withMusic = music?.mode === 'music' ? music : null;
  return {
    selectedIds: [...version.segmentIds],
    variant: version.variant,
    songId: withMusic?.songId ?? null,
    songTitle: null,
    musicDecided: true,
    musicVolume: withMusic ? Math.round(withMusic.musicVolume * 100) : MUSIC_VOLUME_DEFAULT_PERCENT,
    gameVolume: withMusic?.gameVolume !== undefined ? Math.round(withMusic.gameVolume * 100) : GAME_VOLUME_DEFAULT_PERCENT,
    editConfig: constrainEditConfig({ ...DEFAULT_EDIT_CONFIG, ...parsed }),
  };
}

const VERSION_DATE = new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

export function versionDateLabel(createdAt: string): string {
  const date = new Date(createdAt);
  return Number.isNaN(date.getTime()) ? '-' : VERSION_DATE.format(date);
}

/** The options a version was rendered with, one line in Versiones. */
export function versionOptionsLabel(version: Pick<VideoVersion, 'planId' | 'rounds' | 'segmentIds' | 'music'>): string {
  if (version.planId !== null) {
    if (version.rounds.length === 0) return 'Demo completa';
    return `${version.rounds.length === 1 ? 'Ronda' : 'Rondas'} ${roundList(version.rounds)}`;
  }
  const plays = version.segmentIds.length;
  const music = parseEffectiveRenderMusic(version.music)?.mode === 'music' ? 'con música' : 'sin música';
  return plays > 0 ? `${plays} ${plays === 1 ? 'jugada' : 'jugadas'}, ${music}` : music;
}

/** The version Editar opens: the one asked for, else the current one, else the newest. */
export function pickVersion(versions: readonly VideoVersion[], revision: string | null): VideoVersion | null {
  return versions.find((version) => version.revision === revision) ?? versions.find((version) => version.current) ?? versions[0] ?? null;
}
