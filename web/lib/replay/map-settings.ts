export const MAP_ARTS = ['scheme', 'official', 'real'] as const;
export type MapArtSource = (typeof MAP_ARTS)[number];

export const MAP_ART_LABELS: Record<MapArtSource, string> = {
  scheme: 'Esquema',
  official: 'Radar oficial',
  real: 'Mapa real',
};

export const MAP_SIZES = ['s', 'm', 'l'] as const;
export type MapSize = (typeof MAP_SIZES)[number];

export const MAP_SIZE_LABELS: Record<MapSize, string> = { s: 'Pequeño', m: 'Mediano', l: 'Grande' };

/** How much each size multiplies the player dots and the name tags. */
export const MAP_SIZE_SCALE: Record<MapSize, number> = { s: 0.8, m: 1, l: 1.3 };

/** On a two-floor map (Nuke, Vertigo): the floor the players are on, or both side by side. */
export const MAP_FLOORS = ['one', 'both'] as const;
export type MapFloors = (typeof MAP_FLOORS)[number];

export const MAP_FLOORS_LABELS: Record<MapFloors, string> = { one: 'Un piso', both: 'Los dos' };

/** How the Replay's map is drawn, kept per user across matches. */
export type MapSettings = { art: MapArtSource; dots: MapSize; names: MapSize; floors: MapFloors };

export const DEFAULT_MAP_SETTINGS: MapSettings = { art: 'scheme', dots: 'm', names: 'm', floors: 'both' };

export const MAP_SETTINGS_KEY = 'cliphub.replay.map';

function oneOf<T extends string>(values: readonly T[], value: unknown, fallback: T): T {
  return values.find((entry) => entry === value) ?? fallback;
}

/** Reads stored settings; anything missing or unknown falls back to the default. */
export function parseMapSettings(raw: string | null): MapSettings {
  if (raw === null) return DEFAULT_MAP_SETTINGS;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return DEFAULT_MAP_SETTINGS;
  }
  if (typeof parsed !== 'object' || parsed === null) return DEFAULT_MAP_SETTINGS;
  const stored: { art?: unknown; dots?: unknown; names?: unknown; floors?: unknown } = parsed;
  return {
    art: oneOf(MAP_ARTS, stored.art, DEFAULT_MAP_SETTINGS.art),
    dots: oneOf(MAP_SIZES, stored.dots, DEFAULT_MAP_SETTINGS.dots),
    names: oneOf(MAP_SIZES, stored.names, DEFAULT_MAP_SETTINGS.names),
    floors: oneOf(MAP_FLOORS, stored.floors, DEFAULT_MAP_SETTINGS.floors),
  };
}
