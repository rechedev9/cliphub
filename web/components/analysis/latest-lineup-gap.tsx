import type { ReactNode } from 'react';
import { Users } from 'lucide-react';
import { latestLineupGapText } from '@/lib/analysis/lineup';

export type LatestLineupGapProps = {
  map: string;
  /** Matches the view reads with every player because the default lineup never played the map. */
  fallback: number;
};

/** Says why a map the latest lineup never played shows every player's rounds. */
export function LatestLineupGap({ map, fallback }: LatestLineupGapProps): ReactNode {
  if (fallback === 0) return null;
  return (
    <p role="status" className="flex items-start gap-2 rounded-md border border-warning/45 bg-warning/10 px-3 py-2.5 text-body-sm text-fg-1">
      <Users aria-hidden className="mt-0.5 size-4 shrink-0" />
      {latestLineupGapText(map, fallback)}
    </p>
  );
}
