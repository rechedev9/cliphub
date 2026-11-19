import { customHudTheme } from '../custom-hud.ts';
import { summaryStyle } from '../produce/summary-style.ts';
import { isLandscapeHighlights } from '../reel-brief.ts';
import type { EditConfig } from './types.ts';

/** Display-only labels for the known presets (server is the source of truth). */
const VARIANT_LABELS: Readonly<Record<string, string>> = {
  'viral-60-clean': 'Killfeed',
  'clean-pov-60': 'POV limpio',
  'full-hud-60': 'HUD completo',
  'gameplay-pov-60': 'POV nativo',
};

/** The style a video's title names: a 16:9 video of chosen plays names its capture style, not its preset. */
export function reelStyleLabel(variant: string, edit?: EditConfig): string {
  const hud = customHudTheme(edit?.fullDemo?.document.options.overlays.hud_theme);
  if (hud) return `HUD ${hud.name}`;
  if (edit !== undefined && !edit.fullDemo && isLandscapeHighlights(edit)) return summaryStyle(edit.nativeHud ? {} : edit).label;
  return VARIANT_LABELS[variant] ?? variant;
}
