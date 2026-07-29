import type { ReelIntent } from './reel-store';
import type { EditConfig, Video } from './types';

export type EffectiveRenderMusic =
  | { mode: 'clean' }
  | { mode: 'music'; songId: string; musicVolume: number };

/** Parses the Go edit document's snake-case wire representation. */
export function parseEffectiveEditConfig(value: unknown): EditConfig | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const edit = value as Record<string, unknown>;
  const formats = new Set<EditConfig['format']>(['short-9x16', 'landscape-16x9']);
  const killEffects = new Set<EditConfig['killEffect']>(['clean', 'punch-in', 'velocity', 'freeze-flash']);
  const transitions = new Set<EditConfig['transition']>(['cut', 'flash', 'whip', 'dip']);
  const covers = new Set<EditConfig['coverStrategy']>(['generated-gameplay', 'no-cover']);
  if (
    typeof edit.format !== 'string' ||
    !formats.has(edit.format as EditConfig['format']) ||
    typeof edit.kill_effect !== 'string' ||
    !killEffects.has(edit.kill_effect as EditConfig['killEffect']) ||
    typeof edit.transition !== 'string' ||
    !transitions.has(edit.transition as EditConfig['transition']) ||
    typeof edit.cover_strategy !== 'string' ||
    !covers.has(edit.cover_strategy as EditConfig['coverStrategy']) ||
    typeof edit.intro !== 'boolean' ||
    typeof edit.outro !== 'boolean' ||
    typeof edit.hook_text !== 'boolean' ||
    typeof edit.kill_counter !== 'boolean'
  ) {
    return undefined;
  }
  const parsed: EditConfig = {
    format: edit.format as EditConfig['format'],
    killEffect: edit.kill_effect as EditConfig['killEffect'],
    transition: edit.transition as EditConfig['transition'],
    coverStrategy: edit.cover_strategy as EditConfig['coverStrategy'],
    intro: edit.intro,
    outro: edit.outro,
    hookText: edit.hook_text,
    killCounter: edit.kill_counter,
  };
  if (typeof edit.intro_text === 'string') parsed.introText = edit.intro_text;
  if (typeof edit.outro_text === 'string') parsed.outroText = edit.outro_text;
  return parsed;
}

/** Parses the durable music snapshot returned by the render-variant API. */
export function parseEffectiveRenderMusic(value: unknown): EffectiveRenderMusic | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const music = value as Record<string, unknown>;
  if (typeof music.key !== 'string' || typeof music.volume !== 'number' || !Number.isFinite(music.volume)) {
    return undefined;
  }
  if (music.key === '' && music.volume === 0) return { mode: 'clean' };
  if (music.key === '' || music.volume <= 0 || music.volume > 1) return undefined;
  return { mode: 'music', songId: music.key, musicVolume: music.volume };
}

/** Applies server-confirmed music to the durable local intent. */
export function applyEffectiveRenderMusic(intent: ReelIntent, music: EffectiveRenderMusic): boolean {
  if (music.mode === 'clean') {
    const changed =
      intent.mode !== 'clean' ||
      intent.songId !== undefined ||
      intent.musicVolume !== undefined;
    intent.mode = 'clean';
    delete intent.songId;
    delete intent.musicVolume;
    return changed;
  }

  const changed =
    intent.mode !== 'music' ||
    intent.songId !== music.songId ||
    intent.musicVolume !== music.musicVolume;
  intent.mode = 'music';
  intent.songId = music.songId;
  intent.musicVolume = music.musicVolume;
  return changed;
}

/** Rehydrates live card fields from the latest durable local intent. */
export function hydrateVideoFromIntent(video: Video, intent: ReelIntent): Video {
  return {
    ...video,
    mode: intent.mode,
    songId: intent.songId,
    musicVolume: intent.musicVolume,
    editConfig: intent.editConfig,
  };
}

/** Removes URLs that belong to an older render revision. */
export function clearVideoArtifactUrls(video: Video): Video {
  const next = { ...video };
  delete next.downloadUrl;
  delete next.thumbnailUrl;
  return next;
}
