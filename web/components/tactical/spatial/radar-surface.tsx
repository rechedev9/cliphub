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
import { floorRectsOf, mapFloorRects, mapRect } from '@/lib/replay/floor-split';
import { boxWindow } from '@/lib/replay/map-view';
import type { NavRadarModel } from '@/lib/replay/nav-radar';
import { loadMapPlate } from '@/components/replay/radar/map-art';
import { bakeMapPlate } from '@/components/replay/radar/radar-bake';
import type { MapArt } from '@/components/replay/radar/radar-bake';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { MapFooter } from '@/components/replay/map-footer';

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
 * A radar plate for the multi-round views: the replay's own radar image (the
 * play-derived cells only while it loads or when it is missing), framed by the nav mesh,
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
  viewer,
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
  /** Draws the map in the Replay's viewer: its box, header and footer, at the size the Replay draws it. */
  viewer?: { title: string };
  children?: (projection: RadarProjection, zoomed: boolean, level: RadarLevel) => ReactNode;
}): ReactNode {
  // Primitives, so a caller's fresh `levels` array does not look like a new framing and reset the zoom.
  const split = levels !== undefined && levels.length > 1;
  const firstLevel = split ? levels[0] : level;
  const secondLevel = split ? levels[1] : undefined;
  const shown = useMemo<readonly RadarLevel[]>(() => (secondLevel === undefined ? [firstLevel] : [firstLevel, secondLevel]), [firstLevel, secondLevel]);
  const repaints = useRef(new Map<RadarLevel, () => void>());
  const drawable = isCalibrationUsable(geometry.calibration) && geometry.cell_size > 0;
  // The replay's radar, so a coach sees one map everywhere; nothing is drawn until it
  // answers, so the cells never flash before it, and the map takes no zoom meanwhile.
  const map = geometry.map;
  const twoFloors = geometry.calibration.lower_altitude_max !== undefined;
  const [plateArt, setPlateArt] = useState<{ map: string; model: NavRadarModel | null; art: MapArt } | null>(null);
  useEffect(() => {
    let active = true;
    void loadMapPlate(map, twoFloors).then((loaded) => {
      if (active) setPlateArt({ map, ...loaded });
    });
    return () => {
      active = false;
    };
  }, [map, twoFloors]);
  const settled = plateArt?.map === map;
  const model = settled ? plateArt.model : null;
  const art = settled ? plateArt.art : GRID;
  // Cropped to the map alone, as the Replay crops it: the whole map on one plate, each floor on two.
  // Only without the nav mesh do the marks shape it.
  const rects = useMemo(() => {
    if (model !== null && drawable && shown.length === 1) return [mapRect(geometry, model)];
    const mapRects = model === null || !drawable ? null : mapFloorRects(geometry, model);
    if (mapRects !== null) return shown.map((plateLevel) => mapRects[plateLevel]);
    if (!split) return shown.map(() => radarViewRect(geometry, marks));
    const floorRects = floorRectsOf(geometry, marks);
    return shown.map((plateLevel) => floorRects[plateLevel]);
  }, [drawable, geometry, marks, model, shown, split]);
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

  // Both floors at one scale: a plate is as wide and as tall as its floor's extent, relative to the row.
  const widest = Math.max(...rects.map((rect) => rect.width));
  const tallest = Math.max(...rects.map((rect) => rect.height));
  if (viewer !== undefined) {
    return (
      <div className="flex h-(--map-viewer-h) min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-border bg-surface-0">
        <header className="flex h-9 shrink-0 items-center gap-3 border-b border-border-subtle bg-surface-1 px-3">
          <h2 className="truncate font-display text-body font-bold text-fg-1">{viewer.title}</h2>
        </header>
        <div className="flex min-h-0 flex-1 gap-2">
          {shown.map((plateLevel, index) => (
            <div key={plateLevel} className="min-w-0 basis-0 self-center" style={{ flexGrow: shown.length > 1 ? rects[index].width : 1, height: `${(100 * rects[index].height) / tallest}%` }}>
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
                controls={false}
                framed={false}
              >
                {children}
              </RadarPlate>
            </div>
          ))}
        </div>
        <MapFooter demo={null} zoom={<MapZoomControls zoom={zoom.zoom} zoomed={zoom.zoomed} onZoomBy={zoom.zoomBy} onReset={zoom.reset} className="border-0 bg-transparent p-0" />} />
      </div>
    );
  }

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
  framed = true,
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
  /** Its own bordered box sized to the floor; inside a viewer it fills the viewer's box instead. */
  framed?: boolean;
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
    // The plate covers the whole box, not just the view, as the Replay's does.
    const plate = boxWindow(box, projection);
    const key = `${size}|${dpr}|${level}|${geometry.map}|${plate.x},${plate.y},${plate.width},${plate.height}|${style.map}|${art.kind}`;
    if (backgroundRef.current?.key !== key) {
      backgroundRef.current = { key, canvas: bakeMapPlate({ art, geometry, level, size, dpr, view: plate, style }) };
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = style.background;
    ctx.fillRect(0, 0, canvas.width / dpr, canvas.height / dpr);
    ctx.translate(offsetX, offsetY);
    ctx.drawImage(backgroundRef.current.canvas, plate.x * size, plate.y * size, plate.width * size, plate.height * size);
    paintRef.current({ ctx, size, view, style, level });
    ctx.setTransform(1, 0, 0, 1, 0, 0);
  }, [art, box, drawable, dpr, geometry, level, projection, settled, view]);

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
    // A GPU process reset hands the canvas back blank, and the baked background with it.
    const onRestored = (): void => {
      backgroundRef.current = null;
      repaintRef.current();
    };
    canvas.addEventListener('contextrestored', onRestored);
    return () => {
      canvas.removeEventListener('contextrestored', onRestored);
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
        'relative w-full overflow-hidden bg-surface-0',
        framed ? 'rounded-lg border border-border-strong shadow-[inset_0_1px_0_0_var(--edge-shade),inset_0_-1px_0_0_var(--edge-light)]' : 'h-full',
        zoom.zoomed && 'touch-none',
        zoom.cursor,
      )}
      style={framed ? { aspectRatio: `${rect.width} / ${rect.height}` } : undefined}
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
