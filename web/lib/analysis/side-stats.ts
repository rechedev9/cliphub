import { COMPARE_TEAMS_MAX, type SideStat, type SideStatKey } from '../api/side-stats.ts';
import { formatDecimal } from '../format.ts';
import { compareDelta, deltaArrow, type DeltaTone } from './compare.ts';

/**
 * The Estadísticas tab's arithmetic: the tiles in their order with labels and
 * formulas, how a figure and its difference to the group read, and the
 * "Comparar con" teams kept in the URL.
 */

type Unit = 'pct' | 'per-round' | 'count';

export type SideStatTile = { readonly key: SideStatKey; readonly label: string; readonly unit: Unit; readonly formula?: string };

export const SIDE_STAT_TILES: readonly SideStatTile[] = [
  { key: 'maps_won', label: 'Mapas ganados', unit: 'pct', formula: 'Mapas en que el equipo sumó más rondas que el rival, sobre los mapas con alguna ronda en el alcance.' },
  { key: 'rounds_won', label: 'Rondas ganadas', unit: 'pct', formula: 'Rondas ganadas sobre las rondas jugadas en el alcance.' },
  { key: 'pistols_won', label: 'Pistolas ganadas', unit: 'pct', formula: 'Rondas de pistola ganadas sobre las rondas de pistola del alcance.' },
  { key: 'opening_kills', label: 'Primera baja', unit: 'pct', formula: 'Rondas en que el equipo hizo la primera baja, sobre las rondas con alguna baja.' },
  { key: 'won_5v4', label: '5 contra 4 ganadas', unit: 'pct', formula: 'Rondas ganadas tras hacer la primera baja, sobre las rondas en que la hizo.' },
  { key: 'won_4v5', label: '4 contra 5 ganadas', unit: 'pct', formula: 'Rondas ganadas tras sufrir la primera baja, sobre las rondas en que la sufrió.' },
  { key: 'trades', label: 'Trades', unit: 'pct', formula: 'Muertes del equipo vengadas por un compañero en 5 segundos, sobre sus muertes.' },
  {
    key: 'utility_damage_per_round',
    label: 'Daño de utilidad por ronda',
    unit: 'per-round',
    formula: 'Daño de HE y molotov a rivales, sobre las rondas de partidas analizadas con efectos de utilidad.',
  },
  {
    key: 'flash_assists_per_round',
    label: 'Asistencias de flash por ronda',
    unit: 'per-round',
    formula: 'Bajas de un compañero sobre un rival que el equipo había cegado, mientras seguía ciego, por ronda.',
  },
  { key: 'rounds_played', label: 'Rondas jugadas', unit: 'count', formula: 'Rondas con ganador que cuentan en el alcance.' },
];

/** The text of the rating's (i): ClipHub's own rating, never HLTV's. */
export const RATING_FORMULA_TEXT =
  'Rating de ClipHub: la media de KPR / 0,68, (1 - DPR) / 0,32 y ADR / 75. Un jugador medio da 1,00.';

const NO_DATA = '-';

/** A figure in its unit: "52 %", "8,4", "37"; "-" without data, never 0 %. */
export function formatSideValue(value: number | null, unit: Unit): string {
  if (value === null) return NO_DATA;
  if (unit === 'pct') return `${Math.round(value)} %`;
  if (unit === 'per-round') return formatDecimal(value, 2);
  return String(Math.round(value));
}

export type SideDelta = { readonly text: string; readonly tone: DeltaTone };

/** "▼ 15 %": the difference to the group mean in the figure's own unit; null when either is missing. */
export function sideDelta(stat: SideStat, unit: Unit): SideDelta | null {
  if (stat.value === null || stat.group_avg === null) return null;
  const epsilon = unit === 'per-round' ? 0.005 : 0.5;
  const { delta, tone } = compareDelta(stat.value, stat.group_avg, { higherIsBetter: true, epsilon });
  if (tone === 'even') return { text: '=', tone };
  return { text: `${deltaArrow(delta, epsilon)} ${formatSideValue(Math.abs(delta), unit)}`, tone };
}

/** "media 75 %", the group mean under the tile. */
export function groupMeanText(stat: SideStat, unit: Unit): string {
  return stat.group_avg === null ? 'Sin media del grupo' : `media ${formatSideValue(stat.group_avg, unit)}`;
}

/** The URL key of the "Comparar con" teams: comma separated team ids; none is the whole library. */
export const COMPARE_PARAM = 'comparar';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** The chosen teams, sorted and without repeats; one bad id drops the set back to the library. */
export function parseCompareTeams(value: string | null): string[] {
  if (value === null || value === '') return [];
  const ids = value.toLowerCase().split(',');
  if (ids.length > COMPARE_TEAMS_MAX || !ids.every((id) => UUID_RE.test(id))) return [];
  return [...new Set(ids)].sort();
}

/** The query string with the chosen teams written over `current`; the library leaves the key out. */
export function writeCompareTeams(current: string, ids: readonly string[]): string {
  const params = new URLSearchParams(current);
  const sorted = [...new Set(ids)].sort();
  if (sorted.length === 0) params.delete(COMPARE_PARAM);
  else params.set(COMPARE_PARAM, sorted.join(','));
  return params.toString();
}

/** One team in or out of the set; the last one out returns to the library. */
export function toggleCompareTeam(ids: readonly string[], id: string): string[] {
  const next = ids.includes(id) ? ids.filter((entry) => entry !== id) : [...ids, id];
  return [...new Set(next)].sort().slice(0, COMPARE_TEAMS_MAX);
}
