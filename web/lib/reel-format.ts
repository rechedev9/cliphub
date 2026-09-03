import type { Preset, RenderFormat } from './api/types.ts';
import { FULL_DEMO_VARIANT } from './full-demo.ts';

export function isLandscapePreset(preset: Pick<Preset, 'name' | 'width' | 'height'>): boolean {
  if (typeof preset.width === 'number' && typeof preset.height === 'number' && preset.width > 0 && preset.height > 0) {
    return preset.width > preset.height;
  }
  return preset.name === FULL_DEMO_VARIANT;
}

/** Matches outputShapeForEdit: landscape format wins; else a 16:9 preset keeps its size. */
export function resolvedReelFormat(format: RenderFormat, preset: Preset | null): RenderFormat {
  if (format === 'landscape-16x9') return 'landscape-16x9';
  if (preset && isLandscapePreset(preset)) return 'landscape-16x9';
  return 'short-9x16';
}

export function shortsPresetsForFormat(presets: Preset[], format: RenderFormat): Preset[] {
  if (format === 'landscape-16x9') return presets;
  return presets.filter((preset) => !isLandscapePreset(preset));
}

export function selectShortsPreset(
  presetName: string,
  currentFormat: RenderFormat,
  presets: Preset[],
): { format: RenderFormat; variant: string } {
  const preset = presets.find((item) => item.name === presetName);
  if (preset && isLandscapePreset(preset)) {
    return { format: 'landscape-16x9', variant: presetName };
  }
  return { format: currentFormat, variant: presetName };
}

export function selectShortsFormat(
  format: RenderFormat,
  currentVariant: string | null,
  presets: Preset[],
): { format: RenderFormat; variant: string | null } {
  const visible = shortsPresetsForFormat(presets, format);
  if (currentVariant != null && visible.some((preset) => preset.name === currentVariant)) {
    return { format, variant: currentVariant };
  }
  return { format, variant: visible.find((preset) => preset.default)?.name ?? visible[0]?.name ?? null };
}
