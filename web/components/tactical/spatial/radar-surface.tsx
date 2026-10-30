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
import type { RadarRect } from '@/lib/tactical-transform';
import { radarProjection, renderedToWorld } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';

/** What a paint callback draws with: the context already panned to the radar square. */
export type RadarFrame = {
  ctx: CanvasRenderingContext2D;
  size: number;
  view: RadarRect;
  style: RadarStyle;
};

/** Lets the owner repaint the marks without React rendering, from an animation loop. */
export type RadarSurfaceHandle = { repaint: () => void };

/** Same height ceiling as the round replay, so the two views share one plate size. */
const RADAR_MAX_HEIGHT_VH = 74;

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
  const view = useMemo(() => radarViewRect(geometry), [geometry]);
  const drawable = isCalibrationUsable(geometry.calibration) && geometry.cell_size > 0;

  const repaint = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (canvas === null || container === null || !drawable) return;
    const ctx = canvas.getContext('2d', { alpha: false });
    if (ctx === null) return;
    const style = styleRef.current ?? readRadarStyle(canvas);
    styleRef.current = style;
    const { size, offsetX, offsetY } = projection;
    const key = `${size}|${dpr}|${level}|${geometry.map}|${view.x},${view.y},${view.width},${view.height}|${style.map}`;
    if (backgroundRef.current?.key !== key) {
      backgroundRef.current = { key, canvas: renderRadarBackground(geometry, level, size, dpr, style, view) };
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = style.background;
    ctx.fillRect(0, 0, canvas.width / dpr, canvas.height / dpr);
    ctx.translate(offsetX, offsetY);
    ctx.drawImage(backgroundRef.current.canvas, view.x * size, view.y * size, view.width * size, view.height * size);
    paintRef.current({ ctx, size, view, style });
    ctx.setTransform(1, 0, 0, 1, 0, 0);
  }, [drawable, dpr, geometry, level, projection, view]);

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
        <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full font-mono" />
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
