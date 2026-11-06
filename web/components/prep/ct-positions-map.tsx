'use client';

import type { ReactNode } from 'react';
import type { TeamPrep } from '@/lib/api/playbook';
import type { TacticalGeometry } from '@/lib/api/tactical';
import { radarImageUrl } from '@/lib/api/game-assets';
import { mapSilhouette } from '@/lib/analysis/map-silhouette';
import { placeLabels, prepWindowLabel, windowMarkers, type LabelSide, type SpotMarker } from '@/lib/analysis/prep';
import { prettyMapName } from '@/lib/format';
import { placeLabel } from '@/lib/tactical-labels';
import { worldToRendered } from '@/lib/tactical-transform';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { useRadarImage } from '@/hooks/use-radar-image';

const SIZE = 100;
export const PLAYER_COLORS = ['--chart-1', '--chart-2', '--chart-3', '--chart-6', '--chart-7'];
/** Markers on one callout fan out on a ring this wide, in radar units. */
const FAN = 2.6;

export function playerColor(index: number): string {
  return `var(${PLAYER_COLORS[index % PLAYER_COLORS.length] ?? '--fg-1'})`;
}

type Frame = { viewBox: string; width: number; height: number; image: boolean; silhouette: string | null };

function frameOf(geometry: TacticalGeometry, radar: HTMLImageElement | null): Frame | null {
  if (radar) {
    const width = (SIZE * radar.naturalWidth) / geometry.calibration.size;
    const height = (SIZE * radar.naturalHeight) / geometry.calibration.size;
    return { viewBox: `0 0 ${width} ${height}`, width, height, image: true, silhouette: null };
  }
  const silhouette = mapSilhouette(geometry, SIZE);
  if (silhouette === null) return null;
  const [, , width = SIZE, height = SIZE] = silhouette.viewBox.split(' ').map(Number);
  return { viewBox: silhouette.viewBox, width, height, image: false, silhouette: silhouette.path };
}

type Placed = { marker: SpotMarker; x: number; y: number; side: LabelSide; text: string };

const LABEL_GAP = 1.9;
const ANCHOR: Readonly<Record<LabelSide, 'start' | 'end' | 'middle'>> = { right: 'start', left: 'end', above: 'middle', below: 'middle' };

/** Dots fanned out on shared callouts, each label on a side that does not hit another label. */
function placeMarkers(markers: readonly SpotMarker[], geometry: TacticalGeometry, font: number): Placed[] {
  const dots = markers.map((marker) => {
    const at = worldToRendered(geometry.calibration, marker.x, marker.y, SIZE);
    const angle = (2 * Math.PI * marker.slot) / marker.slots - Math.PI / 2;
    const text = `${Math.round(marker.pct)} %`;
    return {
      marker,
      text,
      x: marker.slots > 1 ? at.x + FAN * Math.cos(angle) : at.x,
      y: marker.slots > 1 ? at.y + FAN * Math.sin(angle) : at.y,
      width: text.length * font * 0.62,
      height: font,
    };
  });
  const sides = placeLabels(dots, LABEL_GAP);
  return dots.map((dot, index) => ({ marker: dot.marker, x: dot.x, y: dot.y, text: dot.text, side: sides[index] ?? 'right' }));
}

function Marker({ placed, names, font }: { placed: Placed; names: readonly string[]; font: number }): ReactNode {
  const { marker, x, y, side } = placed;
  let dx = 0;
  if (side === 'right') dx = LABEL_GAP;
  else if (side === 'left') dx = -LABEL_GAP;
  let dy = 0;
  if (side === 'above') dy = -LABEL_GAP - font / 2;
  else if (side === 'below') dy = LABEL_GAP + font / 2;
  return (
    <g>
      <title>{`${names[marker.player] ?? ''}: ${placeLabel(marker.callout)}, ${Math.round(marker.pct)} % de sus rondas`}</title>
      <circle cx={x} cy={y} r={marker.main ? 1.4 : 0.9} fill={playerColor(marker.player)} className="stroke-surface-0" strokeWidth={0.4} />
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
 * a dot in their colour with the share of their rounds spent there.
 */
export function CtPositionsMap({ prep }: { prep: TeamPrep }): ReactNode {
  const geometry = useMapGeometry(prep.team_id, [prep.map]).get(prep.map);
  const radar = useRadarImage(prep.map);
  if (geometry === undefined || radar === undefined) {
    return <div aria-hidden className="h-96 rounded-md bg-surface-3" />;
  }
  if (geometry === null) return null;
  const frame = frameOf(geometry, radar);
  if (frame === null) return null;
  const centers = new Map(geometry.callouts.map((callout) => [callout.name, callout.center] as const));
  const names = prep.ct_players.map((player) => player.name);
  const windows = prep.ct_players[0]?.windows ?? [];
  const font = Math.max(2, frame.width / 42);

  return (
    <div className="flex flex-col gap-2">
      <ul aria-label="Jugadores" className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-2">
        {names.map((name, index) => (
          <li key={prep.ct_players[index]?.steamid64 ?? name} className="inline-flex items-center gap-1.5">
            <span aria-hidden className="size-2 rounded-full" style={{ backgroundColor: playerColor(index) }} />
            {name}
          </li>
        ))}
      </ul>
      <div className="grid grid-cols-2 gap-2 @[120rem]/content:grid-cols-4 print:grid-cols-4">
        {windows.map((window, index) => {
          const placed = placeMarkers(windowMarkers(prep.ct_players, { window: index, centers }), geometry, font);
          return (
            <figure key={window.from_seconds} className="flex min-w-0 flex-col gap-1">
              <figcaption className="text-meta font-medium text-fg-2">{prepWindowLabel(window)}</figcaption>
              <svg
                viewBox={frame.viewBox}
                role="img"
                aria-label={`Posiciones CT de ${prep.team_name} en ${prettyMapName(prep.map)}, ${prepWindowLabel(window)}`}
                className="aspect-square w-full rounded-md bg-surface-3"
              >
                {frame.image ? (
                  <image
                    href={radarImageUrl(prep.map)}
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
                  <Marker key={`${entry.marker.player}-${entry.marker.callout}`} placed={entry} names={names} font={font} />
                ))}
              </svg>
            </figure>
          );
        })}
      </div>
    </div>
  );
}
