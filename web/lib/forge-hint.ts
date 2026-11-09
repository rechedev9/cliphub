import { PRODUCE_READY_HINT } from './produce/copy.ts';

export const FORGE_HINT_EMPTY_PLAYS = 'Elige al menos una jugada para empezar.';
export const FORGE_HINT_CHOOSE_PRESET = 'Elige un estilo para continuar.';
export const FORGE_HINT_LOADING_PRESETS = 'Cargando los estilos…';

/** Sticky-bar next step for the Shorts producer; music is optional, so plays and a style are enough. */
export function forgeHint(selectionLabel: string | null, preset: { label: string | null; loading: boolean }): string {
  if (selectionLabel == null) return FORGE_HINT_EMPTY_PLAYS;
  if (preset.loading) return FORGE_HINT_LOADING_PRESETS;
  if (preset.label == null) return FORGE_HINT_CHOOSE_PRESET;
  return PRODUCE_READY_HINT;
}
