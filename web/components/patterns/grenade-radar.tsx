'use client';

import { useCallback, useMemo, useRef, useState, type PointerEvent, type ReactNode } from 'react';
import type { TeamGrenade, TeamGrenadeKind } from '@/lib/api/grenades';
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry } from '@/lib/api/tactical';
import {
  dragWorldBox,
  grenadeId,
  grenadeTracer,
  nearestTracer,
  type GrenadeTracer,
  type WorldBox,
} from '@/lib/analysis/grenade-patterns';
import type { RadarPoint, WorldMark } from '@/lib/tactical-transform';
import { levelForAltitude, worldToRendered } from '@/lib/tactical-transform';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { cn } from '@/lib/utils';

/** One token per kind, shared by the tracers and the kind toggles. */
export const GRENADE_KIND_TOKEN: Record<TeamGrenadeKind, string> = {
  smoke: '--fg-2',
  flash: '--chart-1',
  he: '--chart-3',
  molotov: '--chart-2',
  decoy: '--chart-4',
};

/** A drag shorter than this many pixels is a click. */
const DRAG_MIN = 4;
const HOVER_REACH = 8;

/** A zone box and the floor it was drawn on. */
export type FloorBox = { box: WorldBox; level: RadarLevel };

type Props = {
  geometry: TacticalGeometry;
  /** The floors shown; two put them side by side, each grenade on the floor it lands on. */
  levels: readonly RadarLevel[];
  grenades: readonly TeamGrenade[];
  /** Grenade id to its spot key, for hover and highlight. */
  spotOf: ReadonlyMap<string, string>;
  /** Ids inside the box; null when no box is drawn. */
  selected: ReadonlySet<string> | null;
  highlight: string | null;
  box: FloorBox | null;
  onBox: (box: FloorBox | null) => void;
  onHover: (spotKey: string | null) => void;
};

function paintTracer(ctx: CanvasRenderingContext2D, tracer: GrenadeTracer, color: string, strong: boolean): void {
  ctx.strokeStyle = color;
  ctx.fillStyle = color;
  ctx.lineWidth = strong ? 2.5 : 1.25;
  if (tracer.from !== null) {
    ctx.beginPath();
    ctx.moveTo(tracer.from.x, tracer.from.y);
    ctx.lineTo(tracer.to.x, tracer.to.y);
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(tracer.from.x, tracer.from.y, strong ? 3 : 2, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.beginPath();
  ctx.arc(tracer.to.x, tracer.to.y, strong ? 4.5 : 3, 0, Math.PI * 2);
  ctx.fill();
}

/**
 * Every throw on the radar as a tracer from the thrower to the landing,
 * coloured by kind. Dragging draws a box that filters the list (Shift + drag
 * once zoomed, when a plain drag pans); hovering a tracer highlights its spot.
 */
export function GrenadeRadar({ geometry, levels, grenades, spotOf, selected, highlight, box, onBox, onHover }: Props): ReactNode {
  const split = levels.length > 1;
  const onPlate = useCallback(
    (level: RadarLevel) => (split ? grenades.filter((grenade) => levelForAltitude(geometry.calibration, grenade.landing[2]) === level) : grenades),
    [geometry.calibration, grenades, split],
  );
  const paint = useCallback(
    ({ ctx, size, level }: RadarFrame) => {
      const computed = getComputedStyle(ctx.canvas);
      const colors = new Map<TeamGrenadeKind, string>();
      const colorOf = (kind: TeamGrenadeKind): string => {
        const known = colors.get(kind);
        if (known !== undefined) return known;
        const value = computed.getPropertyValue(GRENADE_KIND_TOKEN[kind]).trim() || '#9aa4b2';
        colors.set(kind, value);
        return value;
      };
      const strong: { tracer: GrenadeTracer; kind: TeamGrenadeKind }[] = [];
      for (const grenade of onPlate(level)) {
        const tracer = grenadeTracer(geometry.calibration, size, grenade);
        if (tracer === null) continue;
        const id = grenadeId(grenade);
        if (highlight !== null && spotOf.get(id) === highlight) {
          strong.push({ tracer, kind: grenade.kind });
          continue;
        }
        let alpha = highlight === null ? 0.7 : 0.3;
        if (selected !== null && !selected.has(id)) alpha = 0.12;
        ctx.globalAlpha = alpha;
        paintTracer(ctx, tracer, colorOf(grenade.kind), false);
      }
      ctx.globalAlpha = 1;
      for (const entry of strong) paintTracer(ctx, entry.tracer, colorOf(entry.kind), true);
    },
    [geometry.calibration, highlight, onPlate, selected, spotOf],
  );
  const marks = useMemo(() => grenades.flatMap((grenade): WorldMark[] => (grenade.origin === null ? [grenade.landing] : [grenade.landing, grenade.origin])), [grenades]);

  return (
    <RadarSurface
      geometry={geometry}
      level={levels[0] ?? RADAR_LEVELS.default}
      levels={levels}
      paint={paint}
      marks={marks}
      label="Radar con las granadas del equipo"
    >
      {(projection, zoomed, level) => (
        <DragLayer
          geometry={geometry}
          projection={projection}
          zoomed={zoomed}
          grenades={onPlate(level)}
          spotOf={spotOf}
          box={box !== null && (!split || box.level === level) ? box.box : null}
          onBox={(next) => onBox(next === null ? null : { box: next, level })}
          onHover={onHover}
        />
      )}
    </RadarSurface>
  );
}

function DragLayer({
  geometry,
  projection,
  grenades,
  spotOf,
  box,
  onBox,
  onHover,
  zoomed,
}: Pick<Props, 'geometry' | 'grenades' | 'spotOf' | 'onHover'> & {
  projection: RadarProjection;
  zoomed: boolean;
  box: WorldBox | null;
  onBox: (box: WorldBox | null) => void;
}): ReactNode {
  const start = useRef<RadarPoint | null>(null);
  const [drag, setDrag] = useState<{ a: RadarPoint; b: RadarPoint } | null>(null);
  const { size, offsetX, offsetY } = projection;
  const tracers = useMemo(
    () =>
      grenades.flatMap((grenade) => {
        const tracer = grenadeTracer(geometry.calibration, size, grenade);
        return tracer === null ? [] : [{ tracer, id: grenadeId(grenade) }];
      }),
    [geometry.calibration, grenades, size],
  );

  const local = (event: PointerEvent<HTMLDivElement>): RadarPoint => {
    const rect = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - rect.left - offsetX, y: event.clientY - rect.top - offsetY };
  };

  const onPointerDown = (event: PointerEvent<HTMLDivElement>): void => {
    // Once zoomed, a plain drag belongs to the map's pan.
    if (event.button !== 0 || (zoomed && !event.shiftKey)) return;
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    start.current = local(event);
    onHover(null);
  };

  const onPointerMove = (event: PointerEvent<HTMLDivElement>): void => {
    const point = local(event);
    if (start.current !== null) {
      setDrag({ a: start.current, b: point });
      return;
    }
    const index = nearestTracer(
      tracers.map((entry) => entry.tracer),
      point,
      HOVER_REACH,
    );
    const hit = tracers[index];
    onHover(hit === undefined ? null : (spotOf.get(hit.id) ?? null));
  };

  const onPointerUp = (event: PointerEvent<HTMLDivElement>): void => {
    const from = start.current;
    start.current = null;
    setDrag(null);
    if (from === null) return;
    event.stopPropagation();
    const to = local(event);
    if (Math.hypot(to.x - from.x, to.y - from.y) < DRAG_MIN) onBox(null);
    else onBox(dragWorldBox(geometry.calibration, size, from, to));
  };

  let shown: { x: number; y: number; width: number; height: number } | null = null;
  if (drag !== null) {
    shown = {
      x: Math.min(drag.a.x, drag.b.x),
      y: Math.min(drag.a.y, drag.b.y),
      width: Math.abs(drag.a.x - drag.b.x),
      height: Math.abs(drag.a.y - drag.b.y),
    };
  } else if (box !== null) {
    const a = worldToRendered(geometry.calibration, box.minX, box.maxY, size);
    const b = worldToRendered(geometry.calibration, box.maxX, box.minY, size);
    shown = { x: a.x, y: a.y, width: b.x - a.x, height: b.y - a.y };
  }

  return (
    <div
      aria-hidden
      className={cn('absolute inset-0 touch-none select-none', zoomed ? '' : 'cursor-crosshair')}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={() => {
        start.current = null;
        setDrag(null);
      }}
      onPointerLeave={() => {
        if (start.current === null) onHover(null);
      }}
    >
      {shown === null ? null : (
        <div
          className="pointer-events-none absolute border border-dashed border-primary bg-primary/8"
          style={{ left: shown.x + offsetX, top: shown.y + offsetY, width: shown.width, height: shown.height }}
        />
      )}
    </div>
  );
}
