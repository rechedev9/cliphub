'use client';

import type { ReactNode } from 'react';
import { compareDelta, deltaArrow, type DeltaTone } from '@/lib/analysis/compare';
import type { Reliability } from '@/lib/analysis/reference';
import { EdgeTile, type EdgeTone } from '@/components/records/edge-tile';
import { DELTA_TONE_CLASS } from '@/components/ui/delta-value';
import { Sparkline } from '@/components/ui/sparkline';
import { cn } from '@/lib/utils';

const TONE: Record<DeltaTone, EdgeTone> = { better: 'good', worse: 'bad', even: 'neutral' };

function signOf(delta: number): string {
  if (delta > 0) return '+';
  return delta < 0 ? '-' : '';
}

export type PlayerMetricTileProps = {
  label: string;
  /** null when the scope has no data for it. */
  value: number | null;
  format: (value: number) => string;
  /** What the figure is compared with, such as the library mean; null leaves it uncompared. */
  mean: number | null;
  /** The name of that reference, read aloud with the delta. */
  meanLabel: string;
  higherIsBetter: boolean;
  epsilon?: number;
  /** The shrunk estimate: arrow and colour only when its interval leaves the reference out. */
  reliability: Reliability | null;
  /** One value per match, oldest first. */
  trend?: readonly number[];
};

/** One headline figure of the player as an EDGE tile: a coloured edge for better or worse, the delta to the reference under it. */
export function PlayerMetricTile({ label, value, format, mean, meanLabel, higherIsBetter, epsilon, reliability, trend }: PlayerMetricTileProps): ReactNode {
  const result = value !== null && mean !== null ? compareDelta(value, mean, { higherIsBetter, epsilon }) : null;
  // A difference the interval cannot tell from noise keeps its number but loses the arrow and the colour.
  const called = result !== null && result.tone !== 'even' && (reliability ? reliability.differs : true);
  let delta: ReactNode = null;
  if (result !== null && mean !== null) {
    const sign = signOf(result.delta);
    delta = (
      <span title={`${meanLabel}: ${format(mean)}.${reliability ? ` ${reliability.detail}` : ''}`}>
        <span className={cn('font-medium', called ? DELTA_TONE_CLASS[result.tone] : 'text-fg-2')}>
          {result.tone === 'even' ? '=' : `${called ? `${deltaArrow(result.delta, epsilon)} ` : ''}${sign}${format(Math.abs(result.delta)).replace(/^[+-]/, '')}`}
        </span>{' '}
        vs {format(mean)}
        <span className="sr-only">
          , {meanLabel.toLowerCase()}
          {reliability ? `. ${reliability.detail}` : ''}
        </span>
      </span>
    );
  }
  return (
    <EdgeTile
      label={label}
      tone={called && result !== null ? TONE[result.tone] : 'neutral'}
      value={value === null ? <span className="text-body-sm font-normal text-fg-3">Sin dato</span> : format(value)}
      note={delta}
      detail={
        reliability?.weak === true || trend ? (
          <>
            {reliability?.weak === true ? 'Poca muestra' : null}
            {trend ? <Sparkline values={trend} label={`${label} por partida`} format={format} width={64} height={18} /> : null}
          </>
        ) : null
      }
    />
  );
}
