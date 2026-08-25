import type { EditConfig, Preset } from './api/types';
import { NATIVE_HUD_LABEL, PRESET_DESCRIPTION_ES } from './preset-copy.ts';

export const FULL_DEMO_HREF = '/full-demo' as const;

export const FULL_DEMO_ROUNDS_PENDING =
  'Esta demo no tiene plan de rondas todavía. Espera a que termine el parseo o elige otra.';

export const FULL_DEMO_RECAP_ERROR =
  'No se pudo cargar el plan de rondas de esta partida. Recarga o elige otra demo.';

export const FULL_DEMO_FORGE_HINT_EMPTY = 'Espera el plan de rondas para empezar.';

export const FULL_DEMO_FORGE_HINT_ERROR = 'No se pudo cargar el plan de rondas.';

export const FULL_DEMO_VARIANT = 'gameplay-pov-60';

/** Locked mix: team comms in front of full game audio. No music bed. */
export const FULL_DEMO_VOICE_VOLUME = 0.85;

export const FULL_DEMO_EDIT: EditConfig = {
  format: 'landscape-16x9',
  killEffect: 'clean',
  transition: 'cut',
  intro: false,
  outro: false,
  hookText: false,
  killCounter: false,
  matchRecap: true,
  voiceComms: true,
  voiceVolume: FULL_DEMO_VOICE_VOLUME,
  nativeHud: true,
  coverStrategy: 'generated-gameplay',
};

export const FULL_DEMO_PRESET: Preset = {
  name: FULL_DEMO_VARIANT,
  label: 'POV nativo',
  description: PRESET_DESCRIPTION_ES['gameplay-pov-60'],
  hudMode: 'gameplay',
};

/** Prefer the live registry card; keep a clickable fallback if the orchestrator is stale. */
export function resolveFullDemoPreset(presets: Preset[]): Preset {
  return presets.find((preset) => preset.name === FULL_DEMO_VARIANT) ?? FULL_DEMO_PRESET;
}

export const FULL_DEMO_CONTRACT = [
  { label: 'Formato', value: 'Horizontal 16:9 · 1920×1080' },
  { label: 'Entrega', value: 'Rondas en vivo · sin freeze' },
  { label: 'Comms', value: 'Comms del equipo del POV · 85%' },
  { label: 'HUD', value: NATIVE_HUD_LABEL },
  { label: 'Efectos', value: 'Sin punch-in ni transiciones de Short' },
  { label: 'Mix', value: 'Comms + juego · sin música' },
] as const;
