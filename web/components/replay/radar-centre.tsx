'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { X } from 'lucide-react';
import type { RadarLevel, TacticalSample } from '@/lib/api/tactical';
import { hitLines } from '@/lib/replay/hit-lines';
import { layerSamples, layerTrails } from '@/lib/replay/layers';
import { blockingSmokes, frameSight } from '@/lib/replay/sight-lines';
import type { FrameSight } from '@/lib/replay/sight-lines';
import { WHEEL_STEP, followView, paddedView, panView, sameRect, zoomLevel, zoomViewAt } from '@/lib/replay/map-view';
import { createInterpolatedSampleReader, createSampleTrailReader, dominantLevel, frameCursor } from '@/lib/tactical-replay';
import type { FrameCursor, TrailPoint } from '@/lib/tactical-replay';
import { radarProjection } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { timelineSeconds } from '@/lib/tactical-timeline';
import { isCalibrationUsable, radarViewRect, worldRectToRadarRect, worldToRendered } from '@/lib/tactical-transform';
import type { RadarRect } from '@/lib/tactical-transform';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { TelestratorLayer } from '@/components/tactical/spatial/telestrator';
import { Segmented } from '@/components/ui/segmented';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { cn } from '@/lib/utils';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayCentreProps, ReplayCentreView } from './centre';
import { loadNavRadar, loadRadarImage } from './radar/map-art';
import { PlayerTooltip, placeTooltip } from './radar/player-tooltip';
import { bakeMapPlate } from './radar/radar-bake';
import type { MapArt } from './radar/radar-bake';
import type { FrameAnalysis } from './radar/radar-analysis';
import { discRadius, drawRoundFrame, preloadGrenadeIcons } from './radar/radar-round';
import type { DrawnPlayer } from './radar/radar-round';
import { loadSightGeometry } from './radar/sight-geometry';
import type { SightGeometry } from './radar/sight-geometry';

type Box = { width: number; height: number; dpr: number };

/** Superposed rounds sit under the focused one at this strength. */
const OVERLAY_ALPHA = 0.42;
/** Air round the nav mesh, as a share of its longer side. */
const VIEW_MARGIN = 0.03;
/** A press that moves less than this is a click on a player, not a drag. */
const CLICK_SLOP = 4;
/** View wedges run this far in world units before they fade out, cast as this many rays each. */
const WEDGE_REACH = 1800;
const WEDGE_RAYS = 32;

type Readers = {
  samples: (cursor: FrameCursor) => TacticalSample[];
  trails: (cursor: FrameCursor) => ReadonlyMap<number, readonly TrailPoint[]>;
};

type ArtSource = 'scheme' | 'official';

/**
 * The 2D map: a stylized radar drawn from the map's nav mesh (or the game's
 * radar image when Studio has it), the players, their names and the grenades.
 * The wheel zooms on the cursor, a drag pans, a click highlights a player.
 * The canvas mirrors the clock and is `aria-hidden`; the panels say the same in text.
 */
function RadarCentre({ mode, rounds, clock, tickOf, layers, telestrator, selectedSlot, onSelectSlot, followSlot, onFollowSlot }: ReplayCentreProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const layerRef = useRef<HTMLCanvasElement | null>(null);
  const styleRef = useRef<RadarStyle | null>(null);
  const plateRef = useRef<{ key: string; canvas: HTMLCanvasElement } | null>(null);
  const drawnRef = useRef<DrawnPlayer[]>([]);
  const spotsRef = useRef(new Map<number, number>());
  const samplesRef = useRef<TacticalSample[]>([]);
  const tipRef = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState<Box>({ width: 1, height: 1, dpr: 1 });
  const [art, setArt] = useState<MapArt>({ kind: 'grid' });
  const [navReady, setNavReady] = useState<'loading' | 'ready' | 'failed'>('loading');
  const [official, setOfficial] = useState<{ upper: HTMLImageElement; lower: HTMLImageElement | null } | null>(null);
  const [source, setSource] = useState<ArtSource>('official');
  const [zoomed, setZoomed] = useState<RadarRect | null>(null);
  const [hover, setHover] = useState<number | null>(null);
  const [grabbing, setGrabbing] = useState(false);
  const [sightGeometry, setSightGeometry] = useState<{ map: string; result: SightGeometry } | null>(null);
  const sightMemo = useRef<{ key: string; value: FrameSight } | null>(null);

  const focused = rounds.find((round) => round.focused) ?? rounds.at(-1);
  const geometry = focused?.doc.geometry;
  const map = geometry?.map ?? '';
  const drawable = geometry !== undefined && isCalibrationUsable(geometry.calibration);
  const split = geometry?.calibration.lower_altitude_max !== undefined;

  useEffect(() => {
    if (map === '') return;
    let active = true;
    setNavReady('loading');
    setZoomed(null);
    void loadNavRadar(map).then((model) => {
      if (!active) return;
      setNavReady(model === null ? 'failed' : 'ready');
      setArt(model === null ? { kind: 'grid' } : { kind: 'nav', model });
    });
    setOfficial(null);
    void Promise.all([loadRadarImage(map, 'upper'), split ? loadRadarImage(map, 'lower') : Promise.resolve(null)]).then(([upper, lower]) => {
      if (active) setOfficial(upper === null ? null : { upper, lower });
    });
    return () => {
      active = false;
    };
  }, [map, split]);

  useEffect(() => {
    if (!layers.sight || map === '') return;
    let active = true;
    void loadSightGeometry(map).then((result) => {
      if (active) setSightGeometry({ map, result });
    });
    return () => {
      active = false;
    };
  }, [layers.sight, map]);
  const sightBvh = layers.sight && sightGeometry?.map === map && sightGeometry.result.kind === 'ready' ? sightGeometry.result.bvh : null;

  const shownArt = useMemo<MapArt>(() => (source === 'official' && official !== null ? { kind: 'image', upper: official.upper, lower: official.lower } : art), [art, official, source]);

  const base = useMemo<RadarRect>(() => {
    if (geometry === undefined || !drawable) return { x: 0, y: 0, width: 1, height: 1 };
    if (art.kind === 'nav') {
      const { bounds } = art.model;
      const rect = worldRectToRadarRect(geometry.calibration, { minX: bounds.minX, minY: bounds.minY, maxX: bounds.maxX, maxY: bounds.maxY }, 1);
      return paddedView(rect, VIEW_MARGIN);
    }
    return radarViewRect(geometry);
  }, [art, drawable, geometry]);
  const view = zoomed ?? base;
  const projection = useMemo<RadarProjection>(() => radarProjection(box, view), [box, view]);
  const zoom = zoomLevel(view, base);

  const readers = useMemo(() => {
    const out = new Map<string, Readers>();
    for (const round of rounds) {
      out.set(round.key, {
        samples: createInterpolatedSampleReader(round.frames),
        trails: createSampleTrailReader(round.frames, round.timeline.tickrate, layers.trailSeconds),
      });
    }
    return out;
  }, [layers.trailSeconds, rounds]);

  const plate = useCallback(
    (level: RadarLevel, style: RadarStyle): HTMLCanvasElement | null => {
      if (geometry === undefined) return null;
      const key = `${shownArt.kind}|${level}|${projection.size}|${box.dpr}|${view.x},${view.y},${view.width},${view.height}|${map}`;
      if (plateRef.current?.key === key) return plateRef.current.canvas;
      const canvas = bakeMapPlate({ art: shownArt, geometry, level, size: projection.size, dpr: box.dpr, view, style, zoom });
      plateRef.current = { key, canvas };
      return canvas;
    },
    [box.dpr, geometry, map, projection.size, shownArt, view, zoom],
  );

  const drawRound = useCallback(
    (ctx: CanvasRenderingContext2D, round: CentreRound, seconds: number, style: RadarStyle): DrawnPlayer[] => {
      const reader = readers.get(round.key);
      if (reader === undefined || geometry === undefined) return [];
      const tick = tickOf(round, seconds);
      const cursor = frameCursor(round.frames, tick);
      if (cursor === undefined) return [];
      const samples = reader.samples(cursor);
      if (round.focused) samplesRef.current = samples;
      const nowSeconds = timelineSeconds(round.timeline, tick);
      const deadFor = new Map<number, number>();
      for (const player of round.round.players) {
        if (player.death_tick !== undefined && player.death_tick <= tick) deadFor.set(player.slot, nowSeconds - timelineSeconds(round.timeline, player.death_tick));
      }
      const visible = { x: view.x * projection.size, y: view.y * projection.size, width: view.width * projection.size, height: view.height * projection.size };
      const shown = layerSamples(samples, layers);
      const secondsOf = (at: number): number => timelineSeconds(round.timeline, at);
      let analysis: FrameAnalysis | null = null;
      if (round.focused) {
        let sight: FrameSight | null = null;
        if (sightBvh !== null) {
          const smokes = blockingSmokes({ events: round.round.events, nowSeconds, secondsOf });
          const key = `${smokes.length}|${shown.map((sample) => `${sample.slot}:${sample.flags}:${Math.round(sample.x)},${Math.round(sample.y)},${Math.round(sample.z)},${Math.round(sample.yaw)}`).join(';')}`;
          if (sightMemo.current?.key !== key) sightMemo.current = { key, value: frameSight({ bvh: sightBvh, samples: shown, smokes, reach: WEDGE_REACH, rays: WEDGE_RAYS }) };
          sight = sightMemo.current.value;
        }
        const hits = layers.hits ? hitLines({ hits: round.round.hits ?? [], events: round.round.events, nowSeconds, secondsOf, hiddenSlots: layers.hiddenSlots }) : [];
        analysis = { sight, hits };
      }
      return drawRoundFrame(
        ctx,
        {
          calibration: geometry.calibration,
          size: projection.size,
          samples: shown,
          trails: layerTrails(reader.trails(cursor), layers),
          events: round.events,
          nowSeconds,
          secondsOf,
          deadFor,
          labels: round.focused ? round.labels : new Map(),
          selectedSlot: round.focused ? selectedSlot : null,
          utility: layers.utility,
          bomb: layers.bomb,
          style,
          analysis,
        },
        round.focused ? { visible, spots: spotsRef.current } : null,
      );
    },
    [geometry, layers, projection.size, readers, selectedSlot, sightBvh, tickOf, view],
  );

  const render = useCallback(
    (seconds: number) => {
      const canvas = canvasRef.current;
      if (!drawable || geometry === undefined || focused === undefined || canvas === null) return;
      const ctx = canvas.getContext('2d', { alpha: false });
      if (ctx === null) return;
      const style = styleRef.current ?? readRadarStyle(canvas);
      styleRef.current = style;
      const { dpr } = box;
      const reader = readers.get(focused.key);
      const cursor = frameCursor(focused.frames, tickOf(focused, seconds));
      const level = dominantLevel(geometry.calibration, cursor === undefined || reader === undefined ? [] : reader.samples(cursor));

      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.fillStyle = '#0b0c0e';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      const baked = plate(level, style);
      const left = projection.offsetX + view.x * projection.size;
      const top = projection.offsetY + view.y * projection.size;
      if (baked !== null) ctx.drawImage(baked, Math.round(left * dpr), Math.round(top * dpr));

      const others = rounds.filter((round) => round !== focused);
      if (others.length > 0) {
        const layer = layerRef.current ?? document.createElement('canvas');
        layerRef.current = layer;
        if (layer.width !== canvas.width) layer.width = canvas.width;
        if (layer.height !== canvas.height) layer.height = canvas.height;
        const layerCtx = layer.getContext('2d');
        if (layerCtx !== null) {
          layerCtx.setTransform(1, 0, 0, 1, 0, 0);
          layerCtx.clearRect(0, 0, layer.width, layer.height);
          layerCtx.setTransform(dpr, 0, 0, dpr, projection.offsetX * dpr, projection.offsetY * dpr);
          for (const round of others) drawRound(layerCtx, round, seconds, style);
          ctx.globalAlpha = OVERLAY_ALPHA;
          ctx.drawImage(layer, 0, 0);
          ctx.globalAlpha = 1;
        }
      }
      ctx.setTransform(dpr, 0, 0, dpr, projection.offsetX * dpr, projection.offsetY * dpr);
      drawnRef.current = drawRound(ctx, focused, seconds, style);
      ctx.setTransform(1, 0, 0, 1, 0, 0);

      const tip = tipRef.current;
      const hovered = hover === null ? undefined : drawnRef.current.find((player) => player.slot === hover);
      if (tip !== null && hovered !== undefined) placeTooltip(tip, hovered.x + projection.offsetX, hovered.y + projection.offsetY, box);
      const followed = followSlot === null ? undefined : samplesRef.current.find((sample) => sample.slot === followSlot);
      if (followed !== undefined) {
        const at = worldToRendered(geometry.calibration, followed.x, followed.y, 1);
        const next = followView({ view, base, point: { x: at.x, y: at.y } });
        if (!sameRect(next, view)) setZoomed(next);
      }
    },
    [base, box, drawRound, drawable, focused, followSlot, geometry, hover, plate, projection, readers, rounds, tickOf, view],
  );

  const renderRef = useRef(render);
  useEffect(() => {
    renderRef.current = render;
    render(clock.seconds);
  }, [render, clock]);
  useEffect(() => clock.subscribe((seconds) => renderRef.current(seconds)), [clock]);
  useEffect(() => {
    let active = true;
    void preloadGrenadeIcons().then(() => active && renderRef.current(clock.seconds));
    return () => {
      active = false;
    };
  }, [clock]);

  useEffect(() => {
    const container = containerRef.current;
    const canvas = canvasRef.current;
    if (container === null || canvas === null) return;
    const measure = (): void => {
      const rect = container.getBoundingClientRect();
      const dpr = window.devicePixelRatio > 0 ? window.devicePixelRatio : 1;
      const width = Math.max(1, Math.floor(rect.width));
      const height = Math.max(1, Math.floor(rect.height));
      const pixelWidth = Math.round(width * dpr);
      const pixelHeight = Math.round(height * dpr);
      // Resizing a canvas clears it, so only resize on a real change and draw again at once.
      if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
        canvas.width = pixelWidth;
        canvas.height = pixelHeight;
        renderRef.current(clock.seconds);
      }
      setBox((current) => (current.width === width && current.height === height && current.dpr === dpr ? current : { width, height, dpr }));
    };
    const restyle = (): void => {
      styleRef.current = null;
      plateRef.current = null;
      renderRef.current(clock.seconds);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    const themeObserver = new MutationObserver(restyle);
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    document.fonts.addEventListener('loadingdone', restyle);
    return () => {
      observer.disconnect();
      themeObserver.disconnect();
      document.fonts.removeEventListener('loadingdone', restyle);
    };
  }, [clock]);

  // The wheel listener is not passive so the page does not scroll under the map.
  const zoomStateRef = useRef({ box, view, base });
  zoomStateRef.current = { box, view, base };
  useEffect(() => {
    const container = containerRef.current;
    if (container === null) return;
    const onWheel = (event: WheelEvent): void => {
      event.preventDefault();
      const rect = container.getBoundingClientRect();
      const state = zoomStateRef.current;
      const next = zoomViewAt({ box: state.box, view: state.view, base: state.base, cursor: { x: event.clientX - rect.left, y: event.clientY - rect.top }, factor: WHEEL_STEP ** -event.deltaY });
      setZoomed(sameRect(next, state.base) ? null : next);
    };
    container.addEventListener('wheel', onWheel, { passive: false });
    return () => container.removeEventListener('wheel', onWheel);
  }, []);

  const playerAt = (x: number, y: number): number | null => {
    const reach = discRadius(projection.size) + 6;
    let best: { slot: number; distance: number } | null = null;
    for (const player of drawnRef.current) {
      if (!player.alive) continue;
      const distance = Math.hypot(player.x + projection.offsetX - x, player.y + projection.offsetY - y);
      if (distance <= reach && (best === null || distance < best.distance)) best = { slot: player.slot, distance };
    }
    return best?.slot ?? null;
  };

  const dragRef = useRef<{ x: number; y: number; moved: boolean; id: number } | null>(null);
  const local = (event: { clientX: number; clientY: number }): { x: number; y: number } => {
    const rect = containerRef.current?.getBoundingClientRect();
    return rect === undefined ? { x: 0, y: 0 } : { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };

  const zoomBy = (factor: number): void => {
    const next = zoomViewAt({ box, view, base, cursor: { x: box.width / 2, y: box.height / 2 }, factor });
    setZoomed(sameRect(next, base) ? null : next);
  };

  let cursorClass = 'cursor-grab';
  if (grabbing) cursorClass = 'cursor-grabbing';
  else if (hover !== null) cursorClass = 'cursor-pointer';
  const label = mode === 'multi' ? `${rounds.length} rondas superpuestas desde el final del tiempo de compra` : `Mapa de la ronda ${focused?.round.number ?? ''}`;
  const hovered = hover === null ? null : drawnRef.current.find((player) => player.slot === hover);
  const hoveredSample = hover === null ? undefined : samplesRef.current.find((sample) => sample.slot === hover);
  const followedName = followSlot === null || focused === undefined ? null : (focused.doc.players.find((player) => player.slot === followSlot)?.name ?? null);
  const hoveredName = hover === null || focused === undefined ? '' : (focused.doc.players.find((player) => player.slot === hover)?.name ?? '');

  return (
    <div className="absolute inset-0 overflow-hidden bg-[#0b0c0e]">
      <div
        ref={containerRef}
        role="img"
        aria-label={label}
        className={cn('absolute inset-0', cursorClass)}
        onPointerDown={(event) => {
          if (event.button !== 0) return;
          const at = local(event);
          dragRef.current = { ...at, moved: false, id: event.pointerId };
          event.currentTarget.setPointerCapture(event.pointerId);
        }}
        onPointerMove={(event) => {
          const at = local(event);
          const drag = dragRef.current;
          if (drag !== null && drag.id === event.pointerId) {
            const dx = at.x - drag.x;
            const dy = at.y - drag.y;
            if (!drag.moved && Math.hypot(dx, dy) < CLICK_SLOP) return;
            drag.moved = true;
            if (followSlot !== null) onFollowSlot(null);
            setGrabbing(true);
            setHover(null);
            drag.x = at.x;
            drag.y = at.y;
            const next = panView({ box, view, base, dx, dy });
            setZoomed(sameRect(next, base) ? null : next);
            return;
          }
          const slot = playerAt(at.x, at.y);
          setHover((current) => (current === slot ? current : slot));
        }}
        onPointerUp={(event) => {
          const drag = dragRef.current;
          dragRef.current = null;
          setGrabbing(false);
          if (drag === null || drag.moved) return;
          const at = local(event);
          const slot = playerAt(at.x, at.y);
          if (slot !== null) onSelectSlot(slot === selectedSlot ? null : slot);
        }}
        onPointerLeave={() => setHover(null)}
        onDoubleClick={() => setZoomed(null)}
      >
        <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
      </div>
      {drawable && telestrator !== null && geometry !== undefined ? (
        <TelestratorLayer calibration={geometry.calibration} projection={projection} shown={telestrator.shown} draft={telestrator.draft} />
      ) : null}
      {hovered === undefined || hovered === null || hoveredSample === undefined || grabbing ? null : (
        <PlayerTooltip
          ref={(element) => {
            tipRef.current = element;
            if (element !== null) placeTooltip(element, hovered.x + projection.offsetX, hovered.y + projection.offsetY, box);
          }}
          sample={hoveredSample}
          name={hoveredName}
        />
      )}

      <div className="pointer-events-none absolute bottom-2 left-2 flex max-w-[calc(100%-12rem)] flex-wrap items-end gap-2">
        {official === null ? null : (
          <div className="pointer-events-auto">
            <Segmented
              label="Dibujo del mapa"
              value={source}
              options={[
                { value: 'scheme', label: 'Esquema' },
                { value: 'official', label: 'Radar oficial' },
              ]}
              onChange={setSource}
            />
          </div>
        )}
        {followedName === null ? null : (
          <button
            type="button"
            onClick={() => onFollowSlot(null)}
            className="pointer-events-auto flex h-9 items-center gap-1.5 rounded-md border border-border-strong bg-surface-1/95 px-2.5 text-meta text-fg-1 outline-none hover:bg-surface-3 focus-visible:ring-2 focus-visible:ring-ring"
          >
            Siguiendo a <span className="max-w-32 truncate font-semibold">{followedName}</span>
            <X className="size-3.5 text-fg-3" aria-label="Dejar de seguir" />
          </button>
        )}
        {layers.sight && sightGeometry?.map === map && sightGeometry.result.kind === 'failed' ? (
          <p className="pointer-events-auto max-w-72 rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">{sightGeometry.result.message}</p>
        ) : null}
        {layers.sight && sightGeometry?.map !== map ? (
          <p role="status" className="pointer-events-auto rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            Leyendo las paredes del mapa…
          </p>
        ) : null}
        {navReady === 'failed' && (source === 'scheme' || official === null) ? (
          <p className="pointer-events-auto max-w-72 rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            No se pudo leer el mapa de tu instalación de CS2: se dibuja con las posiciones de la demo.
          </p>
        ) : null}
      </div>

      <MapZoomControls zoom={zoom} zoomed={zoomed !== null} onZoomBy={zoomBy} onReset={() => setZoomed(null)} className="absolute right-2 bottom-2" />

      {drawable ? null : (
        <p className="absolute inset-0 grid place-items-center px-6 text-center text-body-sm leading-5 text-fg-2">
          Este mapa no tiene una calibración de radar utilizable, así que no se puede dibujar la repetición. Los eventos siguen en la lista.
        </p>
      )}
      {drawable && focused !== undefined && focused.frames.length === 0 ? (
        <p className="absolute inset-x-0 bottom-12 mx-auto w-fit rounded-md border border-warning/45 bg-surface-0/90 px-3 py-2 text-body-sm text-warning">
          Esta ronda no tiene posiciones guardadas: el mapa queda vacío, los eventos no.
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
