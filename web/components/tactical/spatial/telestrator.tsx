'use client';

import { useCallback, useEffect, useRef, useState, type PointerEvent, type ReactNode } from 'react';
import { ArrowUpRight, Brush, Eraser, Undo2 } from 'lucide-react';
import { ANNOTATION_STROKES_MAX, STROKE_COLORS, STROKE_POINTS_MAX } from '@/lib/api/spatial';
import type { Stroke, StrokeColor } from '@/lib/api/spatial';
import { RADAR_LEVELS } from '@/lib/api/tactical';
import type { RadarCalibration, RadarLevel } from '@/lib/api/tactical';
import { worldToRendered } from '@/lib/tactical-transform';
import { arrowHead, renderedToWorld, strokeAt, strokePath, thinStroke } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** The colour each stroke resolves to. The radar plate is dark in both themes, so the
 * accent pen is a fixed near white rather than the theme's ink colour. */
export const STROKE_VAR: Record<StrokeColor, string> = {
  accent: 'oklch(0.96 0.002 264)',
  ct: 'var(--side-ct)',
  t: 'var(--side-t)',
  mark: 'var(--primary)',
};

export const STROKE_LABEL: Record<StrokeColor, string> = {
  accent: 'Blanco',
  ct: 'Color CT',
  t: 'Color T',
  mark: 'Violeta',
};

/** Stroke widths are radar pixels at the 1024 reference, scaled with the plate. */
const STROKE_WIDTH = 3;
const REFERENCE_SIZE = 1024;
/** World units a freehand point must move before it is kept. */
const THIN_WORLD_UNITS = 12;
/** How close to a stroke the eraser has to pass, in screen pixels. */
const ERASER_REACH_PX = 8;

/**
 * `erase`: the pen takes off the stroke it passes over instead of drawing.
 * `timed`: each stroke fades out on its own and stays out of the note, for explaining live.
 */
export type TelestratorTool = { color: StrokeColor; arrow: boolean; erase: boolean; timed: boolean };

/** How long a timed stroke stays on the map; it fades out over the last second. */
export const TIMED_STROKE_MS = 4000;
const TIMED_FADE_MS = 1000;

/** The unsaved drawing of a note: its strokes, the active tool and whether the pen is down. */
export type TelestratorDraft = {
  drawing: boolean;
  setDrawing: (drawing: boolean) => void;
  tool: TelestratorTool;
  setTool: (tool: TelestratorTool) => void;
  strokes: Stroke[];
  /** Strokes drawn with `timed`, until each fades out. */
  fleeting: readonly { id: number; stroke: Stroke }[];
  addStroke: (stroke: Stroke) => void;
  removeStroke: (index: number) => void;
  undo: () => void;
  clear: () => void;
};

export function useTelestratorDraft(): TelestratorDraft {
  const [drawing, setDrawing] = useState(false);
  const [tool, setTool] = useState<TelestratorTool>({ color: 'accent', arrow: false, erase: false, timed: false });
  const [strokes, setStrokes] = useState<Stroke[]>([]);
  const [fleeting, setFleeting] = useState<{ id: number; stroke: Stroke }[]>([]);
  const nextFleeting = useRef(0);
  const timers = useRef(new Set<number>());
  useEffect(() => {
    const pending = timers.current;
    return () => pending.forEach((timer) => window.clearTimeout(timer));
  }, []);
  const timed = tool.timed;
  const addStroke = useCallback(
    (stroke: Stroke) => {
      if (!timed) {
        setStrokes((current) => (current.length >= ANNOTATION_STROKES_MAX ? current : [...current, stroke]));
        return;
      }
      nextFleeting.current += 1;
      const id = nextFleeting.current;
      setFleeting((current) => [...current, { id, stroke }]);
      const timer = window.setTimeout(() => {
        timers.current.delete(timer);
        setFleeting((current) => current.filter((entry) => entry.id !== id));
      }, TIMED_STROKE_MS);
      timers.current.add(timer);
    },
    [timed],
  );
  const removeStroke = useCallback((index: number) => setStrokes((current) => current.filter((_, at) => at !== index)), []);
  const undo = useCallback(() => setStrokes((current) => current.slice(0, -1)), []);
  const clear = useCallback(() => {
    setStrokes([]);
    setFleeting([]);
  }, []);
  return { drawing, setDrawing, tool, setTool, strokes, fleeting, addStroke, removeStroke, undo, clear };
}

/** What a press on the map does: move it, draw, draw an arrow, or erase a stroke. */
export type DrawMode = 'move' | 'draw' | 'arrow' | 'erase';

export function drawMode(draft: TelestratorDraft): DrawMode {
  if (!draft.drawing) return 'move';
  if (draft.tool.erase) return 'erase';
  return draft.tool.arrow ? 'arrow' : 'draw';
}

export function chooseDrawMode(draft: TelestratorDraft, mode: DrawMode): void {
  draft.setDrawing(mode !== 'move');
  if (mode !== 'move') draft.setTool({ ...draft.tool, arrow: mode === 'arrow', erase: mode === 'erase' });
}

/** A stroke's on-screen width for a plate of this size. */
export function strokeWidthPx(stroke: Stroke, projection: RadarProjection): number {
  return Math.max(1.5, stroke.width * (projection.size / REFERENCE_SIZE));
}

function StrokeShape({
  stroke,
  calibration,
  projection,
  faded,
  fading = false,
}: {
  stroke: Stroke;
  calibration: RadarCalibration;
  projection: RadarProjection;
  faded?: boolean;
  /** A timed stroke: it fades out by itself before it is removed. */
  fading?: boolean;
}): ReactNode {
  const points = stroke.points.map(([x, y]) => {
    const p = worldToRendered(calibration, x, y, projection.size);
    return { x: p.x + projection.offsetX, y: p.y + projection.offsetY };
  });
  if (points.length < 2) return null;
  const width = strokeWidthPx(stroke, projection);
  const color = STROKE_VAR[stroke.color];
  return (
    <g opacity={faded ? 0.55 : 1}>
      {fading ? <animate attributeName="opacity" from="1" to="0" begin={`${(TIMED_STROKE_MS - TIMED_FADE_MS) / 1000}s`} dur={`${TIMED_FADE_MS / 1000}s`} fill="freeze" /> : null}
      <path d={strokePath(points)} fill="none" stroke={color} strokeWidth={width} strokeLinecap="round" strokeLinejoin="round" />
      {stroke.arrow ? (
        <polygon points={arrowHead(points[points.length - 2], points[points.length - 1], width * 4)} fill={color} />
      ) : null}
    </g>
  );
}

/** Whether a stroke belongs on a floor's pane; a stroke drawn with one floor on screen sits on the upper pane. */
export function onLevel(stroke: Stroke, level: RadarLevel | undefined): boolean {
  return level === undefined || (stroke.level ?? RADAR_LEVELS.default) === level;
}

/**
 * The drawing layer over a radar plate: saved drawings, the draft being made,
 * and, while the pen is down, the pointer capture that turns a drag into a
 * stroke in world units. Arrows are straight; free strokes are thinned.
 * With `level` it is one floor's pane: it shows that floor's strokes and tags new ones with it.
 */
export function TelestratorLayer({
  calibration,
  projection,
  shown,
  draft,
  level,
}: {
  calibration: RadarCalibration;
  projection: RadarProjection;
  /** Saved drawings to show, already filtered to the moment on screen. */
  shown: readonly { id: string; strokes: readonly Stroke[]; faded?: boolean }[];
  draft?: TelestratorDraft;
  level?: RadarLevel;
}): ReactNode {
  const live = useRef<[number, number][] | null>(null);
  const erasing = useRef(false);
  const [preview, setPreview] = useState<Stroke | null>(null);
  const drawing = draft?.drawing ?? false;
  const eraser = drawing && draft?.tool.erase === true;

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
    const stroke: Stroke = { color: draft.tool.color, width: STROKE_WIDTH, arrow: draft.tool.arrow, points: kept.slice(0, STROKE_POINTS_MAX) };
    return level === undefined ? stroke : { ...stroke, level };
  };

  const eraseAt = (event: PointerEvent<SVGSVGElement>): void => {
    if (draft === undefined) return;
    const reach = ERASER_REACH_PX * (calibration.size / projection.size) * calibration.scale;
    const index = strokeAt(draft.strokes, toWorld(event), reach);
    if (index !== null) draft.removeStroke(index);
  };

  const onPointerDown = (event: PointerEvent<SVGSVGElement>): void => {
    if (!drawing) return;
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    if (eraser) {
      erasing.current = true;
      eraseAt(event);
      return;
    }
    live.current = [toWorld(event)];
  };
  const onPointerMove = (event: PointerEvent<SVGSVGElement>): void => {
    if (erasing.current) eraseAt(event);
    if (live.current === null) return;
    live.current.push(toWorld(event));
    setPreview(strokeFrom(live.current));
  };
  const onPointerUp = (event: PointerEvent<SVGSVGElement>): void => {
    erasing.current = false;
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
      className={cn('absolute inset-0 h-full w-full', drawing ? 'pointer-events-auto cursor-crosshair touch-none' : 'pointer-events-none')}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={() => {
        live.current = null;
        erasing.current = false;
        setPreview(null);
      }}
      onClick={(event) => {
        if (drawing) event.stopPropagation();
      }}
    >
      {shown.map((note) =>
        note.strokes.map((stroke, i) =>
          onLevel(stroke, level) ? (
            <StrokeShape key={`${note.id}:${i}`} stroke={stroke} calibration={calibration} projection={projection} faded={note.faded} />
          ) : null,
        ),
      )}
      {draft?.strokes.map((stroke, i) =>
        onLevel(stroke, level) ? <StrokeShape key={`draft:${i}`} stroke={stroke} calibration={calibration} projection={projection} /> : null,
      )}
      {draft?.fleeting.map((entry) =>
        onLevel(entry.stroke, level) ? <StrokeShape key={`timed:${entry.id}`} stroke={entry.stroke} calibration={calibration} projection={projection} fading /> : null,
      )}
      {preview ? <StrokeShape stroke={preview} calibration={calibration} projection={projection} /> : null}
    </svg>
  );
}

/** Pen, colour, arrow and undo for the draft of a note. */
export function TelestratorToolbar({ draft }: { draft: TelestratorDraft }): ReactNode {
  const { drawing, setDrawing, tool, setTool, strokes, undo, clear } = draft;
  return (
    <div className="flex flex-wrap items-center gap-1.5" role="toolbar" aria-label="Dibujo sobre el radar">
      <Button
        type="button"
        size="xs"
        variant={drawing ? 'outline' : 'ghost'}
        aria-pressed={drawing}
        onClick={() => {
          setTool({ ...tool, erase: false });
          setDrawing(!drawing);
        }}
      >
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
        onClick={() => setTool({ ...tool, arrow: !tool.arrow, erase: false })}
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
