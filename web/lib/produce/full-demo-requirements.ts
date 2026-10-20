import type { FullDemoOptions } from '../full-demo-plan.ts';

/** Footer error when a create attempt is blocked by an enabled option without its file. */
export const FULL_DEMO_MISSING_FILES = 'Faltan archivos por añadir. Añádelos o desactiva esas opciones antes de crear el vídeo.';

type Bumpers = NonNullable<FullDemoOptions['bumpers']>;

function enabledBumpers(options: Pick<FullDemoOptions, 'bumpers'>): Bumpers['intro'][] {
  const bumpers = options.bumpers;
  if (!bumpers) return [];
  return [bumpers.intro, bumpers.sponsor, bumpers.outro].filter((slot): slot is Bumpers['intro'] => slot?.enabled === true);
}

/**
 * True when the intro, sponsor or outro slot is switched on without its video.
 *
 * The producer keeps these as quiet hints while the user edits and only turns
 * them into errors once a create attempt hits one.
 */
export function hasMissingFullDemoFiles(options: Pick<FullDemoOptions, 'bumpers'>): boolean {
  return enabledBumpers(options).some((slot) => slot.video === null);
}

/** True when the render reads a file the user uploaded; currentFullDemoOptions already drops music and screenshot images. */
export function usesOwnFullDemoFiles(options: Pick<FullDemoOptions, 'bumpers' | 'overlays'>): boolean {
  if (options.overlays.hud_theme === 'focus' && options.overlays.hud_portrait != null) return true;
  return enabledBumpers(options).some((slot) => slot.video !== null);
}
