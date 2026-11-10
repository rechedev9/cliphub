import type { ScoutEvidence } from './api/scout.ts';
import type { HeatmapCell } from './api/spatial.ts';

/** One line of the heatmap's zone list: a named place on one floor, summing its 64 unit cells. */
export type HeatPlace = {
  /** The floor and the name, so a place is picked and highlighted as one. */
  key: string;
  name: string;
  level: string;
  count: number;
  /** The most rounds any one of its cells counted: the rounds of the place can only be more, so it is exact only for one cell. */
  rounds: number;
  /** Median seconds of its cells, weighted by what each counted. */
  seconds: number;
  cells: HeatmapCell[];
  /** At most three rounds from its busiest cells, to open. */
  examples: ScoutEvidence[];
};

const MAX_EXAMPLES = 3;

function medianSeconds(cells: readonly HeatmapCell[]): number {
  const byTime = [...cells].sort((a, b) => a.seconds - b.seconds);
  const half = byTime.reduce((sum, cell) => sum + cell.count, 0) / 2;
  let seen = 0;
  for (const cell of byTime) {
    seen += cell.count;
    if (seen >= half) return cell.seconds;
  }
  return 0;
}

function busiestFirst(a: HeatmapCell, b: HeatmapCell): number {
  return b.count - a.count || a.seconds - b.seconds || a.y - b.y || a.x - b.x;
}

/** The places of `cells`, busiest first. `nameOf` names the place at a cell; a name is never shared across floors. */
export function heatPlaces(options: { cells: readonly HeatmapCell[]; nameOf: (cell: HeatmapCell) => string }): HeatPlace[] {
  const { cells, nameOf } = options;
  const places = new Map<string, HeatmapCell[]>();
  const names = new Map<string, string>();
  for (const cell of cells) {
    const name = nameOf(cell);
    const key = `${cell.level}|${name}`;
    const known = places.get(key);
    if (known === undefined) places.set(key, [cell]);
    else known.push(cell);
    names.set(key, name);
  }
  const out: HeatPlace[] = [];
  for (const [key, own] of places) {
    const sorted = own.sort(busiestFirst);
    const examples: ScoutEvidence[] = [];
    for (const cell of sorted) {
      for (const example of cell.examples) {
        const seen = examples.some((entry) => entry.match_id === example.match_id && entry.round === example.round);
        if (!seen && examples.length < MAX_EXAMPLES) examples.push(example);
      }
    }
    out.push({
      key,
      name: names.get(key) ?? '',
      level: sorted[0].level,
      count: sorted.reduce((sum, cell) => sum + cell.count, 0),
      rounds: Math.max(...sorted.map((cell) => cell.rounds)),
      seconds: medianSeconds(sorted),
      cells: sorted,
      examples,
    });
  }
  return out.sort((a, b) => b.count - a.count || a.seconds - b.seconds || a.key.localeCompare(b.key));
}
