'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import type { RadarLevel } from '@/lib/api/tactical';
import { layerSamples, layerTrails } from '@/lib/replay/layers';
import { TacticalDrawCache } from '@/lib/tactical-draw-cache';
import { createInterpolatedSampleReader, createSampleTrailReader, dominantLevel, frameCursor } from '@/lib/tactical-replay';
import type { FrameCursor, TrailPoint } from '@/lib/tactical-replay';
import { radarProjection } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { TRAIL_SECONDS, timelineSeconds } from '@/lib/tactical-timeline';
import { isCalibrationUsable, radarViewRect } from '@/lib/tactical-transform';
import type { TacticalSample } from '@/lib/api/tactical';
import { drawTacticalScene, renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { TelestratorLayer } from '@/components/tactical/spatial/telestrator';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayCentreProps, ReplayCentreView } from './centre';

type CanvasMetrics = { cssWidth: number; cssHeight: number; dpr: number; size: number; offsetX: number; offsetY: number };

const INITIAL_METRICS: CanvasMetrics = { cssWidth: 1, cssHeight: 1, dpr: 1, size: 1, offsetX: 0, offsetY: 0 };

/** Superposed rounds sit under the focused one at this strength. */
const OVERLAY_ALPHA = 0.42;

/** The plate's height ceiling, so the strip and the timeline stay on screen under it. */
const RADAR_MAX_HEIGHT_VH = 62;

type RoundReaders = {
  samples: (cursor: FrameCursor) => TacticalSample[];
  trails: (cursor: FrameCursor) => ReadonlyMap<number, readonly TrailPoint[]>;
};

/**
 * The 2D radar centre. The canvas mirrors the clock and is `aria-hidden`; the
 * event list and the player cards say the same in text.
 */
function RadarCentre({ mode, rounds, clock, tickOf, layers, telestrator }: ReplayCentreProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const layerRef = useRef<HTMLCanvasElement | null>(null);
  const metricsRef = useRef<CanvasMetrics>(INITIAL_METRICS);
  const styleRef = useRef<RadarStyle | null>(null);
  const backgroundsRef = useRef(new Map<string, HTMLCanvasElement>());
  const [projection, setProjection] = useState<RadarProjection>({ size: 1, offsetX: 0, offsetY: 0 });

  const focused = rounds.find((round) => round.focused) ?? rounds.at(-1);
  const geometry = focused?.doc.geometry;
  const view = useMemo(() => (geometry === undefined ? { x: 0, y: 0, width: 1, height: 1 } : radarViewRect(geometry)), [geometry]);
  const drawable = geometry !== undefined && isCalibrationUsable(geometry.calibration) && geometry.cell_size > 0;
  const drawCache = useMemo(() => new TacticalDrawCache(), []);

  const readers = useMemo(() => {
    const out = new Map<string, RoundReaders>();
    for (const round of rounds) {
      out.set(round.key, {
        samples: createInterpolatedSampleReader(round.frames),
        trails: createSampleTrailReader(round.frames, round.timeline.tickrate, TRAIL_SECONDS),
      });
    }
    return out;
  }, [rounds]);

  const background = useCallback(
    (level: RadarLevel, size: number, dpr: number, style: RadarStyle): HTMLCanvasElement | null => {
      if (geometry === undefined) return null;
      const key = `${size}|${dpr}|${level}|${geometry.map}|${view.x},${view.y},${view.width},${view.height}`;
      const cached = backgroundsRef.current.get(key);
      if (cached !== undefined) return cached;
      const baked = renderRadarBackground(geometry, level, size, dpr, style, view);
      if (backgroundsRef.current.size > 8) backgroundsRef.current.clear();
      backgroundsRef.current.set(key, baked);
      return baked;
    },
    [geometry, view],
  );

  const drawRound = useCallback(
    (context: CanvasRenderingContext2D, round: CentreRound, seconds: number, style: RadarStyle, size: number): TacticalSample[] => {
      const reader = readers.get(round.key);
      if (reader === undefined || geometry === undefined) return [];
      const tick = tickOf(round, seconds);
      const cursor = frameCursor(round.frames, tick);
      if (cursor === undefined) return [];
      const samples = reader.samples(cursor);
      drawTacticalScene(context, {
        size,
        view,
        // One calibration for every round: world coordinates of one map agree, the focused plate is the frame.
        geometry,
        activeLevel: dominantLevel(geometry.calibration, samples),
        samples: layerSamples(samples, layers),
        trails: layerTrails(reader.trails(cursor), layers),
        events: round.events,
        nowSeconds: timelineSeconds(round.timeline, tick),
        labels: round.focused ? round.labels : new Map(),
        style,
      }, drawCache);
      return samples;
    },
    [drawCache, geometry, layers, readers, tickOf, view],
  );

  const render = useCallback(
    (seconds: number) => {
      if (!drawable || geometry === undefined || focused === undefined) return;
      const canvas = canvasRef.current;
      if (canvas === null) return;
      const context = canvas.getContext('2d', { alpha: false });
      if (context === null) return;
      const style = styleRef.current ?? readRadarStyle(canvas);
      const { cssWidth, cssHeight, dpr, size, offsetX, offsetY } = metricsRef.current;

      const focusTick = tickOf(focused, seconds);
      const focusCursor = frameCursor(focused.frames, focusTick);
      const focusSamples = focusCursor === undefined ? [] : (readers.get(focused.key)?.samples(focusCursor) ?? []);
      const level = dominantLevel(geometry.calibration, focusSamples);

      context.setTransform(dpr, 0, 0, dpr, 0, 0);
      context.fillStyle = style.background;
      context.fillRect(0, 0, cssWidth, cssHeight);
      context.translate(offsetX, offsetY);
      const baked = background(level, size, dpr, style);
      if (baked !== null) context.drawImage(baked, view.x * size, view.y * size, view.width * size, view.height * size);

      const others = rounds.filter((round) => round !== focused);
      if (others.length > 0) {
        const layer = layerRef.current ?? document.createElement('canvas');
        layerRef.current = layer;
        if (layer.width !== canvas.width) layer.width = canvas.width;
        if (layer.height !== canvas.height) layer.height = canvas.height;
        const layerContext = layer.getContext('2d');
        if (layerContext !== null) {
          layerContext.setTransform(1, 0, 0, 1, 0, 0);
          layerContext.clearRect(0, 0, layer.width, layer.height);
          layerContext.setTransform(dpr, 0, 0, dpr, 0, 0);
          layerContext.translate(offsetX, offsetY);
          for (const round of others) drawRound(layerContext, round, seconds, style, size);
          context.setTransform(1, 0, 0, 1, 0, 0);
          context.globalAlpha = OVERLAY_ALPHA;
          context.drawImage(layer, 0, 0);
          context.globalAlpha = 1;
          context.setTransform(dpr, 0, 0, dpr, 0, 0);
          context.translate(offsetX, offsetY);
        }
      }
      drawRound(context, focused, seconds, style, size);
      context.setTransform(1, 0, 0, 1, 0, 0);
    },
    [background, drawRound, drawable, focused, geometry, readers, rounds, tickOf, view],
  );

  const renderRef = useRef(render);
  useEffect(() => {
    renderRef.current = render;
    render(clock.seconds);
  }, [render, clock]);

  useEffect(() => clock.subscribe((seconds) => renderRef.current(seconds)), [clock]);

  useEffect(() => {
    backgroundsRef.current.clear();
  }, [geometry]);

  useEffect(() => {
    const container = containerRef.current;
    const canvas = canvasRef.current;
    if (container === null || canvas === null) return;
    const measure = (): void => {
      const box = container.getBoundingClientRect();
      const dpr = window.devicePixelRatio > 0 ? window.devicePixelRatio : 1;
      const cssWidth = Math.max(1, box.width);
      const cssHeight = Math.max(1, box.height);
      const width = Math.round(cssWidth * dpr);
      const height = Math.round(cssHeight * dpr);
      if (canvas.width !== width) canvas.width = width;
      if (canvas.height !== height) canvas.height = height;
      const fit = radarProjection({ width: cssWidth, height: cssHeight }, view);
      metricsRef.current = { cssWidth, cssHeight, dpr, ...fit };
      setProjection((current) =>
        current.size === fit.size && current.offsetX === fit.offsetX && current.offsetY === fit.offsetY ? current : fit,
      );
      styleRef.current = readRadarStyle(canvas);
      renderRef.current(clock.seconds);
    };
    const stale = (): void => {
      drawCache.clear();
      backgroundsRef.current.clear();
      measure();
    };
    measure();
    document.fonts.addEventListener('loadingdone', stale);
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    window.addEventListener('resize', measure);
    const themeObserver = new MutationObserver(stale);
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    return () => {
      observer.disconnect();
      themeObserver.disconnect();
      document.fonts.removeEventListener('loadingdone', stale);
      window.removeEventListener('resize', measure);
    };
  }, [view, drawCache, clock]);

  // Typed rather than cast: CSSProperties has no index signature for custom properties.
  const radarVars = useMemo<CSSProperties & { '--radar-max': string }>(
    () => ({ '--radar-max': `min(100%, ${((RADAR_MAX_HEIGHT_VH * view.width) / view.height).toFixed(2)}vh)` }),
    [view],
  );

  const label = mode === 'multi'
    ? `${rounds.length} rondas superpuestas desde el final del tiempo de compra`
    : `Radar de la ronda ${focused?.round.number ?? ''}`;

  return (
    <div className="flex min-w-0 flex-col gap-2" style={radarVars}>
      <div
        ref={containerRef}
        role="img"
        aria-label={label}
        className="relative mx-auto w-full max-w-(--radar-max) overflow-hidden rounded-lg border border-border-strong bg-surface-0 shadow-[inset_0_1px_0_0_var(--edge-shade),inset_0_-1px_0_0_var(--edge-light)]"
        style={{ aspectRatio: `${view.width} / ${view.height}` }}
      >
        <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
        {drawable && telestrator !== null && geometry !== undefined ? (
          <TelestratorLayer calibration={geometry.calibration} projection={projection} shown={telestrator.shown} draft={telestrator.draft} />
        ) : null}
        {drawable ? null : (
          <p className="absolute inset-0 grid place-items-center px-6 text-center text-body-sm leading-5 text-fg-2">
            Este mapa no tiene una calibración de radar utilizable, así que no se puede dibujar la repetición. Los eventos
            siguen en la lista.
          </p>
        )}
      </div>
      {drawable && focused !== undefined && focused.frames.length === 0 ? (
        <p className="mx-auto w-full max-w-(--radar-max) rounded-md border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm leading-5 text-warning">
          Esta ronda no tiene posiciones guardadas: el radar queda vacío, los eventos no.
        </p>
      ) : null}
    </div>
  );
}

/** The default centre view. Put a 3D view after it in `centreViews` to offer both. */
export const RADAR_2D_VIEW: ReplayCentreView = {
  id: '2d',
  label: '2D',
  render: (props) => <RadarCentre {...props} />,
};
