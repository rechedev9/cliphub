import type { EditConfig, Preset } from './api/types.ts';

/** Mirrors internal/radaroverlay: the corners, sizes and opacity range of ClipHub's radar. */
export const RADAR_POSITIONS = ['top-left', 'top-right', 'bottom-left', 'bottom-right'] as const;
export type RadarPosition = (typeof RADAR_POSITIONS)[number];

export const RADAR_SIZES = ['small', 'medium', 'large'] as const;
export type RadarSize = (typeof RADAR_SIZES)[number];

export const RADAR_MIN_OPACITY = 0.4;
export const RADAR_DEFAULT_OPACITY = 0.9;

export const RADAR_POSITION_LABEL: Record<RadarPosition, string> = {
  'top-left': 'Arriba a la izquierda',
  'top-right': 'Arriba a la derecha',
  'bottom-left': 'Abajo a la izquierda',
  'bottom-right': 'Abajo a la derecha',
};

export const RADAR_SIZE_LABEL: Record<RadarSize, string> = {
  small: 'Pequeño',
  medium: 'Mediano',
  large: 'Grande',
};

export function isRadarPosition(value: unknown): value is RadarPosition {
  return RADAR_POSITIONS.some((position) => position === value);
}

export function isRadarSize(value: unknown): value is RadarSize {
  return RADAR_SIZES.some((size) => size === value);
}

/** A chosen opacity the server takes; the default and anything out of range read as unset. */
export function coerceRadarOpacity(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined;
  if (value < RADAR_MIN_OPACITY || value > 1 || value === RADAR_DEFAULT_OPACITY) return undefined;
  return value;
}

/** The corner the format picks when none is chosen: above the caption band in 9:16, top left in 16:9. */
export function radarAutoPosition(format: EditConfig['format']): RadarPosition {
  return format === 'short-9x16' ? 'bottom-left' : 'top-left';
}

/** A style that records CS2's HUD already shows CS2's radar, so ClipHub's radar is refused over it. */
export function radarAllowed(preset: Pick<Preset, 'hudMode'> | null): boolean {
  return preset?.hudMode !== 'gameplay';
}

/** The radar fields as sent: only a Short of plays recorded without CS2's HUD carries them. */
export function radarEditFields(edit: EditConfig): Pick<EditConfig, 'radar' | 'radarPosition' | 'radarSize' | 'radarOpacity'> {
  if (!edit.radar || edit.fullDemo || edit.matchRecap || edit.nativeHud) return {};
  const fields: Pick<EditConfig, 'radar' | 'radarPosition' | 'radarSize' | 'radarOpacity'> = { radar: true };
  if (edit.radarPosition) fields.radarPosition = edit.radarPosition;
  if (edit.radarSize && edit.radarSize !== 'medium') fields.radarSize = edit.radarSize;
  const opacity = coerceRadarOpacity(edit.radarOpacity);
  if (opacity !== undefined) fields.radarOpacity = opacity;
  return fields;
}

/** "Abajo a la izquierda · Mediano · 90 %" for the creative brief; "No" without the radar. */
export function radarBriefLabel(edit: EditConfig): string {
  const fields = radarEditFields(edit);
  if (!fields.radar) return 'No';
  const position = fields.radarPosition ?? radarAutoPosition(edit.format);
  const opacity = Math.round((fields.radarOpacity ?? RADAR_DEFAULT_OPACITY) * 100);
  return `${RADAR_POSITION_LABEL[position]} · ${RADAR_SIZE_LABEL[fields.radarSize ?? 'medium']} · ${opacity} %`;
}
