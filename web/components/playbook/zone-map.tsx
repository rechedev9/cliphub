'use client';

import type { ReactNode } from 'react';
import { mapSilhouette } from '@/lib/analysis/map-silhouette';
import { zoneShapes } from '@/lib/analysis/zone-shapes';
import { prettyMapName } from '@/lib/format';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { cn } from '@/lib/utils';

const SIZE = 100;

/**
 * The map's silhouette with the controlled zones on it; chosen zones in ink,
 * the rest faint. Nothing renders when the map has no analysed geometry.
 */
export function ZoneMap({
  teamId,
  map,
  zones,
  labels,
  selected,
}: {
  teamId: string;
  map: string;
  zones: readonly string[];
  labels: ReadonlyMap<string, string>;
  selected: readonly string[];
}): ReactNode {
  const geometry = useMapGeometry(teamId, [map]).get(map);
  if (geometry === undefined) return <div aria-hidden className="aspect-square w-full max-w-80 rounded-md bg-surface-3" />;
  const silhouette = geometry ? mapSilhouette(geometry, SIZE) : null;
  if (geometry === null || silhouette === null) return null;
  const shapes = zoneShapes(geometry, zones, SIZE);
  const [, , width = SIZE] = silhouette.viewBox.split(' ').map(Number);
  const font = Math.max(1.6, width / 32);
  return (
    <svg
      viewBox={silhouette.viewBox}
      role="img"
      aria-label={`Zonas controladas en ${prettyMapName(map)}`}
      className="aspect-square w-full max-w-80 rounded-md bg-surface-3"
    >
      <path d={silhouette.path} className="fill-fg-4/50" />
      {shapes.map((shape) => (
        <path key={`area-${shape.zone}`} d={shape.path} className={selected.includes(shape.zone) ? 'fill-primary/80' : 'fill-fg-3/40'} />
      ))}
      {shapes.map((shape) => (
        <text
          key={`label-${shape.zone}`}
          x={shape.x}
          y={shape.y}
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize={font}
          className={cn('[paint-order:stroke] stroke-surface-3', selected.includes(shape.zone) ? 'fill-fg-1 font-semibold' : 'fill-fg-2')}
          strokeWidth={font / 4}
        >
          {labels.get(shape.zone) ?? shape.zone}
        </text>
      ))}
    </svg>
  );
}
