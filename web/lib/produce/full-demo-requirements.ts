import type { FullDemoOptions } from '../full-demo-plan.ts';

/** Footer error when a create attempt is blocked by an enabled option without its file. */
export const FULL_DEMO_MISSING_FILES = 'Faltan archivos por añadir. Añádelos o desactiva esas opciones antes de crear el vídeo.';

/**
 * True when the sponsor, intro or outro is switched on without its video, or a
 * sponsor that replaces its audio has no narration.
 *
 * The producer keeps these as quiet hints while the user edits and only turns
 * them into errors once a create attempt hits one.
 */
export function hasMissingFullDemoFiles(options: Pick<FullDemoOptions, 'sponsor' | 'bumpers'>): boolean {
  const sponsor = options.sponsor;
  if (sponsor.enabled && sponsor.video === null) return true;
  if (sponsor.enabled && sponsor.audio_policy === 'replace-narration' && sponsor.narration === null) return true;
  const bumpers = options.bumpers;
  if (!bumpers) return false;
  return (bumpers.intro.enabled && bumpers.intro.video === null) || (bumpers.outro.enabled && bumpers.outro.video === null);
}
