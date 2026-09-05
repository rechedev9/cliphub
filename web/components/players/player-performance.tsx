import type { ReactNode } from 'react';
import type { FaceitMatch } from '@/lib/api/faceit';
import { summarizeFaceitMatches } from '@/lib/faceit-stats';
import { cn } from '@/lib/utils';
import { Progress } from '@/components/ui/progress';

export function PlayerPerformance({ matches }: { matches: FaceitMatch[] }): ReactNode {
  const stats = summarizeFaceitMatches(matches);
  return (
    <section aria-labelledby="player-performance-heading">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 text-body-sm">
        <h3 id="player-performance-heading" className="font-semibold text-fg-1">Rendimiento</h3>
        <p className="text-fg-3">Últimas {matches.length} partidas</p>
      </div>
      <dl className="grid grid-cols-2 gap-3 @[48rem]/content:grid-cols-4">
        <Metric label="Victorias" value={stats.winRate === undefined ? '—' : `${stats.winRate}%`} positive={stats.winRate !== undefined && stats.winRate >= 50}>
          <p className="text-meta tracking-normal text-fg-2">{stats.wins} victorias · {stats.losses} derrotas</p>
          {stats.winRate !== undefined ? <Progress value={stats.winRate} tone="success" size="xs" aria-label="Porcentaje de victorias" className="mt-2" /> : null}
        </Metric>
        <Metric label="K/D" value={stats.kd?.toFixed(2) ?? '—'} positive={stats.kd !== undefined && stats.kd >= 1} />
        <Metric label="ADR" value={stats.adr === undefined ? '—' : String(Math.round(stats.adr))} positive={stats.adr !== undefined && stats.adr >= 80} />
        <Metric label="Headshots" value={stats.headshots === undefined ? '—' : `${Math.round(stats.headshots)}%`} />
      </dl>
      {stats.unknown > 0 ? <p className="mt-2 text-meta tracking-normal text-fg-3">{stats.unknown} partidas sin resultado disponible, excluidas del porcentaje de victorias.</p> : null}
    </section>
  );
}

function Metric({ label, value, positive, children }: { label: string; value: string; positive?: boolean; children?: ReactNode }): ReactNode {
  return (
    <div className="studio-panel min-w-0 p-3.5">
      <dt className="text-body-sm text-fg-3">{label}</dt>
      <dd className={cn('mt-1 text-stat font-semibold tabular-nums', positive ? 'text-success' : 'text-fg-1')}>{value}</dd>
      {children ? <dd className="mt-1">{children}</dd> : null}
    </div>
  );
}
