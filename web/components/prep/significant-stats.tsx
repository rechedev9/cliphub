import type { ReactNode } from 'react';
import type { SignificantStat } from '@/lib/analysis/library-pool';
import { cn } from '@/lib/utils';

/** The prep's headline: the numbers that stand out against the library, worse in red and better in green. */
export function SignificantStats({ stats, loading, note }: { stats: readonly SignificantStat[]; loading: boolean; note: string }): ReactNode {
  let body: ReactNode;
  if (loading) body = <p className="text-body-sm text-fg-3">Comparando con la biblioteca.</p>;
  else if (stats.length === 0) body = <p className="text-body-sm text-fg-2">Nada se aleja de la media de la biblioteca lo bastante para destacarlo.</p>;
  else {
    body = (
      <ul className="grid gap-x-6 gap-y-1 @[72rem]/content:grid-cols-2 print:grid-cols-2">
        {stats.map((stat) => (
          <li key={stat.id} className="flex items-baseline gap-2 text-body-sm">
            <span aria-hidden className={cn('text-[0.75em]', stat.tone === 'worse' ? 'text-outcome-loss' : 'text-outcome-win')}>
              {stat.tone === 'worse' ? '▼' : '▲'}
            </span>
            <span className={stat.tone === 'worse' ? 'text-outcome-loss' : 'text-fg-1'}>{stat.text}</span>
          </li>
        ))}
      </ul>
    );
  }
  return (
    <div className="studio-panel flex flex-col gap-1.5 px-3 py-2.5">
      {body}
      <p className="text-meta text-fg-3">{note}</p>
    </div>
  );
}
