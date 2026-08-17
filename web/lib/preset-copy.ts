// Spanish picker copy; registry descriptions stay English. Unknown names fall back to the API string.

/** Spanish registry descriptions keyed by preset name. Keep in sync with preset.go. */
export const PRESET_DESCRIPTION_ES: Record<string, string> = {
  'viral-60-clean':
    'Edición viral limpia por defecto: POV a 60fps sin HUD que conserva el kill feed del juego, con punch-in en las bajas.',
  'clean-pov-60':
    'POV en primera persona totalmente sin HUD: punch-in cinematográfico en las bajas, sin HUD ni kill feed del juego.',
  'full-hud-60':
    'POV con el HUD completo del juego: mantiene visibles el HUD de CS2, la vida, la munición y el radar sobre la edición viral.',
  'viral-aggressive-60':
    'Edición TikTok agresiva: POV a 60fps sin HUD que conserva el kill feed, con grade saturado y pulsos de croma en los headshots.',
};

/** Localized description, or the API English string for unknown presets. */
export function presetDescription(preset: { name: string; description: string }): string {
  return PRESET_DESCRIPTION_ES[preset.name] ?? preset.description;
}
