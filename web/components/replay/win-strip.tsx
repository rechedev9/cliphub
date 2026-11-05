'use client';

import type { ReactNode } from 'react';
import { percentText } from '@/lib/metrics/advanced';
import { winStripPath } from '@/lib/replay/win-strip';
import type { WinStripPoint } from '@/lib/replay/win-strip';

const WIDTH = 1000;
const HEIGHT = 100;

export type WinStripState =
  | { kind: 'ready'; points: readonly WinStripPoint[]; now: number | null }
  | { kind: 'loading' }
  | { kind: 'none'; reason: string };

/**
 * The CT win probability under the timeline, on the same horizontal scale, so
 * each step sits under the kill, plant or defuse that moved it.
 */
export function WinStrip({ state }: { state: WinStripState }): ReactNode {
  if (state.kind === 'none') return <p className="text-meta text-fg-3">{state.reason}</p>;
  return (
    <div className="flex flex-col gap-1">
      <div className="h-10 px-[3px]">
        {state.kind === 'loading' ? (
          <div role="status" aria-label="Calculando la probabilidad de victoria" className="h-full rounded-sm bg-surface-2" />
        ) : (
          <svg
            viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
            preserveAspectRatio="none"
            className="h-full w-full overflow-visible"
            role="img"
            aria-label="Probabilidad de victoria del lado CT a lo largo de la ronda"
          >
            <line x1={0} x2={WIDTH} y1={HEIGHT / 2} y2={HEIGHT / 2} className="stroke-border" strokeDasharray="6 6" vectorEffect="non-scaling-stroke" />
            <path d={winStripPath(state.points, WIDTH, HEIGHT)} fill="none" className="stroke-primary" strokeWidth={2} vectorEffect="non-scaling-stroke" />
          </svg>
        )}
      </div>
      <p className="flex justify-between text-meta text-fg-3">
        <span>Probabilidad de victoria CT</span>
        <span className="tabular-nums text-fg-1">{state.kind === 'ready' && state.now !== null ? percentText(state.now) : ''}</span>
      </p>
    </div>
  );
}
