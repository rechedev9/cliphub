'use client';

import type { ReactNode } from 'react';
import { mapSilhouette } from '@/lib/analysis/map-silhouette';
import { zoneShapes } from '@/lib/analysis/zone-shapes';
import { prettyMapName } from '@/lib/format';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { radarImageUrl } from '@/lib/api/game-assets';
import { useRadarImage } from '@/hooks/use-radar-image';
import { cn } from '@/lib/utils';

const SIZE = 100;

/**
 * CS2's radar (or, without the game, the map's silhouette) with the controlled
 * zones on it; chosen zones in ink, the rest faint. Nothing renders when the
 * map has no analysed geometry.
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
  const radar = useRadarImage(map);
  if (geometry === undefined) return <div aria-hidden className="aspect-square w-full max-w-[26rem] rounded-md bg-surface-3" />;
  const silhouette = geometry ? mapSilhouette(geometry, SIZE) : null;
  if (geometry === null || silhouette === null) return null;
  const shapes = zoneShapes(geometry, zones, SIZE);
  const imageWidth = radar ? (SIZE * radar.naturalWidth) / geometry.calibration.size : 0;
  const imageHeight = radar ? (SIZE * radar.naturalHeight) / geometry.calibration.size : 0;
  // The whole map like the game's radar; only the silhouette fallback is cropped to the played area.
  const viewBox = radar ? `0 0 ${imageWidth} ${imageHeight}` : silhouette.viewBox;
  const [, , width = SIZE] = viewBox.split(' ').map(Number);
  const font = Math.max(1.6, width / 32);
  return (
    <svg
      viewBox={viewBox}
      role="img"
      aria-label={`Zonas controladas en ${prettyMapName(map)}`}
      className="aspect-square w-full max-w-[26rem] rounded-md bg-surface-3"
    >
      {radar ? (
        <image
          href={radarImageUrl(map)}
          x={0}
          y={0}
          width={imageWidth}
          height={imageHeight}
          className="[filter:grayscale(0.55)_brightness(0.78)]"
        />
      ) : (
        <path d={silhouette.path} className="fill-fg-4/50" />
      )}
      {shapes.map((shape) => (
        <path key={`area-${shape.zone}`} d={shape.path} className={selected.includes(shape.zone) ? 'fill-primary/70' : 'fill-fg-3/25'} />
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
