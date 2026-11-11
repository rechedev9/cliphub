export const MAP_SIZES = ['s', 'm', 'l'] as const;
export type MapSize = (typeof MAP_SIZES)[number];

export const MAP_SIZE_LABELS: Record<MapSize, string> = { s: 'Pequeño', m: 'Mediano', l: 'Grande' };

/** How much each size multiplies the name tags. */
export const MAP_SIZE_SCALE: Record<MapSize, number> = { s: 0.8, m: 1, l: 1.3 };

/** The dots take a fourth step, Extra grande; stored settings with the first three keep reading. */
export const DOT_SIZES = ['s', 'm', 'l', 'xl'] as const;
export type DotSize = (typeof DOT_SIZES)[number];
export const DOT_SIZE_LABELS: Record<DotSize, string> = { ...MAP_SIZE_LABELS, xl: 'Extra grande' };
export const DOT_SIZE_SCALE: Record<DotSize, number> = { ...MAP_SIZE_SCALE, xl: 1.7 };

/**
 * On a two-floor map (Nuke, Vertigo): `both` is Separar pisos checked, each floor
 * in its own block; `one` is it unchecked, the floor most players are on. The
 * stored words stay as they were, so saved preferences keep reading.
 */
export const MAP_FLOORS = ['one', 'both'] as const;
export type MapFloors = (typeof MAP_FLOORS)[number];

export const SEPARATE_FLOORS_LABEL = 'Separar pisos';
export const SEPARATE_FLOORS_HINT = 'Cada piso en su bloque, a la misma escala. Apagado, el mapa muestra el piso donde está la mayoría';

export function floorsSeparated(settings: Pick<MapSettings, 'floors'>): boolean {
  return settings.floors === 'both';
}

/** How far one turn of the wheel zooms, by the same three sizes. */
export const ZOOM_SPEED: Record<MapSize, number> = { s: 0.5, m: 1, l: 2 };
export const ZOOM_SPEED_LABELS: Record<MapSize, string> = { s: 'Lenta', m: 'Normal', l: 'Rápida' };

/** The most frames a second the map draws while playing; 0 draws every frame the screen shows. */
export const FRAME_RATES = [30, 60, 120, 0] as const;
export type FrameRate = (typeof FRAME_RATES)[number];

export function frameRateLabel(rate: FrameRate): string {
  return rate === 0 ? 'Sin límite' : String(rate);
}

/** What the replay draws on and around the players and on its timeline, each on or off. */
export const MAP_SHOWS = ['killfeed', 'scoreline', 'flashed', 'damage', 'trajectories', 'bullets', 'held', 'bomb', 'kit', 'tlEvents', 'tlUtility', 'tlKills', 'freezetime'] as const;
export type MapShow = (typeof MAP_SHOWS)[number];
export type MapShows = Readonly<Record<MapShow, boolean>>;

export const MAP_SHOW_LABELS: Record<MapShow, string> = {
  killfeed: 'Feed de bajas',
  scoreline: 'Marcador arriba',
  flashed: 'Cegado',
  damage: 'Daño recibido',
  trajectories: 'Trayectorias de granadas',
  bullets: 'Trazas de disparo',
  held: 'Utilidad en mano',
  bomb: 'Bomba',
  kit: 'Kit de desactivación',
  tlEvents: 'Eventos',
  tlUtility: 'Utilidades',
  tlKills: 'Bajas',
  freezetime: 'Freeze time',
};

export const MAP_SHOW_HINTS: Record<MapShow, string> = {
  killfeed: 'Las últimas bajas arriba a la derecha del mapa',
  scoreline: 'El marcador con el reloj de la ronda arriba a la izquierda del mapa',
  flashed: 'Un centro blanco en el punto de quien está cegado',
  damage: 'Un aro rojo en el punto de quien acaba de recibir daño',
  trajectories: 'La parábola de cada granada mientras vuela',
  bullets: 'Una línea del atacante a la víctima en cada impacto',
  held: 'Un punto bajo cada jugador por cada granada que lleva',
  bomb: 'La bomba en el punto de quien la lleva',
  kit: 'El kit de desactivación en el punto de quien lo lleva',
  tlEvents: 'Las marcas de la bomba en la línea de tiempo',
  tlUtility: 'Una marca por cada granada en la línea de tiempo',
  tlKills: 'Una marca por cada baja en la línea de tiempo',
  freezetime: 'Apagado, cada ronda empieza y vuelve a empezar cuando ya está en vivo',
};

/** How the Replay's map is drawn, kept per user across matches. Names are on or off per mode. */
export type MapSettings = {
  dots: DotSize;
  names: MapSize;
  floors: MapFloors;
  zoom: MapSize;
  fps: FrameRate;
  show: MapShows;
  namesOne: boolean;
  namesMulti: boolean;
};

const DEFAULT_SHOWS: MapShows = {
  killfeed: true,
  scoreline: true,
  flashed: true,
  damage: true,
  trajectories: true,
  bullets: true,
  held: true,
  bomb: true,
  kit: true,
  tlEvents: true,
  tlUtility: true,
  tlKills: true,
  freezetime: true,
};

export const DEFAULT_MAP_SETTINGS: MapSettings = { dots: 'm', names: 'm', floors: 'both', zoom: 'm', fps: 0, show: DEFAULT_SHOWS, namesOne: true, namesMulti: true };

export const MAP_SETTINGS_KEY = 'cliphub.replay.map';

function oneOf<T extends string>(values: readonly T[], value: unknown, fallback: T): T {
  return values.find((entry) => entry === value) ?? fallback;
}

/** Reads stored settings; anything missing or unknown, such as a setting the replay no longer has, falls back to the default or is ignored. */
export function parseMapSettings(raw: string | null): MapSettings {
  if (raw === null) return DEFAULT_MAP_SETTINGS;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return DEFAULT_MAP_SETTINGS;
  }
  if (typeof parsed !== 'object' || parsed === null) return DEFAULT_MAP_SETTINGS;
  const stored: { dots?: unknown; names?: unknown; floors?: unknown; zoom?: unknown; fps?: unknown; show?: unknown; namesOne?: unknown; namesMulti?: unknown } = parsed;
  return {
    dots: oneOf(DOT_SIZES, stored.dots, DEFAULT_MAP_SETTINGS.dots),
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
  if (typeof value !== 'object' || value === null) return DEFAULT_SHOWS;
  const out: Record<MapShow, boolean> = { ...DEFAULT_SHOWS };
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
