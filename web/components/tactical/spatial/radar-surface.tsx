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
import type { RadarRect } from '@/lib/tactical-transform';
import { radarProjection, renderedToWorld } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { useRadarImage } from '@/hooks/use-radar-image';

/** What a paint callback draws with: the context already panned to the radar square. */
export type RadarFrame = {
  ctx: CanvasRenderingContext2D;
  size: number;
  view: RadarRect;
  style: RadarStyle;
};

/** Lets the owner repaint the marks without React rendering, from an animation loop. */
export type RadarSurfaceHandle = { repaint: () => void };

const FULL_VIEW: RadarRect = { x: 0, y: 0, width: 1, height: 1 };

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

/**
 * A radar plate for the multi-round views: the play-derived map of one level,
 * baked once per size, with the owner's marks painted over it. The canvas is
 * `aria-hidden`; each view lists the same facts as text beside it. `children`
 * receives the projection, for layers such as the telestrator that sit on top.
 */
export function RadarSurface({
  geometry,
  level,
  paint,
  handleRef,
  onPoint,
  label,
  children,
}: {
  geometry: TacticalGeometry;
  level: RadarLevel;
  paint: (frame: RadarFrame) => void;
  handleRef?: RefObject<RadarSurfaceHandle | null>;
  /** A click on the plate, in world units. */
  onPoint?: (world: { x: number; y: number }) => void;
  label: string;
  children?: (projection: RadarProjection) => ReactNode;
}): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const backgroundRef = useRef<{ key: string; canvas: HTMLCanvasElement } | null>(null);
  const styleRef = useRef<RadarStyle | null>(null);
  const paintRef = useRef(paint);
  const repaintRef = useRef<() => void>(() => {});
  const [projection, setProjection] = useState<RadarProjection>({ size: 1, offsetX: 0, offsetY: 0 });
  const [dpr, setDpr] = useState(1);
  const drawable = isCalibrationUsable(geometry.calibration) && geometry.cell_size > 0;
  // CS2's radar when the game is installed; the play-derived cells only while it loads or when it is missing.
  const image = useRadarImage(geometry.map, level === RADAR_LEVELS.lower ? 'lower' : 'upper');
  // The whole map like the game's radar; only the play-derived cells are cropped to where people played.
  const view = useMemo(() => (image === null ? radarViewRect(geometry) : FULL_VIEW), [geometry, image]);

  const repaint = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (canvas === null || container === null || !drawable) return;
    const ctx = canvas.getContext('2d', { alpha: false });
    if (ctx === null) return;
    const style = styleRef.current ?? readRadarStyle(canvas);
    styleRef.current = style;
    const { size, offsetX, offsetY } = projection;
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
    paintRef.current({ ctx, size, view, style });
    ctx.setTransform(1, 0, 0, 1, 0, 0);
  }, [drawable, dpr, geometry, image, level, projection, view]);

  useEffect(() => {
    paintRef.current = paint;
    repaintRef.current = repaint;
    repaint();
  }, [paint, repaint]);

  useEffect(() => {
    if (handleRef === undefined) return;
    handleRef.current = { repaint };
    return () => {
      handleRef.current = null;
    };
  }, [handleRef, repaint]);

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
      setProjection((current) => {
        const next = radarProjection({ width: box.width, height: box.height }, view);
        return next.size === current.size && next.offsetX === current.offsetX && next.offsetY === current.offsetY ? current : next;
      });
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
  }, [view]);

  useEffect(() => {
    repaint();
  }, [repaint]);

  const onClick = (event: MouseEvent<HTMLDivElement>): void => {
    if (onPoint === undefined || !drawable) return;
    const box = event.currentTarget.getBoundingClientRect();
    const x = event.clientX - box.left - projection.offsetX;
    const y = event.clientY - box.top - projection.offsetY;
    onPoint(renderedToWorld(geometry.calibration, x, y, projection.size));
  };

  const plateVars: CSSProperties & { '--radar-max': string } = {
    '--radar-max': `min(100%, ${((RADAR_MAX_HEIGHT_VH * view.width) / view.height).toFixed(2)}vh)`,
  };

  return (
    <div className="mx-auto w-full max-w-(--radar-max)" style={plateVars}>
      <div
        ref={containerRef}
        role="img"
        aria-label={label}
        onClick={onClick}
        className="relative w-full overflow-hidden rounded-lg border border-border-strong bg-surface-0 shadow-[inset_0_1px_0_0_var(--edge-shade),inset_0_-1px_0_0_var(--edge-light)]"
        style={{ aspectRatio: `${view.width} / ${view.height}` }}
      >
        <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
        {drawable ? (
          children?.(projection)
        ) : (
          <p className="absolute inset-0 grid place-items-center px-6 text-center text-body-sm leading-5 text-fg-2">
            Este mapa no tiene una calibración de radar utilizable, así que no se puede dibujar.
          </p>
        )}
      </div>
    </div>
  );
}
