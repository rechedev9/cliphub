import type { FullDemoTransitionOptions } from './full-demo-plan.ts';

// Missing settings keep historical documents byte-for-byte intact until edited.
export const DEFAULT_FULL_DEMO_TRANSITIONS: Readonly<FullDemoTransitionOptions> = {
  enabled: false, duration_frames: 8, whip: true, direction: 'alternate', whip_strength: .08, blur_pixels: 24,
  zoom: true, zoom_percent: 10, zoom_anchor: 'cut', flash: false, flash_frames: 2, flash_intensity: .08,
  rgb_split: false, rgb_pixels: 2, whoosh: true, whoosh_gain_db: -18, impact: true, impact_gain_db: -20,
  impact_duration_ms: 180, impact_frequency: 55, comms_tail_seconds: .6, game_fade_ms: 40, game_tail_lowpass_hz: 0,
};

export function fullDemoTransitionPreset(): FullDemoTransitionOptions {
  const base = { ...DEFAULT_FULL_DEMO_TRANSITIONS, enabled: true };
  return { ...base, direction: 'follow-motion', flash: true, rgb_split: true };
}

export function fullDemoTransitionSummary(value: FullDemoTransitionOptions | null | undefined): string {
  return value?.enabled ? 'Dinámico' : 'Corte limpio';
}
