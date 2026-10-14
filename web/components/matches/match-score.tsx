export function parseScore(score: string): { ours: number | null; theirs: number | null } {
  const [left, right] = score.split('-', 2);
  const ours = Number.parseInt(left ?? '', 10);
  const theirs = Number.parseInt(right ?? '', 10);
  return {
    ours: Number.isNaN(ours) ? null : ours,
    theirs: Number.isNaN(theirs) ? null : theirs,
  };
}
