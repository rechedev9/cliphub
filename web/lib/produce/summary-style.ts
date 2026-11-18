import type { CaptureHud, EditConfig } from '../api/types.ts';

/** The capture styles of a one-player summary, named as list_short_options names them. */
export const SUMMARY_STYLES = [
  { hud: 'gameplay', label: 'HUD completo', description: 'Con todo el HUD de CS2: vida, munición, radar y kill feed.' },
  { hud: 'deathnotices', label: 'Killfeed', description: 'Sin HUD, solo con la kill feed de CS2.' },
  { hud: 'clean', label: 'POV limpio', description: 'Sin HUD ni kill feed.' },
] as const satisfies readonly { hud: CaptureHud; label: string; description: string }[];

export function summaryStyle(edit: Pick<EditConfig, 'captureHud'>): (typeof SUMMARY_STYLES)[number] {
  return SUMMARY_STYLES.find((style) => style.hud === edit.captureHud) ?? SUMMARY_STYLES[0];
}

/** HUD completo is the native HUD of the 16:9 look, so only the other styles are stored. */
export function withSummaryStyle(edit: EditConfig, hud: CaptureHud): EditConfig {
  const next: EditConfig = { ...edit };
  if (hud === 'gameplay') delete next.captureHud;
  else next.captureHud = hud;
  return next;
}

/** Several players share one capture, so they get no pre-roll or capture style of their own. */
export function withoutOnePlayerOptions(edit: EditConfig): EditConfig {
  if (edit.preRollSeconds === undefined && edit.captureHud === undefined) return edit;
  const next: EditConfig = { ...edit };
  delete next.preRollSeconds;
  delete next.captureHud;
  return next;
}
