import type { EditConfig } from './api/types';

export const FULL_DEMO_HREF = '/full-demo' as const;

export const FULL_DEMO_VARIANT = 'viral-60-clean';

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
  voiceVolume: 0.85,
  nativeHud: true,
  coverStrategy: 'generated-gameplay',
};

export const FULL_DEMO_CONTRACT = [
  { label: 'Formato', value: 'Horizontal 16:9 · 1920×1080' },
  { label: 'Entrega', value: 'Rondas completas' },
  { label: 'Comms', value: 'Equipo del POV · 85%' },
  { label: 'HUD', value: 'Nativo CS2 (radar, vida, killfeed)' },
  { label: 'Efectos', value: 'Sin punch-in ni transiciones de Short' },
] as const;
