'use client';

import { useMemo, type ReactNode } from 'react';
import { RADAR_LEVELS } from '@/lib/api/tactical';
import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import { zoneShapes } from '@/lib/analysis/zone-shapes';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { placeLabel } from '@/lib/tactical-labels';

/** The zones a finding's link marks, over the radar plate; the lower floor's are dashed. */
export function ZoneLayer({
  geometry,
  projection,
  zones,
  level,
}: {
  geometry: TacticalGeometry;
  projection: RadarProjection;
  zones: readonly string[];
  /** One floor's pane draws only that floor; one plate draws both. */
  level?: RadarLevel;
}): ReactNode {
  const shapes = useMemo(
    () =>
      (level === undefined ? [RADAR_LEVELS.default, RADAR_LEVELS.lower] : [level]).flatMap((floor) =>
        zoneShapes(geometry, { zones, size: projection.size, level: floor }).map((shape) => ({
          ...shape,
          lower: level === undefined && floor === RADAR_LEVELS.lower,
        })),
      ),
    [geometry, level, projection.size, zones],
  );
  if (shapes.length === 0) return null;
  return (
    <svg aria-hidden="true" className="pointer-events-none absolute inset-0 h-full w-full">
      <g transform={`translate(${projection.offsetX} ${projection.offsetY})`}>
        {shapes.map((shape) => (
          <g key={`${shape.zone}:${shape.lower ? 'lower' : 'default'}`}>
            <path
              d={shape.path}
              fill="var(--primary)"
              fillOpacity={0.18}
              stroke="var(--primary)"
              strokeWidth={1.5}
              strokeDasharray={shape.lower ? '5 4' : undefined}
            />
            <text
              x={shape.x}
              y={shape.y}
              textAnchor="middle"
              dominantBaseline="middle"
              className="fill-white text-[11px] font-semibold"
              paintOrder="stroke"
              stroke="black"
              strokeWidth={3}
            >
              {placeLabel(shape.zone)}
            </text>
          </g>
        ))}
      </g>
    </svg>
  );
}
