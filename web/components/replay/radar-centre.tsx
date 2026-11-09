'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { RADAR_LEVELS } from '@/lib/api/tactical';
import type { RadarLevel, TacticalSample } from '@/lib/api/tactical';
import { OFF_FLOOR_ALPHA } from '@/lib/analysis/floors';
import { BOTH_FLOORS, floorPanes, localProjection, onFloorOf, paneAt, paneProjection } from '@/lib/replay/floor-split';
import { hitLines } from '@/lib/replay/hit-lines';
import { layerSamples, layerTrails } from '@/lib/replay/layers';
import { blockingSmokes, frameSight } from '@/lib/replay/sight-lines';
import type { FrameSight } from '@/lib/replay/sight-lines';
import { MAP_SIZE_SCALE } from '@/lib/replay/map-settings';
import { WHEEL_STEP, boxWindow, followView, paddedView, panView, radarImageFits, rememberZoom, rememberedZoom, sameRect, wheelZooms, zoomLevel, zoomViewAt } from '@/lib/replay/map-view';
import { framesDecodeError } from '@/lib/replay/loaded-analysis';
import { createInterpolatedSampleReader, createSampleTrailReader, dominantLevel, frameCursor } from '@/lib/tactical-replay';
import type { FrameCursor, TrailPoint } from '@/lib/tactical-replay';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { timelineSeconds } from '@/lib/tactical-timeline';
import { isCalibrationUsable, radarViewRect, worldRectToRadarRect, worldToRendered } from '@/lib/tactical-transform';
import type { RadarRect } from '@/lib/tactical-transform';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { TelestratorLayer } from '@/components/tactical/spatial/telestrator';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { cn } from '@/lib/utils';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayCentreProps } from './centre';
import { loadNavRadar, loadRadarImage } from './radar/map-art';
import { PlayerTooltip, placeTooltip } from './radar/player-tooltip';
import { bakeMapPlate } from './radar/radar-bake';
import type { MapArt } from './radar/radar-bake';
import { RealPlate } from './radar/real-plate';
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

/** The real map's plate for one map; `plate` is null while it loads or when it failed. */
type RealArt = { map: string; plate: RealPlate | null; failed: boolean };

/** The game's radar images of one map. */
type OfficialArt = { map: string; upper: HTMLImageElement; lower: HTMLImageElement | null };

const GRID: MapArt = { kind: 'grid' };
const NO_FOCUS: readonly number[] = [];

/** The tag spots of one floor's pane, so laying out one pane's tags never forgets the other's. */
function spotsOf(spots: Map<RadarLevel, Map<number, number>>, level: RadarLevel): Map<number, number> {
  const known = spots.get(level);
  if (known !== undefined) return known;
  const fresh = new Map<number, number>();
  spots.set(level, fresh);
  return fresh;
}

/**
 * The 2D map: a stylized radar drawn from the map's nav mesh (or the game's
 * radar image when Studio has it), the players, their names and the grenades.
 * The wheel zooms on the cursor, a drag pans, a click highlights a player and
 * a right-click copies their setpos.
 * The canvas mirrors the clock and is `aria-hidden`; the panels say the same in text.
 */
export function RadarCentre({ mode, rounds, clock, tickOf, layers, telestrator, focus, onPickSlot, onHoverSlot, onCopySlot, followSlot, onFollowSlot, settings, barSlot }: ReplayCentreProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const layerRef = useRef<HTMLCanvasElement | null>(null);
  const styleRef = useRef<RadarStyle | null>(null);
  // One baked plate per floor, so both floors on screen do not bake each other out every frame.
  const plateRef = useRef(new Map<RadarLevel, { key: string; canvas: HTMLCanvasElement }>());
  /** Where each player was drawn, in the container's CSS pixels. */
  const drawnRef = useRef<DrawnPlayer[]>([]);
  const spotsRef = useRef(new Map<RadarLevel, Map<number, number>>());
  const samplesRef = useRef<TacticalSample[]>([]);
  const tipRef = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState<Box>({ width: 1, height: 1, dpr: 1 });
  const [navArt, setNavArt] = useState<{ map: string; art: MapArt; failed: boolean } | null>(null);
  const [officialArt, setOfficialArt] = useState<OfficialArt | null>(null);
  const source = settings.art;
  const dotScale = MAP_SIZE_SCALE[settings.dots];
  const nameScale = MAP_SIZE_SCALE[settings.names];
  const [real, setReal] = useState<RealArt | null>(null);
  const [zoomState, setZoomState] = useState<{ map: string; view: RadarRect | null } | null>(null);
  const [hover, setHover] = useState<number | null>(null);
  const [grabbing, setGrabbing] = useState(false);
  const [sightGeometry, setSightGeometry] = useState<{ map: string; result: SightGeometry } | null>(null);
  const sightMemo = useRef<{ key: string; value: FrameSight } | null>(null);

  const focused = rounds.find((round) => round.focused) ?? rounds.at(-1);
  const focusedMatchId = focused?.matchId;
  const geometry = focused?.doc.geometry;
  const map = geometry?.map ?? '';
  const drawable = geometry !== undefined && isCalibrationUsable(geometry.calibration);
  const split = geometry?.calibration.lower_altitude_max !== undefined;
  // Both floors side by side: each pane draws only what stands on its floor.
  const bothFloors = split && settings.floors === 'both';
  const imageFits = geometry !== undefined && radarImageFits(geometry.calibration);
  // Art and zoom are kept with their map, so a new map never paints under the last one's.
  const art = navArt?.map === map ? navArt.art : GRID;
  const navFailed = navArt?.map === map && navArt.failed;
  const official = imageFits && officialArt?.map === map ? officialArt : null;
  // The zoom outlives a remount, per map.
  const zoomed = zoomState?.map === map ? zoomState.view : rememberedZoom(map);
  const setZoomed = useCallback(
    (view: RadarRect | null) => {
      rememberZoom(map, view);
      setZoomState({ map, view });
    },
    [map],
  );

  useEffect(() => {
    if (map === '') return;
    let active = true;
    void loadNavRadar(map).then((model) => {
      if (active) setNavArt({ map, art: model === null ? GRID : { kind: 'nav', model }, failed: model === null });
    });
    if (imageFits) {
      void Promise.all([loadRadarImage(map, 'upper'), split ? loadRadarImage(map, 'lower') : Promise.resolve(null)]).then(([upper, lower]) => {
        if (active) setOfficialArt(upper === null ? null : { map, upper, lower });
      });
    }
    return () => {
      active = false;
    };
  }, [imageFits, map, split]);

  useEffect(() => {
    if (source !== 'real' || map === '') return;
    let active = true;
    let loaded: RealPlate | null = null;
    const plates = plateRef.current;
    setReal({ map, plate: null, failed: false });
    RealPlate.load(map).then(
      (plate) => {
        if (!active) return plate.dispose();
        loaded = plate;
        plates.clear();
        setReal({ map, plate, failed: false });
      },
      (error: unknown) => {
        console.error('2D: the real map could not be loaded', error);
        if (active) setReal({ map, plate: null, failed: true });
      },
    );
    return () => {
      active = false;
      // Drop the plate before releasing it, so no frame draws from a lost WebGL context.
      setReal(null);
      plates.clear();
      loaded?.dispose();
    };
  }, [map, source]);
  const realPlate = source === 'real' && real?.map === map ? real.plate : null;
  const realLoading = source === 'real' && (real?.map !== map || (real.plate === null && !real.failed));
  const realFailed = source === 'real' && real?.map === map && real.failed;

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

  const shownArt = useMemo<MapArt>(() => {
    if (realPlate !== null) return { kind: 'real', plate: realPlate };
    // While the real map loads, or when it failed, the official radar stands in for it.
    return source !== 'scheme' && official !== null ? { kind: 'image', upper: official.upper, lower: official.lower } : art;
  }, [art, official, realPlate, source]);

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
  const panes = useMemo(() => floorPanes(box, bothFloors ? BOTH_FLOORS : [RADAR_LEVELS.default]), [box, bothFloors]);
  // One view for every pane, so zoom and pan move both floors together.
  const projections = useMemo(() => panes.map((pane) => paneProjection(pane, view)), [panes, view]);
  const projection = projections[0];
  const paneBox = panes[0];
  const zoom = zoomLevel(view, base);
  // The plate covers the whole pane, not just the view, or the strips beside it stay black.
  const shown = useMemo(() => boxWindow(paneBox, localProjection(paneBox, projection)), [paneBox, projection]);

  // Slots mean a player only inside one match: in another match the same players are found by SteamID.
  const focusByMatch = useMemo(() => {
    const out = new Map<string, readonly number[]>();
    const ids = new Set((focused?.doc.players ?? []).filter((player) => focus.includes(player.slot)).map((player) => player.steamid64));
    for (const round of rounds) {
      if (out.has(round.matchId)) continue;
      out.set(round.matchId, round.matchId === focusedMatchId ? focus : round.doc.players.filter((player) => ids.has(player.steamid64)).map((player) => player.slot));
    }
    return out;
  }, [focus, focused, focusedMatchId, rounds]);

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
      const key = `${shownArt.kind}|${projection.size}|${box.dpr}|${shown.x},${shown.y},${shown.width},${shown.height}|${map}`;
      const known = plateRef.current.get(level);
      if (known?.key === key) return known.canvas;
      const canvas = bakeMapPlate({ art: shownArt, geometry, level, size: projection.size, dpr: box.dpr, view: shown, style });
      plateRef.current.set(level, { key, canvas });
      return canvas;
    },
    [box.dpr, geometry, map, projection.size, shown, shownArt],
  );

  const drawRound = useCallback(
    (ctx: CanvasRenderingContext2D, round: CentreRound, options: { seconds: number; style: RadarStyle; level: RadarLevel }): DrawnPlayer[] => {
      const { seconds, style, level } = options;
      const reader = readers.get(round.key);
      // Every round goes through the plate's calibration: one map, one world, whatever each demo derived.
      if (reader === undefined || geometry === undefined) return [];
      const { calibration } = geometry;
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
      const visible = { x: shown.x * projection.size, y: shown.y * projection.size, width: shown.width * projection.size, height: shown.height * projection.size };
      const roundFocus = focusByMatch.get(round.matchId) ?? NO_FOCUS;
      // With players in focus, a superposed round shows only them: forty rounds of one player stay readable.
      const alone = focus.length > 0 && !round.focused;
      const layered = alone ? layerSamples(samples, layers).filter((sample) => roundFocus.includes(sample.slot)) : layerSamples(samples, layers);
      const drawn = bothFloors ? onFloorOf(calibration, level, layered) : layered;
      const secondsOf = (at: number): number => timelineSeconds(round.timeline, at);
      let analysis: FrameAnalysis | null = null;
      if (round.focused) {
        let sight: FrameSight | null = null;
        if (sightBvh !== null) {
          const smokes = blockingSmokes({ events: round.round.events, nowSeconds, secondsOf });
          // Rays are cast once per recorded sample (8 Hz), not once per screen frame.
          const key = `${round.key}|${cursor.index}|${smokes.length}|${layers.hiddenSlots.join(',')}`;
          if (sightMemo.current?.key !== key) {
            const recorded = layerSamples(round.frames[cursor.index]?.samples ?? [], layers);
            sightMemo.current = { key, value: frameSight({ bvh: sightBvh, samples: recorded, smokes, reach: WEDGE_REACH, rays: WEDGE_RAYS }) };
          }
          sight = sightMemo.current.value;
        }
        const hits = layers.hits ? hitLines({ hits: round.round.hits ?? [], events: round.round.events, nowSeconds, secondsOf, hiddenSlots: layers.hiddenSlots }) : [];
        analysis = { sight, hits };
      }
      return drawRoundFrame(
        ctx,
        {
          calibration,
          level,
          offFloorAlpha: bothFloors ? 0 : OFF_FLOOR_ALPHA,
          size: projection.size,
          samples: drawn,
          trails: layerTrails(reader.trails(cursor), layers),
          events: round.events,
          nowSeconds,
          secondsOf,
          deadFor,
          labels: round.focused ? round.labels : new Map(),
          focus: roundFocus,
          dotScale,
          nameScale,
          utility: layers.utility && !alone,
          bomb: layers.bomb && !alone,
          style,
          analysis,
        },
        round.focused ? { visible, spots: spotsOf(spotsRef.current, level) } : null,
      );
    },
    [bothFloors, dotScale, focus, focusByMatch, geometry, layers, nameScale, projection.size, readers, shown, sightBvh, tickOf],
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
      const dominant = dominantLevel(geometry.calibration, cursor === undefined || reader === undefined ? [] : reader.samples(cursor));

      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.fillStyle = '#0b0c0e';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      const others = rounds.filter((round) => round !== focused);
      const drawn: DrawnPlayer[] = [];
      panes.forEach((pane, index) => {
        const level = bothFloors ? pane.level : dominant;
        const at = projections[index];
        ctx.save();
        if (bothFloors) {
          ctx.beginPath();
          ctx.rect(pane.x * dpr, pane.y * dpr, pane.width * dpr, pane.height * dpr);
          ctx.clip();
        }
        const baked = plate(level, style);
        const left = at.offsetX + shown.x * at.size;
        const top = at.offsetY + shown.y * at.size;
        if (baked !== null) ctx.drawImage(baked, Math.round(left * dpr), Math.round(top * dpr));
        if (others.length > 0) {
          const layer = layerRef.current ?? document.createElement('canvas');
          layerRef.current = layer;
          if (layer.width !== canvas.width) layer.width = canvas.width;
          if (layer.height !== canvas.height) layer.height = canvas.height;
          const layerCtx = layer.getContext('2d');
          if (layerCtx !== null) {
            layerCtx.setTransform(1, 0, 0, 1, 0, 0);
            layerCtx.clearRect(0, 0, layer.width, layer.height);
            layerCtx.setTransform(dpr, 0, 0, dpr, at.offsetX * dpr, at.offsetY * dpr);
            for (const round of others) drawRound(layerCtx, round, { seconds, style, level });
            ctx.globalAlpha = OVERLAY_ALPHA;
            ctx.drawImage(layer, 0, 0);
            ctx.globalAlpha = 1;
          }
        }
        ctx.setTransform(dpr, 0, 0, dpr, at.offsetX * dpr, at.offsetY * dpr);
        for (const player of drawRound(ctx, focused, { seconds, style, level })) drawn.push({ ...player, x: player.x + at.offsetX, y: player.y + at.offsetY });
        ctx.restore();
      });
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      if (bothFloors && panes.length > 1) {
        const [first, second] = panes;
        ctx.fillStyle = style.text;
        ctx.globalAlpha = 0.18;
        if (second.x > first.x) ctx.fillRect((first.x + first.width) * dpr, 0, (second.x - first.x - first.width) * dpr, canvas.height);
        else ctx.fillRect(0, (first.y + first.height) * dpr, canvas.width, (second.y - first.y - first.height) * dpr);
        ctx.globalAlpha = 1;
      }
      drawnRef.current = drawn;

      const tip = tipRef.current;
      const hovered = hover === null ? undefined : drawn.find((player) => player.slot === hover);
      if (tip !== null && hovered !== undefined) placeTooltip(tip, hovered.x, hovered.y, box);
      const followed = followSlot === null ? undefined : samplesRef.current.find((sample) => sample.slot === followSlot);
      if (followed !== undefined) {
        const at = worldToRendered(geometry.calibration, followed.x, followed.y, 1);
        const next = followView({ view, base, point: { x: at.x, y: at.y } });
        if (!sameRect(next, view)) setZoomed(next);
      }
    },
    [base, bothFloors, box, drawRound, drawable, focused, followSlot, geometry, hover, panes, plate, projections, readers, rounds, setZoomed, shown, tickOf, view],
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
      plateRef.current.clear();
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

  // Not passive: a turn that zooms keeps the page still; zooming out of the whole map scrolls the page.
  const zoomStateRef = useRef({ panes, view, base, setZoomed });
  zoomStateRef.current = { panes, view, base, setZoomed };
  useEffect(() => {
    const container = containerRef.current;
    if (container === null) return;
    const onWheel = (event: WheelEvent): void => {
      const state = zoomStateRef.current;
      if (!wheelZooms(state.view, state.base, event.deltaY)) return;
      event.preventDefault();
      const rect = container.getBoundingClientRect();
      // The wheel zooms on the cursor inside its floor's pane; both panes share the view.
      const hit = paneAt(state.panes, event.clientX - rect.left, event.clientY - rect.top);
      const pane = hit?.pane ?? state.panes[0];
      const cursor = hit === null ? { x: pane.width / 2, y: pane.height / 2 } : { x: hit.x, y: hit.y };
      const next = zoomViewAt({ box: pane, view: state.view, base: state.base, cursor, factor: WHEEL_STEP ** -event.deltaY });
      state.setZoomed(sameRect(next, state.base) ? null : next);
    };
    container.addEventListener('wheel', onWheel, { passive: false });
    return () => container.removeEventListener('wheel', onWheel);
  }, []);

  const playerAt = (x: number, y: number): number | null => {
    const reach = discRadius(projection.size, dotScale) + 6;
    let best: { slot: number; distance: number } | null = null;
    for (const player of drawnRef.current) {
      if (!player.alive) continue;
      const distance = Math.hypot(player.x - x, player.y - y);
      if (distance <= reach && (best === null || distance < best.distance)) best = { slot: player.slot, distance };
    }
    return best?.slot ?? null;
  };

  useEffect(() => onHoverSlot(hover), [hover, onHoverSlot]);

  const dragRef = useRef<{ x: number; y: number; moved: boolean; id: number } | null>(null);
  const local = (event: { clientX: number; clientY: number }): { x: number; y: number } => {
    const rect = containerRef.current?.getBoundingClientRect();
    return rect === undefined ? { x: 0, y: 0 } : { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };

  const zoomBy = (factor: number): void => {
    const next = zoomViewAt({ box: paneBox, view, base, cursor: { x: paneBox.width / 2, y: paneBox.height / 2 }, factor });
    setZoomed(sameRect(next, base) ? null : next);
  };

  let cursorClass = 'cursor-grab';
  if (grabbing) cursorClass = 'cursor-grabbing';
  else if (hover !== null) cursorClass = 'cursor-pointer';
  const label = mode === 'multi' ? `${rounds.length} rondas superpuestas desde el final del tiempo de compra` : `Mapa de la ronda ${focused?.round.number ?? ''}`;
  const hovered = hover === null ? null : drawnRef.current.find((player) => player.slot === hover);
  const hoveredSample = hover === null ? undefined : samplesRef.current.find((sample) => sample.slot === hover);
  const decodeError = focused === undefined ? null : framesDecodeError(focused.frames);
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
            const next = panView({ box: paneBox, view, base, dx, dy });
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
          if (slot !== null) onPickSlot(slot, event.shiftKey);
        }}
        onPointerLeave={() => setHover(null)}
        onContextMenu={(event) => {
          const at = local(event);
          const slot = playerAt(at.x, at.y);
          if (slot === null) return;
          event.preventDefault();
          onCopySlot(slot);
        }}
        onDoubleClick={() => setZoomed(null)}
      >
        <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
      </div>
      {drawable && telestrator !== null && geometry !== undefined && !bothFloors ? (
        <TelestratorLayer calibration={geometry.calibration} projection={projection} shown={telestrator.shown} draft={telestrator.draft} />
      ) : null}
      {drawable && geometry !== undefined && bothFloors
        ? panes.map((pane, index) => (
            <div key={pane.level} className="pointer-events-none absolute overflow-hidden" style={{ left: pane.x, top: pane.y, width: pane.width, height: pane.height }}>
              {telestrator === null ? null : (
                <TelestratorLayer
                  calibration={geometry.calibration}
                  projection={localProjection(pane, projections[index])}
                  shown={telestrator.shown}
                  draft={telestrator.draft}
                  level={pane.level}
                />
              )}
              <span className="absolute top-2 left-2 rounded-sm bg-surface-0/90 px-1.5 py-0.5 text-meta text-fg-2">{LEVEL_LABEL[pane.level]}</span>
            </div>
          ))
        : null}
      {hovered === undefined || hovered === null || hoveredSample === undefined || grabbing ? null : (
        <PlayerTooltip
          ref={(element) => {
            tipRef.current = element;
            if (element !== null) placeTooltip(element, hovered.x, hovered.y, box);
          }}
          sample={hoveredSample}
          name={hoveredName}
        />
      )}

      <div className="pointer-events-none absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] flex-wrap items-end gap-2">
        {realLoading ? (
          <p role="status" className="pointer-events-auto rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            Cargando el mapa real de CS2…
          </p>
        ) : null}
        {realFailed ? (
          <p className="pointer-events-auto max-w-72 rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            No se pudo cargar el mapa real de tu instalación de CS2: se muestra el radar oficial.
          </p>
        ) : null}
        {layers.sight && sightGeometry?.map === map && sightGeometry.result.kind === 'failed' ? (
          <p className="pointer-events-auto max-w-72 rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">{sightGeometry.result.message}</p>
        ) : null}
        {layers.sight && sightGeometry?.map !== map ? (
          <p role="status" className="pointer-events-auto rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            Leyendo las paredes del mapa…
          </p>
        ) : null}
        {navFailed && (source === 'scheme' || official === null) ? (
          <p className="pointer-events-auto max-w-72 rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            No se pudo leer el mapa de tu instalación de CS2: se dibuja con las posiciones de la demo.
          </p>
        ) : null}
      </div>

      {barSlot === null
        ? null
        : createPortal(<MapZoomControls zoom={zoom} zoomed={zoomed !== null} onZoomBy={zoomBy} onReset={() => setZoomed(null)} className="border-0 bg-transparent p-0" />, barSlot)}

      {drawable ? null : (
        <p className="absolute inset-0 grid place-items-center px-6 text-center text-body-sm leading-5 text-fg-2">
          Este mapa no tiene una calibración de radar utilizable, así que no se puede dibujar el Replay. Los eventos siguen en la lista.
        </p>
      )}
      {drawable && decodeError !== null ? (
        <p className="absolute inset-x-0 bottom-12 mx-auto w-fit rounded-md border border-border-strong bg-surface-0/90 px-3 py-2 text-body-sm text-fg-1">{decodeError}</p>
      ) : null}
      {drawable && decodeError === null && focused !== undefined && focused.frames.length === 0 ? (
        <p className="absolute inset-x-0 bottom-12 mx-auto w-fit rounded-md border border-border-strong bg-surface-0/90 px-3 py-2 text-body-sm text-fg-1">
          Esta ronda no tiene posiciones guardadas: el mapa queda vacío, los eventos no.
        </p>
      ) : null}
    </div>
  );
}
