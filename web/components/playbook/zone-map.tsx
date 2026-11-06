'use client';

import { useMemo, type ReactNode } from 'react';
import { mapSilhouette } from '@/lib/analysis/map-silhouette';
import type { TacticalGeometry } from '@/lib/api/tactical';
import type { RadarRect } from '@/lib/tactical-transform';
import { zoneShapes } from '@/lib/analysis/zone-shapes';
import { prettyMapName } from '@/lib/format';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { radarImageUrl } from '@/lib/api/game-assets';
import { useRadarImage } from '@/hooks/use-radar-image';
import { useMapZoom } from '@/hooks/use-map-zoom';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
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
  if (geometry === null) return null;
  return <ZoneMapPlate geometry={geometry} radar={radar ?? null} map={map} zones={zones} labels={labels} selected={selected} />;
}

function ZoneMapPlate({
  geometry,
  radar,
  map,
  zones,
  labels,
  selected,
}: {
  geometry: TacticalGeometry;
  radar: HTMLImageElement | null;
  map: string;
  zones: readonly string[];
  labels: ReadonlyMap<string, string>;
  selected: readonly string[];
}): ReactNode {
  const silhouette = mapSilhouette(geometry, SIZE);
  const imageWidth = radar ? (SIZE * radar.naturalWidth) / geometry.calibration.size : 0;
  const imageHeight = radar ? (SIZE * radar.naturalHeight) / geometry.calibration.size : 0;
  // The whole map like the game's radar; only the silhouette fallback is cropped to the played area.
  const viewBox = radar ? `0 0 ${imageWidth} ${imageHeight}` : (silhouette?.viewBox ?? `0 0 ${SIZE} ${SIZE}`);
  const base = useMemo<RadarRect>(() => {
    const [x = 0, y = 0, width = SIZE, height = SIZE] = viewBox.split(' ').map(Number);
    return { x, y, width, height };
  }, [viewBox]);
  const zoom = useMapZoom(base);
  if (silhouette === null) return null;
  const shapes = zoneShapes(geometry, zones, SIZE);
  const { view } = zoom;
  // Labels keep their screen size as the map zooms.
  const font = Math.max(1.6, base.width / 32) / zoom.zoom;
  return (
    <div {...zoom.surface} className={cn('relative aspect-square w-full max-w-[26rem] overflow-hidden rounded-md bg-surface-3', zoom.zoomed && 'touch-none', zoom.cursor)}>
      <svg
        viewBox={`${view.x} ${view.y} ${view.width} ${view.height}`}
        role="img"
        aria-label={`Zonas controladas en ${prettyMapName(map)}`}
        className="size-full"
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
      <MapZoomControls zoom={zoom.zoom} zoomed={zoom.zoomed} onZoomBy={zoom.zoomBy} onReset={zoom.reset} className="absolute right-2 bottom-2" />
    </div>
  );
}
