'use client';

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type MouseEvent,
  type ReactNode,
  type RefObject,
} from 'react';
import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import { isCalibrationUsable, radarViewRect } from '@/lib/tactical-transform';
import type { RadarRect, WorldMark } from '@/lib/tactical-transform';
import { LEVEL_LABEL, radarProjection, renderedToWorld } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { cn } from '@/lib/utils';
import { useFloorZoom } from '@/hooks/use-floor-zoom';
import type { FloorZoom } from '@/hooks/use-floor-zoom';
import { floorRectsOf, navFloorRects } from '@/lib/replay/floor-split';
import type { NavRadarModel } from '@/lib/replay/nav-radar';
import { loadNavRadar } from '@/components/replay/radar/map-art';
import { bakeMapPlate } from '@/components/replay/radar/radar-bake';
import type { MapArt } from '@/components/replay/radar/radar-bake';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';

/** What a paint callback draws with: the context already panned to the radar square of `level`. */
export type RadarFrame = {
  ctx: CanvasRenderingContext2D;
  size: number;
  view: RadarRect;
  style: RadarStyle;
  level: RadarLevel;
};

/** Lets the owner repaint the marks without React rendering, from an animation loop. */
export type RadarSurfaceHandle = { repaint: () => void };

const NO_MARKS: readonly WorldMark[] = [];

/** Same height ceiling as the round replay, so the two views share one plate size. */
const RADAR_MAX_HEIGHT_VH = 74;
const GRID: MapArt = { kind: 'grid' };

/**
 * A radar plate for the multi-round views: the replay's own flat plate from the
 * nav mesh (the play-derived cells only while it loads or when it is missing),
 * baked once per size, with the owner's marks painted over it. The canvas is
 * `aria-hidden`; each view lists the same facts as text beside it. `children`
 * receives the projection, for layers such as the telestrator that sit on top,
 * and whether the map is zoomed, when a plain drag pans instead.
 * With two `levels` (Nuke, Vertigo) both floors sit side by side (stacked in a
 * narrow box), the upper one first, each cropped to its own extent on one scale
 * (`lib/replay/floor-split`, the replay's arrangement) and on one zoom.
 */
export function RadarSurface({
  geometry,
  level,
  levels,
  paint,
  handleRef,
  onPoint,
  label,
  marks = NO_MARKS,
  children,
}: {
  geometry: TacticalGeometry;
  level: RadarLevel;
  /** Both floors at once; one entry, or none, shows only `level`. */
  levels?: readonly RadarLevel[];
  paint: (frame: RadarFrame) => void;
  /** World positions `paint` and `children` draw at; the crop grows to keep each in reach. Memoize it. */
  marks?: readonly WorldMark[];
  handleRef?: RefObject<RadarSurfaceHandle | null>;
  /** A click on a plate, in world units, with the floor it was on. */
  onPoint?: (world: { x: number; y: number }, level: RadarLevel) => void;
  label: string;
  children?: (projection: RadarProjection, zoomed: boolean, level: RadarLevel) => ReactNode;
}): ReactNode {
  // Primitives, so a caller's fresh `levels` array does not look like a new framing and reset the zoom.
  const split = levels !== undefined && levels.length > 1;
  const firstLevel = split ? levels[0] : level;
  const secondLevel = split ? levels[1] : undefined;
  const shown = useMemo<readonly RadarLevel[]>(() => (secondLevel === undefined ? [firstLevel] : [firstLevel, secondLevel]), [firstLevel, secondLevel]);
  const repaints = useRef(new Map<RadarLevel, () => void>());
  const drawable = isCalibrationUsable(geometry.calibration) && geometry.cell_size > 0;
  // The replay's flat plate from the nav mesh, so a coach sees one map everywhere; nothing is drawn until it
  // answers, so the cells never flash before it, and the map takes no zoom meanwhile.
  const map = geometry.map;
  const [navArt, setNavArt] = useState<{ map: string; model: NavRadarModel | null } | null>(null);
  useEffect(() => {
    let active = true;
    void loadNavRadar(map).then((model) => {
      if (active) setNavArt({ map, model });
    });
    return () => {
      active = false;
    };
  }, [map]);
  const settled = navArt?.map === map;
  const model = settled ? navArt.model : null;
  const art = useMemo<MapArt>(() => (model === null ? GRID : { kind: 'nav', model }), [model]);
  // Each floor is cropped to its walkable ground and where people played on it, with the marks: the replay's layout.
  const whole = useMemo(() => radarViewRect(geometry, marks), [geometry, marks]);
  const floorRects = useMemo(() => floorRectsOf(geometry, marks), [geometry, marks]);
  const navRects = useMemo(() => (model === null || !drawable ? null : navFloorRects({ geometry, model, marks })), [drawable, geometry, marks, model]);
  const rects = useMemo(() => shown.map((plateLevel) => navRects?.[plateLevel] ?? (split ? floorRects[plateLevel] : whole)), [floorRects, navRects, shown, split, whole]);
  const zoom = useFloorZoom({ enabled: settled });
  const { reset } = zoom;
  // A different map, or different marks that move the crop, is a different framing: the whole map again.
  // Compared by value, so a refreshed answer with the same crop (a dragged time window) keeps the zoom.
  const framing = rects.map((rect) => `${rect.x.toFixed(4)},${rect.y.toFixed(4)},${rect.width.toFixed(4)},${rect.height.toFixed(4)}`).join('|');
  useEffect(() => reset(), [reset, framing]);

  useEffect(() => {
    if (handleRef === undefined) return;
    const plates = repaints.current;
    handleRef.current = { repaint: () => plates.forEach((repaint) => repaint()) };
    return () => {
      handleRef.current = null;
    };
  }, [handleRef]);

  // Both floors at one scale: a plate is as wide as its floor's extent, so the row's widths follow them.
  const widest = Math.max(...rects.map((rect) => rect.width));
  const tallest = Math.max(...rects.map((rect) => rect.height));
  const rowWidth = rects.reduce((sum, rect) => sum + rect.width, 0);
  const plateVars: CSSProperties & { '--radar-max': string } = {
    '--radar-max': split ? `min(100%, calc(${((RADAR_MAX_HEIGHT_VH * rowWidth) / tallest).toFixed(2)}vh + 1rem))` : `min(100%, ${((RADAR_MAX_HEIGHT_VH * rects[0].width) / rects[0].height).toFixed(2)}vh)`,
  };

  return (
    <div className={cn('mx-auto w-full max-w-(--radar-max)', split && '@container')} style={plateVars}>
      <div className={cn(split && 'flex flex-col items-center gap-2 @[32rem]:flex-row @[32rem]:gap-4')}>
        {shown.map((plateLevel, index) => (
          <div
            key={plateLevel}
            className={cn('w-full', split && 'w-(--share) @[32rem]:w-auto @[32rem]:min-w-0 @[32rem]:grow-(--grow) @[32rem]:basis-0')}
            style={split ? sharesOf(rects[index], widest) : undefined}
          >
            <RadarPlate
              geometry={geometry}
              level={plateLevel}
              art={art}
              settled={settled}
              drawable={drawable}
              rect={rects[index]}
              zoom={zoom}
              paint={paint}
              repaints={repaints.current}
              onPoint={onPoint}
              label={split ? `${label} ${LEVEL_LABEL[plateLevel]}.` : label}
              controls={index === shown.length - 1}
            >
              {children}
            </RadarPlate>
          </div>
        ))}
      </div>
    </div>
  );
}

/** A plate's share of the widest one in a narrow box, and its weight in a row: both follow its floor's width. */
function sharesOf(rect: RadarRect, widest: number): CSSProperties & { '--share': string; '--grow': number } {
  return { '--share': `${((100 * rect.width) / widest).toFixed(2)}%`, '--grow': rect.width };
}

/** One floor's canvas: its own bitmap and size, the zoom shared with the other floor. */
function RadarPlate({
  geometry,
  level,
  art,
  settled,
  drawable,
  rect,
  zoom,
  paint,
  repaints,
  onPoint,
  label,
  controls,
  children,
}: {
  geometry: TacticalGeometry;
  level: RadarLevel;
  art: MapArt;
  settled: boolean;
  drawable: boolean;
  /** The floor's extent, as a window of the radar square. */
  rect: RadarRect;
  zoom: FloorZoom;
  paint: (frame: RadarFrame) => void;
  repaints: Map<RadarLevel, () => void>;
  onPoint?: (world: { x: number; y: number }, level: RadarLevel) => void;
  label: string;
  controls: boolean;
  children?: (projection: RadarProjection, zoomed: boolean, level: RadarLevel) => ReactNode;
}): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const backgroundRef = useRef<{ key: string; canvas: HTMLCanvasElement } | null>(null);
  const styleRef = useRef<RadarStyle | null>(null);
  const paintRef = useRef(paint);
  const repaintRef = useRef<() => void>(() => {});
  const [box, setBox] = useState({ width: 1, height: 1 });
  const [dpr, setDpr] = useState(1);
  const view = zoom.windowOf(rect);
  const { surfaceFor } = zoom;
  const surface = useMemo(() => surfaceFor(rect), [surfaceFor, rect]);
  const projection = useMemo<RadarProjection>(() => radarProjection(box, view), [box, view]);
  const bindZoom = surface.ref;
  const bindContainer = useCallback(
    (element: HTMLDivElement | null) => {
      containerRef.current = element;
      return bindZoom(element);
    },
    [bindZoom],
  );

  const repaint = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (canvas === null || container === null || !drawable) return;
    const ctx = canvas.getContext('2d', { alpha: false });
    if (ctx === null) return;
    const style = styleRef.current ?? readRadarStyle(canvas);
    styleRef.current = style;
    const { size, offsetX, offsetY } = projection;
    if (!settled) {
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.fillStyle = style.background;
      ctx.fillRect(0, 0, canvas.width / dpr, canvas.height / dpr);
      return;
    }
    const key = `${size}|${dpr}|${level}|${geometry.map}|${view.x},${view.y},${view.width},${view.height}|${style.map}|${art.kind}`;
    if (backgroundRef.current?.key !== key) {
      backgroundRef.current = { key, canvas: bakeMapPlate({ art, geometry, level, size, dpr, view, style }) };
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = style.background;
    ctx.fillRect(0, 0, canvas.width / dpr, canvas.height / dpr);
    ctx.translate(offsetX, offsetY);
    ctx.drawImage(backgroundRef.current.canvas, view.x * size, view.y * size, view.width * size, view.height * size);
    paintRef.current({ ctx, size, view, style, level });
    ctx.setTransform(1, 0, 0, 1, 0, 0);
  }, [art, drawable, dpr, geometry, level, projection, settled, view]);

  useEffect(() => {
    paintRef.current = paint;
    repaintRef.current = repaint;
    repaint();
  }, [paint, repaint]);

  useEffect(() => {
    repaints.set(level, repaint);
    return () => {
      if (repaints.get(level) === repaint) repaints.delete(level);
    };
  }, [repaints, level, repaint]);

  useEffect(() => {
    const container = containerRef.current;
    const canvas = canvasRef.current;
    if (container === null || canvas === null) return;
    const measure = (): void => {
      const box = container.getBoundingClientRect();
      const ratio = window.devicePixelRatio > 0 ? window.devicePixelRatio : 1;
      const width = Math.round(Math.max(1, box.width) * ratio);
      const height = Math.round(Math.max(1, box.height) * ratio);
      if (canvas.width !== width) canvas.width = width;
      if (canvas.height !== height) canvas.height = height;
      styleRef.current = readRadarStyle(canvas);
      setDpr(ratio);
      setBox((current) => (current.width === box.width && current.height === box.height ? current : { width: box.width, height: box.height }));
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    window.addEventListener('resize', measure);
    // A theme switch changes the tokens without resizing anything.
    const themeObserver = new MutationObserver(() => {
      styleRef.current = readRadarStyle(canvas);
      backgroundRef.current = null;
      measure();
      repaintRef.current();
    });
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    return () => {
      observer.disconnect();
      themeObserver.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, []);

  useEffect(() => {
    repaint();
  }, [repaint]);

  const onClick = (event: MouseEvent<HTMLDivElement>): void => {
    if (onPoint === undefined || !drawable) return;
    const rect = event.currentTarget.getBoundingClientRect();
    const x = event.clientX - rect.left - projection.offsetX;
    const y = event.clientY - rect.top - projection.offsetY;
    onPoint(renderedToWorld(geometry.calibration, x, y, projection.size), level);
  };

  return (
    <div
      {...surface}
      ref={bindContainer}
      role="img"
      aria-label={label}
      onClick={onClick}
      className={cn(
        'relative w-full overflow-hidden rounded-lg border border-border-strong bg-surface-0 shadow-[inset_0_1px_0_0_var(--edge-shade),inset_0_-1px_0_0_var(--edge-light)]',
        zoom.zoomed && 'touch-none',
        zoom.cursor,
      )}
      style={{ aspectRatio: `${rect.width} / ${rect.height}` }}
    >
      <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
      {drawable ? (
        <>
          {children?.(projection, zoom.zoomed, level)}
          {controls ? (
            <MapZoomControls
              zoom={zoom.zoom}
              zoomed={zoom.zoomed}
              onZoomBy={zoom.zoomBy}
              onReset={zoom.reset}
              className="absolute right-2 bottom-2"
            />
          ) : null}
        </>
      ) : (
        <p className="absolute inset-0 grid place-items-center px-6 text-center text-body-sm leading-5 text-fg-2">
          Este mapa no tiene una calibración de radar utilizable, así que no se puede dibujar.
        </p>
      )}
    </div>
  );
}
