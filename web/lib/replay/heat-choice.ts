import { HEATMAP_GRENADES, HEATMAP_KINDS } from '../api/spatial.ts';
import type { HeatmapGrenade, HeatmapKind, KillSpot } from '../api/spatial.ts';

/**
 * What the Mapa de calor counts, kept in the URL so a link, or the
 * `studio_path` of `match_heatmap` and `team_heatmap`, opens the same view.
 */
export type HeatChoice = { kind: HeatmapKind; killAt: KillSpot; grenades: HeatmapGrenade[] };

export const DEFAULT_HEAT: HeatChoice = { kind: 'position', killAt: 'killer', grenades: [] };

/** Query keys: `cuenta` (left out for positions), `baja=victim` for a kill map on the victims, `granada` per grenade of a utility map. */
export const HEAT_QUERY = { kind: 'cuenta', killAt: 'baja', grenade: 'granada' } as const;

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
  return { kind, killAt, grenades };
}

/** The query with the choice written over it, every other key kept. */
export function writeHeatChoice(current: string, choice: HeatChoice): string {
  const params = new URLSearchParams(current);
  for (const key of Object.values(HEAT_QUERY)) params.delete(key);
  if (choice.kind !== DEFAULT_HEAT.kind) params.set(HEAT_QUERY.kind, choice.kind);
  if (choice.kind === 'kill' && choice.killAt === 'victim') params.set(HEAT_QUERY.killAt, 'victim');
  if (choice.kind === 'utility') for (const grenade of choice.grenades) params.append(HEAT_QUERY.grenade, grenade);
  return params.toString();
}
