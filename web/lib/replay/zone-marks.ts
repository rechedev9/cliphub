/** The URL key of the zones a finding's link marks; its players go in `jugador`, like the Ranking's links. */
export const ZONE_QUERY = 'zona';
export const ZONE_MARKS_MAX = 12;

const ZONE_TOKEN = /^[^\p{Cc}]{1,64}$/u;

function distinct(values: readonly string[], keep: (value: string) => boolean, max: number): string[] {
  const out: string[] = [];
  for (const value of values) {
    if (out.length === max) break;
    if (keep(value) && !out.includes(value)) out.push(value);
  }
  return out;
}

/** The zones a link marks, as CS2 place tokens; malformed or repeated ones are dropped. */
export function zonesFromQuery(params: Pick<URLSearchParams, 'getAll'>): string[] {
  return distinct(params.getAll(ZONE_QUERY), (zone) => ZONE_TOKEN.test(zone), ZONE_MARKS_MAX);
}
