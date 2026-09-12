/**
 * Parse a "rounds-rounds" score string (e.g. "13-7") into its two halves.
 * Returns null for either side that is not a number so callers can fall back.
 *
 * This is the ONLY score parser in the app. `matches/[id]/page.tsx` used to ship
 * a second, regex-based copy whose edge cases diverged from this one (it
 * rejected "13 - 7", which `parseInt` accepts here) for the same domain concept,
 * two files apart in the import graph.
 */
export function parseScore(score: string): { ours: number | null; theirs: number | null } {
  const [left, right] = score.split('-', 2);
  const ours = Number.parseInt(left ?? '', 10);
  const theirs = Number.parseInt(right ?? '', 10);
  return {
    ours: Number.isNaN(ours) ? null : ours,
    theirs: Number.isNaN(theirs) ? null : theirs,
  };
}
