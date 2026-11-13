'use client';

import { useMemo, useState, type ReactNode } from 'react';
import type { TeamPrep } from '@/lib/api/playbook';
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry } from '@/lib/api/tactical';
import { radarImageUrl } from '@/lib/api/game-assets';
import { placeLabels, prepWindowLabel, spotLevel, windowMarkers, type LabelSide, type SpotMarker } from '@/lib/analysis/prep';
import { RADAR_PLATE_SIZE, radarPlateFrame, type RadarPlateFrame } from '@/lib/analysis/radar-plate';
import { prettyMapName } from '@/lib/format';
import { placeLabel } from '@/lib/tactical-labels';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, worldToRendered, type RadarRect } from '@/lib/tactical-transform';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { Segmented } from '@/components/ui/segmented';
import { useMapZoom } from '@/hooks/use-map-zoom';
import { useRadarImage } from '@/hooks/use-radar-image';
import { cn } from '@/lib/utils';

const SIZE = RADAR_PLATE_SIZE;
export const PLAYER_COLORS = ['--chart-1', '--chart-2', '--chart-3', '--chart-6', '--chart-7'];
/** Markers on one callout fan out on a ring this wide, in radar units. */
const FAN = 2.6;

export function playerColor(index: number): string {
  return `var(${PLAYER_COLORS[index % PLAYER_COLORS.length] ?? '--fg-1'})`;
}

type Placed = { marker: SpotMarker; x: number; y: number; side: LabelSide; text: string };

const LABEL_GAP = 1.9;
const ANCHOR: Readonly<Record<LabelSide, 'start' | 'end' | 'middle'>> = { right: 'start', left: 'end', above: 'middle', below: 'middle' };

/** Dots fanned out where they share a point, each label on a side that does not hit another label. */
function placeMarkers(markers: readonly SpotMarker[], geometry: TacticalGeometry, font: number, scale: number): Placed[] {
  const dots = markers.map((marker) => {
    const at = worldToRendered(geometry.calibration, marker.x, marker.y, SIZE);
    const angle = (2 * Math.PI * marker.slot) / marker.slots - Math.PI / 2;
    const text = `${Math.round(marker.pct)} %`;
    return {
      marker,
      text,
      x: marker.slots > 1 ? at.x + FAN * scale * Math.cos(angle) : at.x,
      y: marker.slots > 1 ? at.y + FAN * scale * Math.sin(angle) : at.y,
      width: text.length * font * 0.62,
      height: font,
    };
  });
  const sides = placeLabels(dots, LABEL_GAP * scale);
  return dots.map((dot, index) => ({ marker: dot.marker, x: dot.x, y: dot.y, text: dot.text, side: sides[index] ?? 'right' }));
}

function Marker({ placed, names, font, scale }: { placed: Placed; names: readonly string[]; font: number; scale: number }): ReactNode {
  const { marker, x, y, side } = placed;
  const gap = LABEL_GAP * scale;
  let dx = 0;
  if (side === 'right') dx = gap;
  else if (side === 'left') dx = -gap;
  let dy = 0;
  if (side === 'above') dy = -gap - font / 2;
  else if (side === 'below') dy = gap + font / 2;
  return (
    <g>
      <title>{`${names[marker.player] ?? ''}: ${placeLabel(marker.callout)}, ${Math.round(marker.pct)} % de sus rondas`}</title>
      <circle cx={x} cy={y} r={(marker.main ? 1.4 : 0.9) * scale} fill={playerColor(marker.player)} className="stroke-surface-0" strokeWidth={0.4 * scale} />
      <text
        x={x + dx}
        y={y + dy}
        textAnchor={ANCHOR[side]}
        dominantBaseline="middle"
        fontSize={marker.main ? font : font * 0.85}
        className="fill-fg-1 stroke-surface-0 font-medium [paint-order:stroke]"
        strokeWidth={font / 3.5}
      >
        {placed.text}
      </text>
    </g>
  );
}

/**
 * Each window of the round on the full radar, every CT player's usual spot as
 * a dot in their colour at where they stood, with the share of their rounds
 * spent there. The four radars zoom and pan together; a two-level map has a
 * floor picker, each spot drawn on its own floor's radar.
 */
export function CtPositionsMap({ prep }: { prep: TeamPrep }): ReactNode {
  const geometry = useMapGeometry(prep.team_id, [prep.map]).get(prep.map);
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const radar = useRadarImage(prep.map, level === RADAR_LEVELS.lower ? 'lower' : 'upper');
  if (geometry === undefined || radar === undefined) {
    return <div aria-hidden className="h-96 rounded-md bg-surface-3" />;
  }
  if (geometry === null) return null;
  const frame = radarPlateFrame(geometry, radar !== null);
  if (frame === null) return null;
  return <CtPositionsPlates prep={prep} geometry={geometry} frame={frame} level={level} onLevel={setLevel} />;
}

function CtPositionsPlates({
  prep,
  geometry,
  frame,
  level,
  onLevel,
}: {
  prep: TeamPrep;
  geometry: TacticalGeometry;
  frame: RadarPlateFrame;
  level: RadarLevel;
  onLevel: (level: RadarLevel) => void;
}): ReactNode {
  const base = useMemo<RadarRect>(() => {
    const [x = 0, y = 0, width = SIZE, height = SIZE] = frame.viewBox.split(' ').map(Number);
    return { x, y, width, height };
  }, [frame.viewBox]);
  const zoom = useMapZoom(base);
  const { view } = zoom;
  const scale = 1 / zoom.zoom;
  const centers = new Map(geometry.callouts.map((callout) => [callout.name, callout.center] as const));
  const names = prep.ct_players.map((player) => player.name);
  const windows = prep.ct_players[0]?.windows ?? [];
  const font = Math.max(2, frame.width / 42) * scale;
  const multi = isMultiLevelMap(geometry.calibration);
  const lowerSpots = prep.ct_players.reduce(
    (sum, player) => sum + player.windows.reduce((count, window) => count + window.spots.filter((spot) => spotLevel(spot) === 'lower').length, 0),
    0,
  );
  const markerLevel = level === RADAR_LEVELS.lower ? 'lower' : 'default';

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <ul aria-label="Jugadores" className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-2">
          {names.map((name, index) => (
            <li key={prep.ct_players[index]?.steamid64 ?? name} className="inline-flex items-center gap-1.5">
              <span aria-hidden className="size-2 rounded-full" style={{ backgroundColor: playerColor(index) }} />
              {name}
            </li>
          ))}
        </ul>
        <div className="flex items-center gap-2 print:hidden">
          {multi ? (
            <Segmented
              label="Piso"
              value={level}
              options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({
                value,
                label: value === RADAR_LEVELS.lower && lowerSpots > 0 ? `${LEVEL_LABEL[value]} (${lowerSpots})` : LEVEL_LABEL[value],
              }))}
              onChange={onLevel}
            />
          ) : null}
          <MapZoomControls zoom={zoom.zoom} zoomed={zoom.zoomed} onZoomBy={zoom.zoomBy} onReset={zoom.reset} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2 @[120rem]/content:grid-cols-4 print:grid-cols-4">
        {windows.map((window, index) => {
          const placed = placeMarkers(windowMarkers(prep.ct_players, { window: index, centers, level: markerLevel }), geometry, font, scale);
          return (
            <figure key={window.from_seconds} className="flex min-w-0 flex-col gap-1">
              <figcaption className="text-meta font-medium text-fg-2">{prepWindowLabel(window)}</figcaption>
              <div {...zoom.surface} className={cn('aspect-square w-full overflow-hidden rounded-md bg-surface-3', zoom.zoomed && 'touch-none', zoom.cursor)}>
                <svg
                  viewBox={`${view.x} ${view.y} ${view.width} ${view.height}`}
                  role="img"
                  aria-label={`Posiciones CT de ${prep.team_name} en ${prettyMapName(prep.map)}, ${prepWindowLabel(window)}`}
                  className="size-full"
                >
                  {frame.image ? (
                    <image
                      href={radarImageUrl(prep.map, level === RADAR_LEVELS.lower ? 'lower' : 'upper')}
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
                    <Marker key={`${entry.marker.player}-${entry.marker.callout}`} placed={entry} names={names} font={font} scale={scale} />
                  ))}
                </svg>
              </div>
            </figure>
          );
        })}
      </div>
    </div>
  );
}
