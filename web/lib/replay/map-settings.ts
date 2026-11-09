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

/** How far one turn of the wheel zooms, by the same three sizes. */
export const ZOOM_SPEED: Record<MapSize, number> = { s: 0.5, m: 1, l: 2 };
export const ZOOM_SPEED_LABELS: Record<MapSize, string> = { s: 'Lenta', m: 'Normal', l: 'Rápida' };

/** The most frames a second the map draws while playing; 0 draws every frame the screen shows. */
export const FRAME_RATES = [30, 60, 120, 0] as const;
export type FrameRate = (typeof FRAME_RATES)[number];

export function frameRateLabel(rate: FrameRate): string {
  return rate === 0 ? 'Sin límite' : String(rate);
}

/** What the map draws on and around the players, each on or off. */
export const MAP_SHOWS = ['killfeed', 'flashed', 'damage', 'trajectories', 'held'] as const;
export type MapShow = (typeof MAP_SHOWS)[number];
export type MapShows = Readonly<Record<MapShow, boolean>>;

export const MAP_SHOW_LABELS: Record<MapShow, string> = {
  killfeed: 'Feed de bajas',
  flashed: 'Cegados',
  damage: 'Daño recibido',
  trajectories: 'Trayectorias',
  held: 'Utilidad, bomba y kit',
};

export const MAP_SHOW_HINTS: Record<MapShow, string> = {
  killfeed: 'Las últimas bajas arriba a la derecha del mapa',
  flashed: 'Un centro blanco en el punto de quien está cegado',
  damage: 'Un aro rojo en el punto de quien acaba de recibir daño',
  trajectories: 'La parábola de cada granada mientras vuela',
  held: 'Las granadas que lleva cada jugador, la bomba y el kit',
};

/** How the Replay's map is drawn, kept per user across matches. Names are on or off per mode. */
export type MapSettings = {
  art: MapArtSource;
  dots: MapSize;
  names: MapSize;
  floors: MapFloors;
  zoom: MapSize;
  fps: FrameRate;
  show: MapShows;
  namesOne: boolean;
  namesMulti: boolean;
};

const ALL_SHOWN: MapShows = { killfeed: true, flashed: true, damage: true, trajectories: true, held: true };

export const DEFAULT_MAP_SETTINGS: MapSettings = { art: 'scheme', dots: 'm', names: 'm', floors: 'both', zoom: 'm', fps: 0, show: ALL_SHOWN, namesOne: true, namesMulti: true };

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
  const stored: { art?: unknown; dots?: unknown; names?: unknown; floors?: unknown; zoom?: unknown; fps?: unknown; show?: unknown; namesOne?: unknown; namesMulti?: unknown } = parsed;
  return {
    art: oneOf(MAP_ARTS, stored.art, DEFAULT_MAP_SETTINGS.art),
    dots: oneOf(MAP_SIZES, stored.dots, DEFAULT_MAP_SETTINGS.dots),
    names: oneOf(MAP_SIZES, stored.names, DEFAULT_MAP_SETTINGS.names),
    floors: oneOf(MAP_FLOORS, stored.floors, DEFAULT_MAP_SETTINGS.floors),
    zoom: oneOf(MAP_SIZES, stored.zoom, DEFAULT_MAP_SETTINGS.zoom),
    fps: FRAME_RATES.find((rate) => rate === stored.fps) ?? DEFAULT_MAP_SETTINGS.fps,
    show: parseShows(stored.show),
    namesOne: typeof stored.namesOne === 'boolean' ? stored.namesOne : DEFAULT_MAP_SETTINGS.namesOne,
    namesMulti: typeof stored.namesMulti === 'boolean' ? stored.namesMulti : DEFAULT_MAP_SETTINGS.namesMulti,
  };
}

function parseShows(value: unknown): MapShows {
  if (typeof value !== 'object' || value === null) return ALL_SHOWN;
  const out: Record<MapShow, boolean> = { ...ALL_SHOWN };
  for (const key of MAP_SHOWS) {
    const stored: unknown = Reflect.get(value, key);
    if (typeof stored === 'boolean') out[key] = stored;
  }
  return out;
}

/** The Nombres chip of the mode on screen: one round and the superposed rounds keep their own. */
export function namesShown(settings: MapSettings, multi: boolean): boolean {
  return multi ? settings.namesMulti : settings.namesOne;
}

export function toggleNames(settings: MapSettings, multi: boolean): MapSettings {
  return multi ? { ...settings, namesMulti: !settings.namesMulti } : { ...settings, namesOne: !settings.namesOne };
}
