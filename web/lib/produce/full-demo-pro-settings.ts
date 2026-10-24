import type { FullDemoOptions, FullDemoProPlayer, FullDemoProSettings } from '../full-demo-plan.ts';

const ASPECTS = [[4, 3], [5, 4], [16, 10], [16, 9]] as const;
const SCALING_LABEL: Record<FullDemoProSettings['scaling'], string> = { stretched: 'estirado', 'black-bars': 'con bandas negras', native: 'nativo' };

export function withProPlayer(options: FullDemoOptions, player: FullDemoProPlayer | null): FullDemoOptions {
  const { pro_settings: _previous, ...capture } = options.capture;
  return { ...options, capture: player ? { ...capture, pro_settings: player.settings } : capture };
}

function aspectLabel(width: number, height: number): string {
  const ratio = width / height;
  const match = ASPECTS.find(([w, h]) => Math.abs(w / h - ratio) < .01);
  return match ? `${match[0]}:${match[1]}` : `${width}x${height}`;
}

/** "1280x960 · 4:3 estirado"; a 16:9 resolution has no scaling to name. */
export function proResolutionLabel({ width, height, scaling }: Pick<FullDemoProSettings, 'width' | 'height' | 'scaling'>): string {
  const aspect = aspectLabel(width, height);
  return aspect === '16:9' ? `${width}x${height} · 16:9` : `${width}x${height} · ${aspect} ${SCALING_LABEL[scaling]}`;
}

export function proViewmodelLabel(settings: FullDemoProSettings): string {
  return `FOV ${settings.viewmodel_fov} · X ${settings.viewmodel_offset_x} · Y ${settings.viewmodel_offset_y} · Z ${settings.viewmodel_offset_z}`;
}

export function proSettingsSummary(settings: FullDemoProSettings | undefined): string {
  return settings ? `${settings.name} · ${proResolutionLabel(settings)}` : 'Los tuyos';
}
