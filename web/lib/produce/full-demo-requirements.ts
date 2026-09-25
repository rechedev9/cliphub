import type { FullDemoOptions } from '../full-demo-plan.ts';

/** Footer error when a create attempt is blocked by an enabled option without its file. */
export const FULL_DEMO_MISSING_FILES = 'Faltan archivos por añadir. Añádelos o desactiva esas opciones antes de crear el vídeo.';

/**
 * True when the intro, sponsor or outro slot is switched on without its video.
 *
 * The producer keeps these as quiet hints while the user edits and only turns
 * them into errors once a create attempt hits one.
 */
export function hasMissingFullDemoFiles(options: Pick<FullDemoOptions, 'bumpers'>): boolean {
  const bumpers = options.bumpers;
  if (!bumpers) return false;
  return [bumpers.intro, bumpers.sponsor, bumpers.outro].some((slot) => slot?.enabled === true && slot.video === null);
}
