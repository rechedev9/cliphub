import { HEATMAP_GRENADES, HEATMAP_KINDS } from '../api/spatial.ts';
import type { HeatmapGrenade, HeatmapKind, KillSpot } from '../api/spatial.ts';
import { RADAR_LEVELS } from '../api/tactical.ts';
import type { RadarLevel } from '../api/tactical.ts';

/**
 * What the Mapa de calor counts, kept in the URL so a link, or the
 * `studio_path` of `match_heatmap` and `team_heatmap`, opens the same view.
 */
export type HeatChoice = { kind: HeatmapKind; killAt: KillSpot; grenades: HeatmapGrenade[]; floor: HeatFloor };

/** The floor of a two-floor map the heatmap shows: both side by side, or one. */
export type HeatFloor = RadarLevel | 'both';

export const DEFAULT_HEAT: HeatChoice = { kind: 'position', killAt: 'killer', grenades: [], floor: 'both' };

/** Query keys: `cuenta` (left out for positions), `baja=victim` for a kill map on the victims, `granada` per grenade of a utility map, `piso=superior|inferior` for one floor. */
export const HEAT_QUERY = { kind: 'cuenta', killAt: 'baja', grenade: 'granada', floor: 'piso' } as const;

const FLOOR_QUERY: Readonly<Record<RadarLevel, string>> = { [RADAR_LEVELS.default]: 'superior', [RADAR_LEVELS.lower]: 'inferior' };

function heatFloorOf(raw: string | null): HeatFloor {
  if (raw === FLOOR_QUERY.lower) return RADAR_LEVELS.lower;
  if (raw === FLOOR_QUERY.default) return RADAR_LEVELS.default;
  return 'both';
}

export function heatChoiceFromQuery(query: Pick<URLSearchParams, 'get' | 'getAll'>): HeatChoice {
  const rawKind = query.get(HEAT_QUERY.kind);
  const kind = HEATMAP_KINDS.find((entry) => entry === rawKind) ?? DEFAULT_HEAT.kind;
  const killAt: KillSpot = kind === 'kill' && query.get(HEAT_QUERY.killAt) === 'victim' ? 'victim' : 'killer';
  const grenades: HeatmapGrenade[] = [];
  if (kind === 'utility') {
    for (const raw of query.getAll(HEAT_QUERY.grenade)) {
      const grenade = HEATMAP_GRENADES.find((entry) => entry === raw);
      if (grenade !== undefined && !grenades.includes(grenade)) grenades.push(grenade);
    }
  }
  const rawFloor = query.get(HEAT_QUERY.floor);
  const floor = heatFloorOf(rawFloor);
  return { kind, killAt, grenades, floor };
}

/** The query with the choice written over it, every other key kept. */
export function writeHeatChoice(current: string, choice: HeatChoice): string {
  const params = new URLSearchParams(current);
  for (const key of Object.values(HEAT_QUERY)) params.delete(key);
  if (choice.kind !== DEFAULT_HEAT.kind) params.set(HEAT_QUERY.kind, choice.kind);
  if (choice.kind === 'kill' && choice.killAt === 'victim') params.set(HEAT_QUERY.killAt, 'victim');
  if (choice.kind === 'utility') for (const grenade of choice.grenades) params.append(HEAT_QUERY.grenade, grenade);
  if (choice.floor !== 'both') params.set(HEAT_QUERY.floor, FLOOR_QUERY[choice.floor]);
  return params.toString();
}
