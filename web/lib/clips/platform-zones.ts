export const PLATFORMS = ['youtube-shorts', 'tiktok', 'reels'] as const;

export type Platform = (typeof PLATFORMS)[number];

export type ZoneRect = { left: number; top: number; right: number; bottom: number };

type ZoneArea = 'barra superior' | 'botones' | 'texto y audio' | 'navegación';

const ZONES: Record<Platform, Record<ZoneArea, ZoneRect>> = {
  'youtube-shorts': {
    'barra superior': { left: 0, top: 0, right: 1, bottom: 0.15 },
    botones: { left: 0.822, top: 0.4, right: 1, bottom: 0.65 },
    'texto y audio': { left: 0, top: 0.65, right: 1, bottom: 0.935 },
    navegación: { left: 0, top: 0.935, right: 1, bottom: 1 },
  },
  tiktok: {
    'barra superior': { left: 0, top: 0, right: 1, bottom: 0.125 },
    botones: { left: 0.833, top: 0.4375, right: 1, bottom: 0.656 },
    'texto y audio': { left: 0, top: 0.656, right: 1, bottom: 0.935 },
    navegación: { left: 0, top: 0.935, right: 1, bottom: 1 },
  },
  reels: {
    'barra superior': { left: 0, top: 0, right: 1, bottom: 0.14 },
    botones: { left: 0.85, top: 0.5, right: 1, bottom: 0.65 },
    'texto y audio': { left: 0, top: 0.65, right: 1, bottom: 0.935 },
    navegación: { left: 0, top: 0.935, right: 1, bottom: 1 },
  },
};

export type PlatformLayer = 'todas' | Platform | 'ninguna';

export function platformZones(platform: Platform): readonly ZoneRect[] {
  return Object.values(ZONES[platform]);
}

function contains(outer: ZoneRect, inner: ZoneRect): boolean {
  return outer.left <= inner.left && outer.top <= inner.top && outer.right >= inner.right && outer.bottom >= inner.bottom;
}

function unionOfPlatforms(): readonly ZoneRect[] {
  const all = PLATFORMS.flatMap(platformZones);
  return all.filter((zone, at) => !all.some((other, otherAt) => otherAt !== at && contains(other, zone) && (!contains(zone, other) || otherAt < at)));
}

export const PLATFORM_LAYER_OPTIONS: readonly { value: PlatformLayer; label: string; narrowLabel?: string }[] = [
  { value: 'todas', label: 'Todas' },
  { value: 'youtube-shorts', label: 'YouTube Shorts', narrowLabel: 'YouTube' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'reels', label: 'Reels' },
  { value: 'ninguna', label: 'Ninguna' },
];

export function parsePlatformLayer(stored: string | null): PlatformLayer {
  return PLATFORM_LAYER_OPTIONS.find((option) => option.value === stored)?.value ?? 'todas';
}

export function layerZones(layer: PlatformLayer): readonly ZoneRect[] {
  if (layer === 'ninguna') return [];
  if (layer === 'todas') return unionOfPlatforms();
  return platformZones(layer);
}
