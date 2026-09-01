import { DEMO_SOURCE, isDemoSource, SERVICE_UNAVAILABLE_CODE, type DemoSource, type EditConfig, type Preset } from './api/types.ts';
import { NATIVE_HUD_LABEL, PRESET_DESCRIPTION_ES } from './preset-copy.ts';

export const FULL_DEMO_HREF = '/full-demo' as const;

export const FULL_DEMO_ROUNDS_PENDING = 'Generando el plan de rondas…';

export const FULL_DEMO_RECAP_ERROR =
  'No se pudo cargar el plan de rondas de esta partida. Recarga o elige otra demo.';

export const FULL_DEMO_FORGE_HINT_EMPTY = 'Espera el plan de rondas para empezar.';

export const FULL_DEMO_FORGE_HINT_ERROR = 'No se pudo cargar el plan de rondas.';

/** Collapsed Shorts-negative row for the Full Demo brief (wire stays FULL_DEMO_EDIT). */
export const FULL_DEMO_SHORTS_EXTRAS = {
  label: 'Extras de Short',
  value: 'ninguno (sin efectos, transiciones, música, KeyDrop ni portada de reel)',
} as const;

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

export const FULL_DEMO_SOURCE_OPTIONS = [
  { value: DEMO_SOURCE.premier, label: 'Premier' },
  { value: DEMO_SOURCE.professional, label: 'Profesional (HLTV)' },
  { value: DEMO_SOURCE.faceit, label: 'FACEIT' },
] as const;

export function fullDemoSourceLabel(source: DemoSource | ''): string {
  return FULL_DEMO_SOURCE_OPTIONS.find((option) => option.value === source)?.label ?? '';
}

export function fullDemoEdit(source: DemoSource): EditConfig {
  return { ...FULL_DEMO_EDIT, demoSource: source };
}

export function canStartFullDemoCapture({
  roundCount,
  briefApproved,
  demoSource,
  creating,
}: {
  roundCount: number;
  briefApproved: boolean;
  demoSource: DemoSource | '';
  creating: boolean;
}): boolean {
  return roundCount > 0 && briefApproved && isDemoSource(demoSource) && !creating;
}

export const FULL_DEMO_PRESET: Preset = {
  name: FULL_DEMO_VARIANT,
  label: 'POV nativo',
  description: PRESET_DESCRIPTION_ES['gameplay-pov-60'],
  hudMode: 'gameplay',
  width: 1920,
  height: 1080,
};

export const FULL_DEMO_CONTRACT = [
  { label: 'Formato', value: 'Horizontal 16:9 · 1920×1080' },
  { label: 'Entrega', value: 'Rondas en vivo · sin freeze' },
  { label: 'Comms', value: 'Comms del equipo del POV · 85%' },
  { label: 'HUD', value: NATIVE_HUD_LABEL },
  { label: 'Efectos', value: 'Sin punch-in ni transiciones de Short' },
  { label: 'Mix', value: 'Comms + juego · sin música' },
] as const;

/** On-screen brief for /full-demo/[id]: contract facts + one Shorts-extras row. */
export function fullDemoBriefItems(): ReadonlyArray<{ label: string; value: string }> {
  return [...FULL_DEMO_CONTRACT, FULL_DEMO_SHORTS_EXTRAS];
}

/** null = getMatch returned empty (404 / not on disk); offline = 503; error = any other throw. */
export type FullDemoLoadFailure = 'offline' | 'error' | null;

export const FULL_DEMO_EMPTY = {
  offline: {
    title: 'Servicio local sin conexión',
    description: 'Arranca ClipHub y vuelve a abrir esta partida.',
  },
  error: {
    title: 'No se pudo cargar la demo',
    description: 'Hubo un error al leer esta demo. Vuelve a intentarlo o elige otra.',
  },
  missing: {
    title: 'Demo no encontrada',
    description: 'Esta demo ya no está en el disco local.',
  },
} as const;

export function classifyFullDemoLoadFailure(err: unknown): Exclude<FullDemoLoadFailure, null> {
  if (typeof err !== 'object' || err === null || !('code' in err)) return 'error';
  return err.code === SERVICE_UNAVAILABLE_CODE ? 'offline' : 'error';
}

export function fullDemoEmptyState(failure: FullDemoLoadFailure): (typeof FULL_DEMO_EMPTY)[keyof typeof FULL_DEMO_EMPTY] {
  if (failure === 'offline') return FULL_DEMO_EMPTY.offline;
  if (failure === 'error') return FULL_DEMO_EMPTY.error;
  return FULL_DEMO_EMPTY.missing;
}
