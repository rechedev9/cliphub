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
 * each step sits under the kill, plant or defuse that moved it. CT wins up, T down.
 */
export function WinStrip({ state }: { state: WinStripState }): ReactNode {
  if (state.kind === 'none') return <p className="h-5 truncate px-[3px] text-meta leading-5 text-fg-3">{state.reason}</p>;
  if (state.kind === 'loading') return <div role="status" aria-label="Calculando la probabilidad de victoria" className="mx-[3px] h-5 rounded-sm bg-surface-1" />;
  const path = winStripPath(state.points, WIDTH, HEIGHT);
  return (
    <div className="relative h-5 px-[3px]">
      <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} preserveAspectRatio="none" className="h-full w-full overflow-visible" role="img" aria-label="Probabilidad de victoria CT a lo largo de la ronda">
        <rect x={0} y={0} width={WIDTH} height={HEIGHT / 2} className="fill-side-ct/8" />
        <rect x={0} y={HEIGHT / 2} width={WIDTH} height={HEIGHT / 2} className="fill-side-t/8" />
        <path d={path} fill="none" className="stroke-fg-2" strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
      </svg>
      {state.now === null ? null : (
        <span className="pointer-events-none absolute top-0 right-1 rounded-sm bg-surface-0/85 px-1 text-meta leading-5 tabular-nums text-fg-2">
          Victoria CT <span className="text-fg-1">{percentText(state.now)}</span>
        </span>
      )}
    </div>
  );
}
