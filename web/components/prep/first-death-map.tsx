'use client';

import type { ReactNode } from 'react';
import type { FirstDeathZone } from '@/lib/api/first-death';
import { radarImageUrl } from '@/lib/api/game-assets';
import { RADAR_PLATE_SIZE, radarPlateFrame, type RadarPlateFrame } from '@/lib/analysis/radar-plate';
import { placeLabel } from '@/lib/tactical-labels';
import { worldToRendered } from '@/lib/tactical-transform';
import type { TacticalGeometry } from '@/lib/api/tactical';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { useMapZoom } from '@/hooks/use-map-zoom';
import { useRadarImage } from '@/hooks/use-radar-image';
import { cn } from '@/lib/utils';

export type FirstDeathMapProps = {
  teamId: string;
  map: string;
  zone: FirstDeathZone;
  /** The player's colour, the same as on their card. */
  color: string;
  label: string;
};

/** One player's samples in one zone on the radar of the zone's floor, zoomable like the other prep maps. */
export function FirstDeathMap({ teamId, map, zone, color, label }: FirstDeathMapProps): ReactNode {
  const geometry = useMapGeometry(teamId, [map]).get(map);
  const layer = zone.floor === 'lower' ? 'lower' : 'upper';
  const radar = useRadarImage(map, layer);
  if (geometry === undefined || radar === undefined) return <div aria-hidden className="aspect-square w-full max-w-xl rounded-md bg-surface-3" />;
  if (geometry === null) return null;
  const frame = radarPlateFrame(geometry, radar !== null);
  if (frame === null) return null;
  return <Plate geometry={geometry} frame={frame} zone={zone} color={color} label={label} image={radarImageUrl(map, layer)} />;
}

function Plate({ geometry, frame, zone, color, label, image }: {
  geometry: TacticalGeometry;
  frame: RadarPlateFrame;
  zone: FirstDeathZone;
  color: string;
  label: string;
  image: string;
}): ReactNode {
  const zoom = useMapZoom(frame.base);
  const { view } = zoom;
  const r = (Math.max(frame.width, frame.height) / 260) / zoom.zoom;
  return (
    <div className="flex w-full max-w-xl flex-col gap-1.5">
      <div className="flex justify-end print:hidden">
        <MapZoomControls zoom={zoom.zoom} zoomed={zoom.zoomed} onZoomBy={zoom.zoomBy} onReset={zoom.reset} />
      </div>
      <div {...zoom.surface} className={cn('aspect-square w-full overflow-hidden rounded-md bg-surface-3', zoom.zoomed && 'touch-none', zoom.cursor)}>
        <svg viewBox={`${view.x} ${view.y} ${view.width} ${view.height}`} role="img" aria-label={`${label}, ${placeLabel(zone.callout)}`} className="size-full">
          {frame.image ? (
            <image href={image} x={0} y={0} width={frame.width} height={frame.height} className="[filter:grayscale(0.55)_brightness(0.7)]" />
          ) : (
            <path d={frame.silhouette ?? ''} className="fill-fg-4/50" />
          )}
          {zone.samples.map(([x, y], index) => {
            const at = worldToRendered(geometry.calibration, x, y, RADAR_PLATE_SIZE);
            return <circle key={index} cx={at.x} cy={at.y} r={r} fill={color} fillOpacity={0.55} />;
          })}
        </svg>
      </div>
    </div>
  );
}
