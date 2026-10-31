'use client';

import { useCallback, useRef, useState, type PointerEvent, type ReactNode } from 'react';
import { ArrowUpRight, Brush, Eraser, Undo2 } from 'lucide-react';
import { ANNOTATION_STROKES_MAX, STROKE_COLORS, STROKE_POINTS_MAX } from '@/lib/api/spatial';
import type { Stroke, StrokeColor } from '@/lib/api/spatial';
import type { RadarCalibration } from '@/lib/api/tactical';
import { worldToRendered } from '@/lib/tactical-transform';
import { arrowHead, renderedToWorld, strokePath, thinStroke } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** The colour each stroke resolves to. The radar plate is dark in both themes, so the
 * accent pen is a fixed near white rather than the theme's ink colour. */
const STROKE_VAR: Record<StrokeColor, string> = {
  accent: 'oklch(0.96 0.002 264)',
  ct: 'var(--side-ct)',
  t: 'var(--side-t)',
  mark: 'var(--destructive)',
};

const STROKE_LABEL: Record<StrokeColor, string> = {
  accent: 'Blanco',
  ct: 'Color CT',
  t: 'Color T',
  mark: 'Rojo',
};

/** Stroke widths are radar pixels at the 1024 reference, scaled with the plate. */
const STROKE_WIDTH = 3;
const REFERENCE_SIZE = 1024;
/** World units a freehand point must move before it is kept. */
const THIN_WORLD_UNITS = 12;

export type TelestratorTool = { color: StrokeColor; arrow: boolean };

/** The unsaved drawing of a note: its strokes, the active tool and whether the pen is down. */
export type TelestratorDraft = {
  drawing: boolean;
  setDrawing: (drawing: boolean) => void;
  tool: TelestratorTool;
  setTool: (tool: TelestratorTool) => void;
  strokes: Stroke[];
  addStroke: (stroke: Stroke) => void;
  undo: () => void;
  clear: () => void;
};

export function useTelestratorDraft(): TelestratorDraft {
  const [drawing, setDrawing] = useState(false);
  const [tool, setTool] = useState<TelestratorTool>({ color: 'accent', arrow: false });
  const [strokes, setStrokes] = useState<Stroke[]>([]);
  const addStroke = useCallback((stroke: Stroke) => {
    setStrokes((current) => (current.length >= ANNOTATION_STROKES_MAX ? current : [...current, stroke]));
  }, []);
  const undo = useCallback(() => setStrokes((current) => current.slice(0, -1)), []);
  const clear = useCallback(() => setStrokes([]), []);
  return { drawing, setDrawing, tool, setTool, strokes, addStroke, undo, clear };
}

function StrokeShape({
  stroke,
  calibration,
  projection,
  faded,
}: {
  stroke: Stroke;
  calibration: RadarCalibration;
  projection: RadarProjection;
  faded?: boolean;
}): ReactNode {
  const scale = projection.size / REFERENCE_SIZE;
  const points = stroke.points.map(([x, y]) => {
    const p = worldToRendered(calibration, x, y, projection.size);
    return { x: p.x + projection.offsetX, y: p.y + projection.offsetY };
  });
  if (points.length < 2) return null;
  const width = Math.max(1.5, stroke.width * scale);
  const color = STROKE_VAR[stroke.color];
  return (
    <g opacity={faded ? 0.55 : 1}>
      <path d={strokePath(points)} fill="none" stroke={color} strokeWidth={width} strokeLinecap="round" strokeLinejoin="round" />
      {stroke.arrow ? (
        <polygon points={arrowHead(points[points.length - 2], points[points.length - 1], width * 4)} fill={color} />
      ) : null}
    </g>
  );
}

/**
 * The drawing layer over a radar plate: saved drawings, the draft being made,
 * and, while the pen is down, the pointer capture that turns a drag into a
 * stroke in world units. Arrows are straight; free strokes are thinned.
 */
export function TelestratorLayer({
  calibration,
  projection,
  shown,
  draft,
}: {
  calibration: RadarCalibration;
  projection: RadarProjection;
  /** Saved drawings to show, already filtered to the moment on screen. */
  shown: readonly { id: string; strokes: readonly Stroke[]; faded?: boolean }[];
  draft?: TelestratorDraft;
}): ReactNode {
  const live = useRef<[number, number][] | null>(null);
  const [preview, setPreview] = useState<Stroke | null>(null);
  const drawing = draft?.drawing ?? false;

  const toWorld = (event: PointerEvent<SVGSVGElement>): [number, number] => {
    const box = event.currentTarget.getBoundingClientRect();
    const world = renderedToWorld(
      calibration,
      event.clientX - box.left - projection.offsetX,
      event.clientY - box.top - projection.offsetY,
      projection.size,
    );
    return [world.x, world.y];
  };

  const strokeFrom = (points: [number, number][]): Stroke | null => {
    if (draft === undefined || points.length < 2) return null;
    const kept = draft.tool.arrow ? [points[0], points[points.length - 1]] : thinStroke(points, THIN_WORLD_UNITS);
    if (kept.length < 2) return null;
    return { color: draft.tool.color, width: STROKE_WIDTH, arrow: draft.tool.arrow, points: kept.slice(0, STROKE_POINTS_MAX) };
  };

  const onPointerDown = (event: PointerEvent<SVGSVGElement>): void => {
    if (!drawing) return;
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    live.current = [toWorld(event)];
  };
  const onPointerMove = (event: PointerEvent<SVGSVGElement>): void => {
    if (live.current === null) return;
    live.current.push(toWorld(event));
    setPreview(strokeFrom(live.current));
  };
  const onPointerUp = (event: PointerEvent<SVGSVGElement>): void => {
    if (live.current === null) return;
    event.stopPropagation();
    const stroke = strokeFrom(live.current);
    live.current = null;
    setPreview(null);
    if (stroke !== null) draft?.addStroke(stroke);
  };

  return (
    <svg
      aria-hidden="true"
      className={cn('absolute inset-0 h-full w-full', drawing ? 'cursor-crosshair touch-none' : 'pointer-events-none')}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={() => {
        live.current = null;
        setPreview(null);
      }}
      onClick={(event) => {
        if (drawing) event.stopPropagation();
      }}
    >
      {shown.map((note) =>
        note.strokes.map((stroke, i) => (
          <StrokeShape key={`${note.id}:${i}`} stroke={stroke} calibration={calibration} projection={projection} faded={note.faded} />
        )),
      )}
      {draft?.strokes.map((stroke, i) => (
        <StrokeShape key={`draft:${i}`} stroke={stroke} calibration={calibration} projection={projection} />
      ))}
      {preview ? <StrokeShape stroke={preview} calibration={calibration} projection={projection} /> : null}
    </svg>
  );
}

/** Pen, colour, arrow and undo for the draft of a note. */
export function TelestratorToolbar({ draft }: { draft: TelestratorDraft }): ReactNode {
  const { drawing, setDrawing, tool, setTool, strokes, undo, clear } = draft;
  return (
    <div className="flex flex-wrap items-center gap-1.5" role="toolbar" aria-label="Dibujo sobre el radar">
      <Button type="button" size="xs" variant={drawing ? 'outline' : 'ghost'} aria-pressed={drawing} onClick={() => setDrawing(!drawing)}>
        <Brush aria-hidden />
        {drawing ? 'Dibujando' : 'Dibujar'}
      </Button>
      <div className="flex items-center gap-1" role="group" aria-label="Color del trazo">
        {STROKE_COLORS.map((color) => (
          <button
            key={color}
            type="button"
            aria-label={STROKE_LABEL[color]}
            aria-pressed={tool.color === color}
            onClick={() => setTool({ ...tool, color })}
            className={cn(
              'size-6 rounded-full border-2 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
              tool.color === color ? 'border-fg-1' : 'border-transparent',
            )}
            style={{ backgroundColor: STROKE_VAR[color] }}
          />
        ))}
      </div>
      <Button
        type="button"
        size="xs"
        variant={tool.arrow ? 'outline' : 'ghost'}
        aria-pressed={tool.arrow}
        onClick={() => setTool({ ...tool, arrow: !tool.arrow })}
      >
        <ArrowUpRight aria-hidden />
        Flecha
      </Button>
      <Button type="button" size="xs" variant="ghost" disabled={strokes.length === 0} onClick={undo}>
        <Undo2 aria-hidden />
        Deshacer
      </Button>
      <Button type="button" size="xs" variant="ghost" disabled={strokes.length === 0} onClick={clear}>
        <Eraser aria-hidden />
        Borrar
      </Button>
    </div>
  );
}
