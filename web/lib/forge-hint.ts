export const FORGE_HINT_EMPTY_PLAYS = 'Elige al menos una jugada para empezar.';
export const FORGE_HINT_CHOOSE_PRESET = 'Elige un preset para continuar.';
export const FORGE_HINT_DECIDE_MUSIC = 'Decide la música: un tema o sin música.';

/** Sticky-bar next step. Full Demo passes a rondas hint; Shorts keeps the default. */
export function forgeHint(
  selectionLabel: string | null,
  presetLabel: string | null,
  emptySelectionHint: string = FORGE_HINT_EMPTY_PLAYS,
): string {
  if (selectionLabel == null) return emptySelectionHint;
  if (presetLabel == null) return FORGE_HINT_CHOOSE_PRESET;
  return FORGE_HINT_DECIDE_MUSIC;
}
