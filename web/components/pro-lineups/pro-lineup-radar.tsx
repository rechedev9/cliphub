'use client';

import { useCallback, type CSSProperties, type ReactNode } from 'react';
import type { ProLineupKind } from '@/lib/api/pro-lineups';
import type { RadarLevel, TacticalGeometry, TacticalSide } from '@/lib/api/tactical';
import { grenadeIcon } from '@/lib/analysis/icons';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { worldToRendered } from '@/lib/tactical-transform';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { cn } from '@/lib/utils';

/** A landing spot drawn as a grenade marker with how many lineups land there. */
export type RadarSpot = {
  id: string;
  kind: ProLineupKind;
  at: readonly [number, number];
  lineups: number;
  /** 0 to 1: how much it is thrown against the most thrown spot shown. */
  weight: number;
  label: string;
  state: 'normal' | 'active' | 'dim';
};

/** A throw drawn from where it is thrown to where it lands. */
export type RadarThrow = {
  id: string;
  kind: ProLineupKind;
  side: TacticalSide;
  from: readonly [number, number];
  to: readonly [number, number];
  label: string;
  /** Shown inside the throw point, such as a combo step. */
  number?: number;
  state: 'strong' | 'normal' | 'faint';
};

type Props = {
  geometry: TacticalGeometry;
  level: RadarLevel;
  spots?: readonly RadarSpot[];
  throws?: readonly RadarThrow[];
  onSpot?: (id: string) => void;
  onThrow?: (id: string) => void;
  onHoverThrow?: (id: string | null) => void;
  label: string;
};

function kindColor(kind: ProLineupKind): string {
  return `var(${GRENADE_KIND_TOKEN[kind]})`;
}

const LINE_ALPHA: Record<RadarThrow['state'], number> = { strong: 1, normal: 0.7, faint: 0.22 };

/**
 * The pro lineup radar: landing spots as grenade markers to pick, and the
 * throws of the open spot or combo as lines from the throw point to the
 * landing. Markers are buttons over the canvas so they take focus and hover.
 */
export function ProLineupRadar({ geometry, level, spots = [], throws = [], onSpot, onThrow, onHoverThrow, label }: Props): ReactNode {
  const paint = useCallback(
    ({ ctx, size }: RadarFrame) => {
      const computed = getComputedStyle(ctx.canvas);
      const colorOf = (kind: ProLineupKind): string => computed.getPropertyValue(GRENADE_KIND_TOKEN[kind]).trim() || '#9aa4b2';
      const scale = size / 1024;
      // Faint lines first so the strong one is never drawn under them.
      const order = [...throws].sort((a, b) => LINE_ALPHA[a.state] - LINE_ALPHA[b.state]);
      for (const shot of order) {
        const from = worldToRendered(geometry.calibration, shot.from[0], shot.from[1], size);
        const to = worldToRendered(geometry.calibration, shot.to[0], shot.to[1], size);
        const color = colorOf(shot.kind);
        ctx.globalAlpha = LINE_ALPHA[shot.state];
        ctx.strokeStyle = color;
        ctx.lineWidth = Math.max(1.25, (shot.state === 'strong' ? 3 : 1.75) * scale);
        ctx.setLineDash([7 * scale, 5 * scale]);
        ctx.beginPath();
        ctx.moveTo(from.x, from.y);
        ctx.lineTo(to.x, to.y);
        ctx.stroke();
        ctx.setLineDash([]);
        if (shot.state === 'normal') {
          ctx.globalAlpha = 1;
          ctx.beginPath();
          ctx.arc(to.x, to.y, Math.max(4, 7 * scale), 0, Math.PI * 2);
          ctx.fillStyle = color;
          ctx.fill();
        }
        if (shot.state === 'strong') {
          ctx.beginPath();
          ctx.arc(to.x, to.y, Math.max(8, 18 * scale), 0, Math.PI * 2);
          ctx.globalAlpha = 0.28;
          ctx.fillStyle = color;
          ctx.fill();
          ctx.globalAlpha = 1;
          ctx.stroke();
        }
      }
      ctx.globalAlpha = 1;
    },
    [geometry.calibration, throws],
  );

  return (
    <RadarSurface geometry={geometry} level={level} paint={paint} label={label}>
      {(projection) => (
        <>
          {/* Most thrown last, so it sits on top where points pile up. */}
          {[...throws].reverse().map((shot) => (
            <ThrowPoint key={shot.id} shot={shot} at={place(geometry, projection, shot.from)} onThrow={onThrow} onHover={onHoverThrow} />
          ))}
          {spots.map((spot) => (
            <SpotMarker key={spot.id} spot={spot} at={place(geometry, projection, spot.at)} onSpot={onSpot} />
          ))}
        </>
      )}
    </RadarSurface>
  );
}

function place(geometry: TacticalGeometry, projection: RadarProjection, world: readonly [number, number]): CSSProperties {
  const at = worldToRendered(geometry.calibration, world[0], world[1], projection.size);
  return { left: projection.offsetX + at.x, top: projection.offsetY + at.y };
}

function SpotMarker({ spot, at, onSpot }: { spot: RadarSpot; at: CSSProperties; onSpot?: (id: string) => void }): ReactNode {
  const size = Math.round(24 + 12 * Math.sqrt(spot.weight));
  return (
    <button
      type="button"
      aria-label={`${spot.label}: ${spot.lineups} ${spot.lineups === 1 ? 'lineup' : 'lineups'}`}
      aria-pressed={spot.state === 'active'}
      onClick={() => onSpot?.(spot.id)}
      className={cn(
        'group absolute flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border-2 bg-surface-0 outline-none',
        'transition-[opacity,transform] hover:z-20 hover:scale-110 focus-visible:z-20 focus-visible:ring-[3px] focus-visible:ring-ring/60',
        spot.state === 'active' && 'z-10 scale-110 ring-[3px] ring-fg-1',
        spot.state === 'dim' && 'opacity-45 hover:opacity-100',
      )}
      style={{ ...at, width: size, height: size, borderColor: kindColor(spot.kind), color: kindColor(spot.kind) }}
    >
      <EquipmentIcon name={grenadeIcon(spot.kind)} height={Math.round(size * 0.5)} />
      {spot.lineups > 1 ? (
        <span className="absolute -right-2 -bottom-1.5 min-w-5 rounded-full bg-fg-1 px-1 text-center text-meta leading-4 font-semibold text-surface-0 tabular-nums">
          {spot.lineups}
        </span>
      ) : null}
      <span
        aria-hidden
        className="pointer-events-none absolute bottom-full left-1/2 mb-2 hidden -translate-x-1/2 rounded-sm bg-fg-1 px-2 py-1 text-meta font-medium whitespace-nowrap text-surface-0 group-hover:block group-focus-visible:block"
      >
        {spot.label}
      </span>
    </button>
  );
}

function ThrowPoint({
  shot,
  at,
  onThrow,
  onHover,
}: {
  shot: RadarThrow;
  at: CSSProperties;
  onThrow?: (id: string) => void;
  onHover?: (id: string | null) => void;
}): ReactNode {
  const numbered = shot.number !== undefined;
  const className = cn(
    'group absolute flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border-2 border-surface-0 text-meta font-semibold text-surface-0 tabular-nums outline-none',
    numbered ? 'size-6' : 'size-4',
    shot.side === 'CT' ? 'bg-side-ct' : 'bg-side-t',
    shot.state === 'strong' && 'z-10 scale-125 ring-2 ring-fg-1',
    shot.state === 'faint' && 'opacity-50',
  );
  const style: CSSProperties = { ...at };
  if (onThrow === undefined) {
    return (
      <span aria-hidden className={className} style={style}>
        {shot.number}
      </span>
    );
  }
  return (
    <button
      type="button"
      aria-label={shot.number === undefined ? shot.label : `${shot.number}. ${shot.label}`}
      onClick={() => onThrow(shot.id)}
      onPointerEnter={() => onHover?.(shot.id)}
      onPointerLeave={() => onHover?.(null)}
      onFocus={() => onHover?.(shot.id)}
      onBlur={() => onHover?.(null)}
      className={cn(className, 'transition-transform hover:z-20 hover:scale-125 focus-visible:ring-[3px] focus-visible:ring-ring/60')}
      style={style}
    >
      {shot.number}
      <span
        aria-hidden
        className="pointer-events-none absolute bottom-full left-1/2 mb-2 hidden -translate-x-1/2 rounded-sm bg-fg-1 px-2 py-1 text-meta font-medium whitespace-nowrap text-surface-0 group-hover:block group-focus-visible:block"
      >
        {shot.label}
      </span>
    </button>
  );
}
