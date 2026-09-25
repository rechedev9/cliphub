export const FORGE_HINT_EMPTY_PLAYS = 'Elige al menos una jugada para empezar.';
export const FORGE_HINT_CHOOSE_PRESET = 'Elige un preset para continuar.';
export const FORGE_HINT_DECIDE_MUSIC = 'Decide la música: un tema o sin música.';

/** Sticky-bar next step for the Shorts producer. */
export function forgeHint(selectionLabel: string | null, presetLabel: string | null): string {
  if (selectionLabel == null) return FORGE_HINT_EMPTY_PLAYS;
  if (presetLabel == null) return FORGE_HINT_CHOOSE_PRESET;
  return FORGE_HINT_DECIDE_MUSIC;
}
