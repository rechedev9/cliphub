import type { ReelIntent } from './reel-store';
import {
  isAffiliateStyle,
  isDemoSource,
  isKeyDropStyle,
  isOverlayTheme,
  normalizeAffiliateFamily,
  type EditConfig,
  type Video,
} from './types.ts';

export type EffectiveRenderMusic =
  | { mode: 'clean' }
  | { mode: 'music'; songId: string; musicVolume: number; gameVolume?: number };

/** Parses the orchestrator edit wire; accepts both killEffect and kill_effect. */
export function parseEffectiveEditConfig(value: unknown): EditConfig | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const edit = value as Record<string, unknown>;
  const formats = new Set<EditConfig['format']>(['short-9x16', 'landscape-16x9']);
  const killEffects = new Set<EditConfig['killEffect']>(['clean', 'punch-in', 'velocity', 'freeze-flash', 'shake', 'glitch']);
  const transitions = new Set<EditConfig['transition']>(['cut', 'flash', 'whip', 'dip', 'glitch', 'zoom-whip']);
  const covers = new Set<EditConfig['coverStrategy']>(['generated-gameplay', 'no-cover']);
  let killEffectRaw: string | undefined;
  if (typeof edit.killEffect === 'string') {
    killEffectRaw = edit.killEffect;
  } else if (typeof edit.kill_effect === 'string') {
    killEffectRaw = edit.kill_effect;
  }
  if (
    typeof edit.format !== 'string' ||
    !formats.has(edit.format as EditConfig['format']) ||
    killEffectRaw === undefined ||
    !killEffects.has(killEffectRaw as EditConfig['killEffect']) ||
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
    killEffect: killEffectRaw as EditConfig['killEffect'],
    transition: edit.transition as EditConfig['transition'],
    coverStrategy: edit.cover_strategy as EditConfig['coverStrategy'],
    intro: edit.intro,
    outro: edit.outro,
    hookText: edit.hook_text,
    killCounter: edit.kill_counter,
    matchRecap: edit.match_recap === true,
    voiceComms: edit.voice_comms === true,
    nativeHud: edit.native_hud === true,
  };
  if (
    typeof edit.voice_volume === 'number' &&
    Number.isFinite(edit.voice_volume) &&
    edit.voice_volume >= 0 &&
    edit.voice_volume <= 1
  ) {
    parsed.voiceVolume = edit.voice_volume;
  }
  if (typeof edit.intro_text === 'string') parsed.introText = edit.intro_text;
  if (typeof edit.outro_text === 'string') parsed.outroText = edit.outro_text;
  const family =
    typeof edit.keydrop_family === 'string' ? normalizeAffiliateFamily(edit.keydrop_family) : '';
  if (typeof edit.keydrop_style === 'string') {
    if (family && isAffiliateStyle(family, edit.keydrop_style) && isKeyDropStyle(edit.keydrop_style)) {
      parsed.keyDropFamily = family;
      parsed.keyDropStyle = edit.keydrop_style;
    } else if (!family && isKeyDropStyle(edit.keydrop_style)) {
      parsed.keyDropFamily = 'KEYDROP';
      parsed.keyDropStyle = edit.keydrop_style;
    }
  }
  if (typeof edit.keydrop_code === 'string') {
    parsed.keyDropCode = edit.keydrop_code;
  }
  if (typeof edit.keydrop_position_y === 'number' && Number.isFinite(edit.keydrop_position_y)) {
    parsed.keyDropPositionY = edit.keydrop_position_y;
  }
  if (typeof edit.keydrop_start_seconds === 'number' && Number.isFinite(edit.keydrop_start_seconds)) {
    parsed.keyDropStartSeconds = edit.keydrop_start_seconds;
  }
  if (typeof edit.keydrop_end_seconds === 'number' && Number.isFinite(edit.keydrop_end_seconds)) {
    parsed.keyDropEndSeconds = edit.keydrop_end_seconds;
  }
  if (isDemoSource(edit.demo_source)) {
    parsed.demoSource = edit.demo_source;
  }
  if (typeof edit.overlay_theme === 'string' && isOverlayTheme(edit.overlay_theme)) {
    parsed.overlayTheme = edit.overlay_theme;
  }
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
  const parsed: Extract<EffectiveRenderMusic, { mode: 'music' }> = {
    mode: 'music',
    songId: music.key,
    musicVolume: music.volume,
  };
  if (
    typeof music.game_volume === 'number' &&
    Number.isFinite(music.game_volume) &&
    music.game_volume >= 0 &&
    music.game_volume <= 1
  ) {
    parsed.gameVolume = music.game_volume;
  }
  return parsed;
}

/** Applies server-confirmed music to the durable local intent. */
export function applyEffectiveRenderMusic(intent: ReelIntent, music: EffectiveRenderMusic): boolean {
  if (music.mode === 'clean') {
    const changed =
      intent.mode !== 'clean' ||
      intent.songId !== undefined ||
      intent.musicVolume !== undefined ||
      intent.gameVolume !== undefined;
    intent.mode = 'clean';
    delete intent.songId;
    delete intent.musicVolume;
    delete intent.gameVolume;
    return changed;
  }

  const changed =
    intent.mode !== 'music' ||
    intent.songId !== music.songId ||
    intent.musicVolume !== music.musicVolume ||
    intent.gameVolume !== music.gameVolume;
  intent.mode = 'music';
  intent.songId = music.songId;
  intent.musicVolume = music.musicVolume;
  if (music.gameVolume !== undefined) {
    intent.gameVolume = music.gameVolume;
  } else {
    delete intent.gameVolume;
  }
  return changed;
}

/** Rehydrates live card fields from the latest durable local intent. */
export function hydrateVideoFromIntent(video: Video, intent: ReelIntent): Video {
  const next: Video = {
    ...video,
    mode: intent.mode,
    songId: intent.songId,
    musicVolume: intent.musicVolume,
    gameVolume: intent.gameVolume,
    editConfig: intent.editConfig,
  };
  if (intent.selectedCoverName) {
    next.selectedCoverName = intent.selectedCoverName;
  } else {
    delete next.selectedCoverName;
  }
  return next;
}

/** Removes URLs that belong to an older render revision. */
export function clearVideoArtifactUrls(video: Video): Video {
  const next = { ...video };
  delete next.downloadUrl;
  delete next.thumbnailUrl;
  return next;
}
