import { fixedFullDemoFreeze, type FullDemoOptions } from '../full-demo-plan.ts';

/** Reset technical tuning while retaining chosen media, content and fallback decisions. */
export function recommendedFullDemoSettings(current: FullDemoOptions, defaults: FullDemoOptions): FullDemoOptions {
  return fixedFullDemoFreeze({
    ...current,
    capture: { ...current.capture, hud_profile: defaults.capture.hud_profile },
    editorial: { ...defaults.editorial, manual_ranges: [] },
    audio: {
      ...current.audio,
      game: { ...defaults.audio.game },
      voice: { ...current.audio.voice, gain: defaults.audio.voice.gain, normalization: defaults.audio.voice.normalization },
      music: { ...current.audio.music, bed_gain_db: defaults.audio.music.bed_gain_db, ducking: { ...defaults.audio.music.ducking } },
    },
  });
}
