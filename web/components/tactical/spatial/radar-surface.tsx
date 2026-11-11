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
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry } from '@/lib/api/tactical';
import { isCalibrationUsable, radarViewRect } from '@/lib/tactical-transform';
import type { RadarRect, WorldMark } from '@/lib/tactical-transform';
import { LEVEL_LABEL, radarProjection, renderedToWorld } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { useRadarImage } from '@/hooks/use-radar-image';
import { cn } from '@/lib/utils';
import { useMapZoom } from '@/hooks/use-map-zoom';
import type { MapZoom } from '@/hooks/use-map-zoom';
import { radarImageFits } from '@/lib/replay/map-view';
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
/** Greys CS2's radar down so team colours and marks stay the loudest thing on it. */
const RADAR_IMAGE_FILTER = 'grayscale(0.55) brightness(0.78) contrast(1.05)';

/**
 * The background bitmap from CS2's own radar image: the 1024 px overview covers
 * the calibration's native square, so it scales by `size / calibration.size`.
 */
function renderImageBackground(
  image: HTMLImageElement,
  geometry: TacticalGeometry,
  frame: { size: number; dpr: number; style: RadarStyle; view: RadarRect },
): HTMLCanvasElement {
  const { size, dpr, style, view } = frame;
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(view.width * size * dpr));
  canvas.height = Math.max(1, Math.round(view.height * size * dpr));
  const ctx = canvas.getContext('2d');
  if (ctx === null) return canvas;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.fillStyle = style.panel;
  ctx.fillRect(0, 0, view.width * size, view.height * size);
  ctx.translate(-view.x * size, -view.y * size);
  const scale = size / geometry.calibration.size;
  ctx.filter = RADAR_IMAGE_FILTER;
  ctx.drawImage(image, 0, 0, image.naturalWidth * scale, image.naturalHeight * scale);
  ctx.filter = 'none';
  return canvas;
}

function radarLayer(level: RadarLevel): 'upper' | 'lower' {
  return level === RADAR_LEVELS.lower ? 'lower' : 'upper';
}

/**
 * A radar plate for the multi-round views: the play-derived map of one level,
 * baked once per size, with the owner's marks painted over it. The canvas is
 * `aria-hidden`; each view lists the same facts as text beside it. `children`
 * receives the projection, for layers such as the telestrator that sit on top,
 * and whether the map is zoomed, when a plain drag pans instead.
 * With two `levels` (Nuke, Vertigo) both floors sit side by side on one zoom.
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
  const shown: readonly RadarLevel[] = levels !== undefined && levels.length > 1 ? levels.slice(0, 2) : [level];
  const split = shown.length > 1;
  const repaints = useRef(new Map<RadarLevel, () => void>());
  const drawable = isCalibrationUsable(geometry.calibration) && geometry.cell_size > 0;
  // CS2's radar when the game is installed; the play-derived cells only while it loads or when it is missing.
  // A derived calibration does not line up with the image, so such a map keeps its cells.
  const fits = radarImageFits(geometry.calibration);
  const first = useRadarImage(fits ? geometry.map : '', radarLayer(shown[0]));
  const second = useRadarImage(fits && split ? geometry.map : '', radarLayer(shown[1] ?? RADAR_LEVELS.lower));
  const images = split ? [first, second] : [first];
  // Until every image settles nothing is drawn, so the cells never flash before it, and the map takes no zoom.
  const settled = images.every((image) => image !== undefined);
  // Image or cells, both floors are cropped to where people played and the marks: a tall map stays tall, a wide one wide.
  const base = useMemo(() => radarViewRect(geometry, marks), [geometry, marks]);
  const zoom = useMapZoom(base, { enabled: settled });

  useEffect(() => {
    if (handleRef === undefined) return;
    const plates = repaints.current;
    handleRef.current = { repaint: () => plates.forEach((repaint) => repaint()) };
    return () => {
      handleRef.current = null;
    };
  }, [handleRef]);

  const plateHeight = (RADAR_MAX_HEIGHT_VH * base.width) / base.height;
  const plateVars: CSSProperties & { '--radar-max': string } = {
    '--radar-max': split ? `min(100%, calc(${(2 * plateHeight).toFixed(2)}vh + 0.5rem))` : `min(100%, ${plateHeight.toFixed(2)}vh)`,
  };

  return (
    <div className={cn('mx-auto w-full max-w-(--radar-max)', split && '@container')} style={plateVars}>
      <div className={cn(split && 'grid grid-cols-1 gap-2 @[32rem]:grid-cols-2')}>
        {shown.map((plateLevel, index) => (
          <RadarPlate
            key={plateLevel}
            geometry={geometry}
            level={plateLevel}
            image={images[index] ?? null}
            settled={settled}
            drawable={drawable}
            base={base}
            zoom={zoom}
            paint={paint}
            repaints={repaints.current}
            onPoint={onPoint}
            label={split ? `${label} ${LEVEL_LABEL[plateLevel]}.` : label}
            floorLabel={split ? LEVEL_LABEL[plateLevel] : null}
            controls={index === shown.length - 1}
          >
            {children}
          </RadarPlate>
        ))}
      </div>
    </div>
  );
}

/** One floor's canvas: its own bitmap and size, the zoom shared with the other floor. */
function RadarPlate({
  geometry,
  level,
  image,
  settled,
  drawable,
  base,
  zoom,
  paint,
  repaints,
  onPoint,
  label,
  floorLabel,
  controls,
  children,
}: {
  geometry: TacticalGeometry;
  level: RadarLevel;
  image: HTMLImageElement | null;
  settled: boolean;
  drawable: boolean;
  base: RadarRect;
  zoom: MapZoom;
  paint: (frame: RadarFrame) => void;
  repaints: Map<RadarLevel, () => void>;
  onPoint?: (world: { x: number; y: number }, level: RadarLevel) => void;
  label: string;
  floorLabel: string | null;
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
  const view = zoom.view;
  const projection = useMemo<RadarProjection>(() => radarProjection(box, view), [box, view]);
  const bindZoom = zoom.surface.ref;
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
    const key = `${size}|${dpr}|${level}|${geometry.map}|${view.x},${view.y},${view.width},${view.height}|${style.map}|${image ? 'cs2' : 'cells'}`;
    if (backgroundRef.current?.key !== key) {
      const canvas = image
        ? renderImageBackground(image, geometry, { size, dpr, style, view })
        : renderRadarBackground(geometry, level, size, dpr, style, view);
      backgroundRef.current = { key, canvas };
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = style.background;
    ctx.fillRect(0, 0, canvas.width / dpr, canvas.height / dpr);
    ctx.translate(offsetX, offsetY);
    ctx.drawImage(backgroundRef.current.canvas, view.x * size, view.y * size, view.width * size, view.height * size);
    paintRef.current({ ctx, size, view, style, level });
    ctx.setTransform(1, 0, 0, 1, 0, 0);
  }, [drawable, dpr, geometry, image, level, projection, settled, view]);

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
      {...zoom.surface}
      ref={bindContainer}
      role="img"
      aria-label={label}
      onClick={onClick}
      className={cn(
        'relative w-full overflow-hidden rounded-lg border border-border-strong bg-surface-0 shadow-[inset_0_1px_0_0_var(--edge-shade),inset_0_-1px_0_0_var(--edge-light)]',
        zoom.zoomed && 'touch-none',
        zoom.cursor,
      )}
      style={{ aspectRatio: `${base.width} / ${base.height}` }}
    >
      <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
      {drawable ? (
        <>
          {children?.(projection, zoom.zoomed, level)}
          {floorLabel !== null ? (
            <span aria-hidden className="pointer-events-none absolute top-2 left-2 rounded-sm border border-border bg-surface-1 px-1.5 py-0.5 text-meta text-fg-2">
              {floorLabel}
            </span>
          ) : null}
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
