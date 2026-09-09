// Generated with the previews by `go run ./cmd/zv-hud-designs`.
import catalog from '../public/hud/catalog.json' with { type: 'json' };

export const CUSTOM_HUD_CAPTURE_PROFILE = 'broadcast-clean';
export const CUSTOM_HUD_THEMES = catalog;
export type CustomHudTheme = (typeof catalog)[number];

export function isCustomHudTheme(value: unknown): value is string {
  return typeof value === 'string' && catalog.some((theme) => theme.id === value);
}

export function customHudTheme(id: string | undefined): CustomHudTheme | undefined {
  return catalog.find((theme) => theme.id === id);
}

export function customHudLabel(id: string | undefined): string {
  return customHudTheme(id)?.name ?? 'Nativo CS2';
}
