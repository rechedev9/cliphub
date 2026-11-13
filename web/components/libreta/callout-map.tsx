'use client';

import { useState, type ReactNode } from 'react';
import type { MapCallout } from '@/lib/api/callouts';
import { radarImageUrl } from '@/lib/api/game-assets';
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry } from '@/lib/api/tactical';
import { RADAR_PLATE_SIZE, radarPlateFrame, type RadarPlateFrame } from '@/lib/analysis/radar-plate';
import { prettyMapName } from '@/lib/format';
import { placeLabel } from '@/lib/tactical-labels';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, worldToRendered } from '@/lib/tactical-transform';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { useMapZoom } from '@/hooks/use-map-zoom';
import { useRadarImage } from '@/hooks/use-radar-image';
import { cn } from '@/lib/utils';

type Props = {
  map: string;
  geometry: TacticalGeometry;
  callouts: readonly MapCallout[];
  /** The place as the team says it. */
  label: (token: string) => string;
  onPick: (callout: MapCallout) => void;
};

/** The map's radar with a bubble on each CS2 place; a click on one asks for the team's name. */
export function CalloutMap(props: Props): ReactNode {
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const radar = useRadarImage(props.map, level === RADAR_LEVELS.lower ? 'lower' : 'upper');
  if (radar === undefined) return <Skeleton className="aspect-square w-full" />;
  const frame = radarPlateFrame(props.geometry, radar !== null);
  if (frame === null) return <p className="text-body-sm text-fg-2">No se pudo dibujar este mapa. Usa la lista para cambiar los nombres.</p>;
  return <CalloutPlate {...props} frame={frame} level={level} onLevel={setLevel} />;
}

type Placed = { callout: MapCallout; x: number; y: number; text: string };

function CalloutPlate({
  map,
  geometry,
  callouts,
  label,
  onPick,
  frame,
  level,
  onLevel,
}: Props & { frame: RadarPlateFrame; level: RadarLevel; onLevel: (level: RadarLevel) => void }): ReactNode {
  const zoom = useMapZoom(frame.base);
  const { view } = zoom;
  const scale = 1 / zoom.zoom;
  const font = Math.max(1.6, frame.width / 55) * scale;
  const multi = isMultiLevelMap(geometry.calibration);
  const onLevelOf = (callout: MapCallout): RadarLevel => callout.level ?? RADAR_LEVELS.default;
  const placed: Placed[] = callouts.flatMap((callout) => {
    if (callout.center === undefined || onLevelOf(callout) !== level) return [];
    const at = worldToRendered(geometry.calibration, callout.center[0], callout.center[1], RADAR_PLATE_SIZE);
    return [{ callout, x: at.x, y: at.y, text: label(callout.token) }];
  });
  const lower = callouts.filter((callout) => callout.center !== undefined && onLevelOf(callout) === RADAR_LEVELS.lower).length;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <ul aria-label="Leyenda" className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-2">
          <li className="inline-flex items-center gap-1.5">
            <span aria-hidden className="size-2.5 rounded-full bg-primary" />
            Con nombre del equipo
          </li>
          <li className="inline-flex items-center gap-1.5">
            <span aria-hidden className="size-2.5 rounded-full border border-border-strong bg-surface-1" />
            Con el nombre de CS2
          </li>
        </ul>
        <div className="flex items-center gap-2">
          {multi ? (
            <Segmented
              label="Piso"
              value={level}
              options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({
                value,
                label: value === RADAR_LEVELS.lower && lower > 0 ? `${LEVEL_LABEL[value]} (${lower})` : LEVEL_LABEL[value],
              }))}
              onChange={onLevel}
            />
          ) : null}
          <MapZoomControls zoom={zoom.zoom} zoomed={zoom.zoomed} onZoomBy={zoom.zoomBy} onReset={zoom.reset} />
        </div>
      </div>
      <div {...zoom.surface} className={cn('aspect-square w-full overflow-hidden rounded-md bg-surface-3', zoom.zoomed && 'touch-none', zoom.cursor)}>
        <svg viewBox={`${view.x} ${view.y} ${view.width} ${view.height}`} aria-label={`Callouts de ${prettyMapName(map)}`} className="size-full">
          {frame.image ? (
            <image
              href={radarImageUrl(map, level === RADAR_LEVELS.lower ? 'lower' : 'upper')}
              x={0}
              y={0}
              width={frame.width}
              height={frame.height}
              className="[filter:grayscale(0.55)_brightness(0.7)]"
            />
          ) : (
            <path d={frame.silhouette ?? ''} className="fill-fg-4/50" />
          )}
          {placed.map((entry) => (
            <Bubble key={entry.callout.token} entry={entry} font={font} scale={scale} onPick={onPick} />
          ))}
        </svg>
      </div>
      <p className="text-meta text-fg-3">Pulsa un callout para ponerle el nombre que usa tu equipo. La rueda acerca el mapa y arrastrar lo mueve.</p>
    </div>
  );
}

function Bubble({ entry, font, scale, onPick }: { entry: Placed; font: number; scale: number; onPick: (callout: MapCallout) => void }): ReactNode {
  const named = Boolean(entry.callout.name);
  const width = entry.text.length * font * 0.56 + font * 1.2;
  const height = font * 1.7;
  const cs2 = placeLabel(entry.callout.token);
  // Pressing a bubble is a click, never the start of a pan.
  return (
    <g
      role="button"
      tabIndex={0}
      aria-label={named ? `${entry.text} (${cs2} en CS2): cambiar el nombre` : `${cs2}: poner nombre`}
      onPointerDown={(event) => event.stopPropagation()}
      onClick={() => onPick(entry.callout)}
      onKeyDown={(event) => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        onPick(entry.callout);
      }}
      className="group cursor-pointer outline-none"
    >
      <title>{named ? `${entry.text} (en CS2, ${cs2})` : cs2}</title>
      <rect
        x={entry.x - width / 2}
        y={entry.y - height / 2}
        width={width}
        height={height}
        rx={height / 2}
        strokeWidth={0.3 * scale}
        className={cn(
          named ? 'fill-primary stroke-primary' : 'fill-surface-1 stroke-border-strong',
          'group-hover:stroke-fg-1 group-focus-visible:stroke-ring',
        )}
      />
      <text
        x={entry.x}
        y={entry.y}
        textAnchor="middle"
        dominantBaseline="central"
        fontSize={font}
        className={cn('pointer-events-none font-medium', named ? 'fill-primary-foreground' : 'fill-fg-1')}
      >
        {entry.text}
      </text>
    </g>
  );
}
