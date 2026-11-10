import type { ReactNode } from 'react';
import type { HeatmapKind } from '@/lib/api/spatial';
import { HEATMAP_ENDS_LABEL, HEATMAP_LEGEND_LABEL, HEAT_GRADIENT, heatAmount, secondsLabel } from '@/lib/tactical-spatial';

/**
 * The heat scale as the map paints it: blue where little happens, purple, then pink where the
 * most does, with ends in what the map counts. Timing runs reversed: pink is the earliest arrival.
 */
export function HeatLegend({
  kind,
  maxCount,
  maxSeconds,
  sampleSeconds,
}: {
  kind: HeatmapKind;
  maxCount: number;
  maxSeconds: number;
  sampleSeconds: number;
}): ReactNode {
  const timing = kind === 'timing';
  const ends = HEATMAP_ENDS_LABEL[kind];
  return (
    <div role="group" className="flex flex-col gap-1" aria-label={`Leyenda: ${HEATMAP_LEGEND_LABEL[kind]}`}>
      <span className="text-meta text-fg-2">{HEATMAP_LEGEND_LABEL[kind]}</span>
      <div className="h-2.5 rounded-sm" style={{ background: HEAT_GRADIENT }} aria-hidden />
      <div className="flex justify-between gap-2 text-meta tabular-nums text-fg-3">
        <span>{timing ? `${ends.low} (${secondsLabel(maxSeconds)})` : ends.low}</span>
        <span>{timing ? `${ends.high} (0 s)` : ends.high}</span>
      </div>
      {timing || maxCount <= 0 ? null : <span className="text-meta tabular-nums text-fg-2">Zona más activa: {heatAmount({ kind, count: maxCount, sampleSeconds })}</span>}
    </div>
  );
}
