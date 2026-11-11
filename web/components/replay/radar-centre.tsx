'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { RADAR_LEVELS } from '@/lib/api/tactical';
import type { RadarLevel, TacticalSample } from '@/lib/api/tactical';
import { OFF_FLOOR_ALPHA } from '@/lib/analysis/floors';
import { createFloorTimeline } from '@/lib/replay/floor-assign';
import type { FloorTimeline } from '@/lib/replay/floor-assign';
import {
  BOTH_FLOORS,
  WHOLE_FLOORS,
  floorExtent,
  floorRectsOf,
  followFloors,
  isWholeView,
  layoutFloors,
  localProjection,
  nearestPane,
  panFloors,
  paneProjection,
  rememberFloorView,
  rememberedFloorView,
  sameFloorView,
  zoomFloors,
} from '@/lib/replay/floor-split';
import type { FloorPane, FloorRects, FloorView } from '@/lib/replay/floor-split';
import { hitLines, recentDamage } from '@/lib/replay/hit-lines';
import { DOT_SIZE_SCALE, MAP_SIZE_SCALE, ZOOM_SPEED } from '@/lib/replay/map-settings';
import { WHEEL_STEP, boxWindow, paddedView } from '@/lib/replay/map-view';
import { floorBounds, floorPolygons } from '@/lib/replay/nav-radar';
import { framesDecodeError } from '@/lib/replay/loaded-analysis';
import { createInterpolatedSampleReader, createSampleTrailReader, dominantLevel, frameCursor } from '@/lib/tactical-replay';
import type { FrameCursor, TrailPoint } from '@/lib/tactical-replay';
import type { RadarProjection } from '@/lib/tactical-spatial';
import type { MapPane } from './gif-export';
import { timelineSeconds } from '@/lib/tactical-timeline';
import { throwAt, throwMarkers } from '@/lib/replay/throw-hit';
import type { ThrowMarker } from '@/lib/replay/throw-hit';
import { utilityVisuals } from '@/lib/replay/three/utility-visuals';
import { isCalibrationUsable, levelForAltitude, occupancyBounds, radarViewRect, worldRectToRadarRect, worldToRendered } from '@/lib/tactical-transform';
import type { RadarRect } from '@/lib/tactical-transform';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { readRadarStyle } from '@/components/tactical/radar-style';
import { TelestratorLayer } from '@/components/tactical/spatial/telestrator';
import { ZoneLayer } from './zone-layer';
import { MapZoomControls } from '@/components/analysis/map-zoom-controls';
import { cn } from '@/lib/utils';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayCentreProps } from './centre';
import { loadNavRadar } from './radar/map-art';
import { PlayerTooltip, placeTooltip } from './radar/player-tooltip';
import { bakeMapPlate } from './radar/radar-bake';
import type { MapArt } from './radar/radar-bake';
import { discRadius, drawRoundFrame, preloadGrenadeIcons } from './radar/radar-round';
import type { DrawnPlayer } from './radar/radar-round';

type Box = { width: number; height: number; dpr: number };

/** Superposed rounds sit under the focused one at this strength. */
const OVERLAY_ALPHA = 0.42;
/** Air round the nav mesh, as a share of its longer side. */
const VIEW_MARGIN = 0.03;
/** A press that moves less than this is a click on a player, not a drag. */
const CLICK_SLOP = 4;
/** How close a click lands to a throw point to copy it, in CSS pixels. */
const THROW_REACH = 9;
/** How far back the trails reach. */
const TRAIL_SECONDS = 2;

type Readers = {
  samples: (cursor: FrameCursor) => TacticalSample[];
  trails: (cursor: FrameCursor) => ReadonlyMap<number, readonly TrailPoint[]>;
};

/** Where one view puts each floor's pane, and the window of the radar each pane's plate covers. */
type Framing = { projections: RadarProjection[]; shown: RadarRect[] };
type Painted = { drawn: DrawnPlayer[]; throws: ThrowMarker[]; samples: TacticalSample[]; levels: ReadonlyMap<number, RadarLevel>; view: FloorView; panes: MapPane[] };

const GRID: MapArt = { kind: 'grid' };
const NO_FOCUS: readonly number[] = [];
const NO_HURT: ReadonlyMap<number, number> = new Map();
const NO_LEVELS: ReadonlyMap<number, RadarLevel> = new Map();
const NO_MARKS: readonly [number, number][] = [];
const ABOVE_LABEL = 'Sup.';
const BELOW_LABEL = 'Inf.';

/** The tag spots of one floor's pane, so laying out one pane's tags never forgets the other's. */
function spotsOf(spots: Map<RadarLevel, Map<number, number>>, level: RadarLevel): Map<number, number> {
  const known = spots.get(level);
  if (known !== undefined) return known;
  const fresh = new Map<number, number>();
  spots.set(level, fresh);
  return fresh;
}

/** The unit direction from one pane toward the other, for the stubs of lines that cross to it. */
function directionTo(from: FloorPane, to: FloorPane): { dx: number; dy: number } {
  const dx = to.x + to.width / 2 - (from.x + from.width / 2);
  const dy = to.y + to.height / 2 - (from.y + from.height / 2);
  const length = Math.hypot(dx, dy);
  return length > 0 ? { dx: dx / length, dy: dy / length } : { dx: 1, dy: 0 };
}

/**
 * The 2D map: a stylized radar drawn from the map's nav mesh (or the game's
 * radar image when Studio has it), the players, their names and the grenades.
 * On a two-floor map each floor has its own block, cropped to its extent.
 * The wheel zooms on the cursor, a drag pans, a click highlights a player and
 * a right-click copies their setpos; a click on a throw point copies the throw's.
 * The canvas mirrors the clock and is `aria-hidden`; the panels say the same in text.
 */
export function RadarCentre({ mode, rounds, clock, tickOf, telestrator, zones, focus, onPickSlot, onHoverSlot, onCopySlot, onCopyThrow, followSlot, onFollowSlot, settings, barSlot, captureRef, speakingAt }: ReplayCentreProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const layerRef = useRef<HTMLCanvasElement | null>(null);
  const styleRef = useRef<RadarStyle | null>(null);
  // One baked plate per floor, so both floors on screen do not bake each other out every frame.
  const plateRef = useRef(new Map<RadarLevel, { key: string; canvas: HTMLCanvasElement }>());
  /** Where each player was drawn, in the container's CSS pixels. */
  const drawnRef = useRef<DrawnPlayer[]>([]);
  /** Where each throw point of the focused round was drawn, in the container's CSS pixels. */
  const throwsRef = useRef<ThrowMarker[]>([]);
  const spotsRef = useRef(new Map<RadarLevel, Map<number, number>>());
  const samplesRef = useRef<TacticalSample[]>([]);
  const dominantRef = useRef<RadarLevel>(RADAR_LEVELS.default);
  const tipRef = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState<Box>({ width: 1, height: 1, dpr: 1 });
  const [navArt, setNavArt] = useState<{ map: string; art: MapArt; failed: boolean } | null>(null);
  const dotScale = DOT_SIZE_SCALE[settings.dots];
  const nameScale = MAP_SIZE_SCALE[settings.names];
  const show = settings.show;
  const [zoomState, setZoomState] = useState<{ map: string; view: FloorView } | null>(null);
  const [hover, setHover] = useState<number | null>(null);
  const [overThrow, setOverThrow] = useState(false);
  const [grabbing, setGrabbing] = useState(false);

  const focused = rounds.find((round) => round.focused) ?? rounds.at(-1);
  const focusedMatchId = focused?.matchId;
  const geometry = focused?.doc.geometry;
  const map = geometry?.map ?? '';
  const drawable = geometry !== undefined && isCalibrationUsable(geometry.calibration);
  const split = geometry?.calibration.lower_altitude_max !== undefined;
  // Separar pisos: each floor in its own block, and each block draws only the players standing on its floor.
  const bothFloors = split && settings.floors === 'both';
  // Art and zoom are kept with their map, so a new map never paints under the last one's.
  const art = navArt?.map === map ? navArt.art : GRID;
  const navFailed = navArt?.map === map && navArt.failed;
  // The zoom outlives a remount, per map.
  const view = zoomState?.map === map ? zoomState.view : rememberedFloorView(map);
  // Pointer moves can arrive faster than renders; each drag step builds on the last one, not on a stale render.
  const viewRef = useRef(view);
  viewRef.current = view;
  const setView = useCallback(
    (next: FloorView) => {
      viewRef.current = next;
      rememberFloorView(map, next);
      setZoomState({ map, view: next });
    },
    [map],
  );

  useEffect(() => {
    if (map === '') return;
    let active = true;
    void loadNavRadar(map).then((model) => {
      if (active) setNavArt({ map, art: model === null ? GRID : { kind: 'nav', model }, failed: model === null });
    });
    return () => {
      active = false;
    };
  }, [map]);

  // The whole map's window: what one floor shows, and what every floor of a one-floor map is cropped to.
  const base = useMemo<RadarRect>(() => {
    if (geometry === undefined || !drawable) return { x: 0, y: 0, width: 1, height: 1 };
    if (art.kind === 'nav') {
      const { bounds } = art.model;
      const rect = worldRectToRadarRect(geometry.calibration, { minX: bounds.minX, minY: bounds.minY, maxX: bounds.maxX, maxY: bounds.maxY }, 1);
      return paddedView(rect, VIEW_MARGIN);
    }
    return radarViewRect(geometry);
  }, [art, drawable, geometry]);
  // Each floor cropped to its own walkable extent (or, without the nav mesh, to where it was played).
  const rects = useMemo<FloorRects>(() => {
    if (geometry === undefined || !drawable || !bothFloors) return { default: base, lower: base };
    const fallback = floorRectsOf(geometry, NO_MARKS);
    if (art.kind !== 'nav') return fallback;
    const { calibration } = geometry;
    const of = (level: RadarLevel): RadarRect => {
      const played = occupancyBounds({ ...geometry, levels: geometry.levels.filter((entry) => entry.name === level) });
      const nav = floorBounds(floorPolygons(art.model, calibration.lower_altitude_max, level));
      return floorExtent({ calibration, nav, played }) ?? fallback[level];
    };
    return { default: of(RADAR_LEVELS.default), lower: of(RADAR_LEVELS.lower) };
  }, [art, base, bothFloors, drawable, geometry]);
  const layout = useMemo(() => layoutFloors({ box, rects, levels: bothFloors ? BOTH_FLOORS : [RADAR_LEVELS.default] }), [box, rects, bothFloors]);
  const { panes } = layout;
  // One view for every pane, so zoom and pan move both floors together.
  const framingOf = useCallback(
    (frameView: FloorView): Framing => {
      const projections = panes.map((pane) => paneProjection({ layout, pane, view: frameView }));
      // The plate covers the whole pane, not just the view, or the strips beside it stay black.
      return { projections, shown: panes.map((pane, index) => boxWindow(pane, localProjection(pane, projections[index]))) };
    },
    [layout, panes],
  );
  const framing = useMemo(() => framingOf(view), [framingOf, view]);
  const { projections } = framing;
  const projection = projections[0];
  const zoomed = !isWholeView(view);

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

  // The floor of each player over each round, settled once with the dead band round the altitude cut.
  const timelines = useMemo(() => {
    const out = new Map<string, FloorTimeline>();
    if (geometry === undefined) return out;
    for (const round of rounds) out.set(round.key, createFloorTimeline(geometry.calibration, round.frames));
    return out;
  }, [geometry, rounds]);

  // The floor each thrower stood on when a grenade left the hand, by the grenade's place in its round's events.
  const throwLevels = useMemo(() => {
    const out = new Map<string, ReadonlyMap<number, RadarLevel>>();
    for (const round of rounds) {
      const timeline = timelines.get(round.key);
      const levels = new Map<number, RadarLevel>();
      round.events.forEach((entry, index) => {
        const { actor_slot: slot, throw: thrown } = entry.event;
        const cursor = thrown === undefined || slot === undefined ? undefined : frameCursor(round.frames, thrown.tick);
        if (timeline === undefined || cursor === undefined || slot === undefined) return;
        const level = timeline.levelAt(slot, cursor.index, RADAR_LEVELS.default);
        levels.set(index, level);
      });
      out.set(round.key, levels);
    }
    return out;
  }, [rounds, timelines]);

  const readers = useMemo(() => {
    const out = new Map<string, Readers>();
    for (const round of rounds) {
      const timeline = timelines.get(round.key);
      out.set(round.key, {
        samples: createInterpolatedSampleReader(round.frames),
        trails: createSampleTrailReader(round.frames, round.timeline.tickrate, TRAIL_SECONDS, (slot, index) => timeline?.levelAt(slot, index, RADAR_LEVELS.default) ?? RADAR_LEVELS.default),
      });
    }
    return out;
  }, [rounds, timelines]);

  // The floor each of `samples` stands on at `cursor`, by its round's timeline.
  const levelsOf = useCallback(
    (round: CentreRound, cursor: FrameCursor | undefined, samples: readonly TacticalSample[]): ReadonlyMap<number, RadarLevel> => {
      if (geometry === undefined || cursor === undefined) return NO_LEVELS;
      const timeline = timelines.get(round.key);
      const out = new Map<number, RadarLevel>();
      for (const sample of samples) out.set(sample.slot, timeline?.levelAt(sample.slot, cursor.index, levelForAltitude(geometry.calibration, sample.z)) ?? levelForAltitude(geometry.calibration, sample.z));
      return out;
    },
    [geometry, timelines],
  );

  const plate = useCallback(
    (level: RadarLevel, style: RadarStyle, at: Framing, index: number): HTMLCanvasElement | null => {
      if (geometry === undefined) return null;
      const shown = at.shown[index];
      const { size } = at.projections[index];
      const key = `${art.kind}|${size}|${box.dpr}|${shown.x},${shown.y},${shown.width},${shown.height}|${map}`;
      const known = plateRef.current.get(level);
      if (known?.key === key) return known.canvas;
      const canvas = bakeMapPlate({ art, geometry, level, size, dpr: box.dpr, view: shown, style });
      plateRef.current.set(level, { key, canvas });
      return canvas;
    },
    [box.dpr, geometry, map, art],
  );

  const drawRound = useCallback(
    (ctx: CanvasRenderingContext2D, round: CentreRound, options: { seconds: number; style: RadarStyle; level: RadarLevel; at: Framing; index: number; away: { dx: number; dy: number } | null }): DrawnPlayer[] => {
      const { seconds, style, level, index, away } = options;
      const shown = options.at.shown[index];
      const { size } = options.at.projections[index];
      const reader = readers.get(round.key);
      // Every round goes through the plate's calibration: one map, one world, whatever each demo derived.
      if (reader === undefined || geometry === undefined) return [];
      const { calibration } = geometry;
      const tick = tickOf(round, seconds);
      const cursor = frameCursor(round.frames, tick);
      if (cursor === undefined) return [];
      const samples = reader.samples(cursor);
      const nowSeconds = timelineSeconds(round.timeline, tick);
      const deadFor = new Map<number, number>();
      for (const player of round.round.players) {
        if (player.death_tick !== undefined && player.death_tick <= tick) deadFor.set(player.slot, nowSeconds - timelineSeconds(round.timeline, player.death_tick));
      }
      const visible = { x: shown.x * size, y: shown.y * size, width: shown.width * size, height: shown.height * size };
      const roundFocus = focusByMatch.get(round.matchId) ?? NO_FOCUS;
      // With players in focus, a superposed round shows only them: forty rounds of one player stay readable.
      const alone = focus.length > 0 && !round.focused;
      const layered = alone ? samples.filter((sample) => roundFocus.includes(sample.slot)) : samples;
      const levels = levelsOf(round, cursor, layered);
      const onThisFloor = (sample: TacticalSample): boolean => levels.get(sample.slot) === level;
      const drawn = bothFloors ? layered.filter(onThisFloor) : layered;
      const across =
        bothFloors && away !== null
          ? { ...away, label: level === RADAR_LEVELS.default ? BELOW_LABEL : ABOVE_LABEL, slots: new Set(layered.filter((sample) => !onThisFloor(sample)).map((sample) => sample.slot)) }
          : undefined;
      const secondsOf = (at: number): number => timelineSeconds(round.timeline, at);
      const hits = round.focused && show.bullets ? hitLines({ hits: round.round.hits ?? [], events: round.round.events, nowSeconds, secondsOf }) : [];
      return drawRoundFrame(
        ctx,
        {
          calibration,
          level,
          offFloorAlpha: bothFloors ? 0 : OFF_FLOOR_ALPHA,
          size,
          samples: drawn,
          everyone: layered,
          levels,
          throwLevels: throwLevels.get(round.key),
          across,
          trails: reader.trails(cursor),
          events: round.events,
          nowSeconds,
          secondsOf,
          deadFor,
          labels: round.focused ? round.labels : new Map(),
          focus: roundFocus,
          dotScale,
          nameScale,
          utility: !alone,
          bomb: !alone,
          show,
          hurt: show.damage ? recentDamage({ hits: round.round.hits ?? [], nowSeconds, secondsOf }) : NO_HURT,
          style,
          hits,
          speaking: round.focused && speakingAt !== undefined ? speakingAt(round, tick) : undefined,
        },
        round.focused ? { visible, spots: spotsOf(spotsRef.current, level) } : null,
      );
    },
    [bothFloors, dotScale, focus, focusByMatch, geometry, levelsOf, nameScale, readers, show, speakingAt, throwLevels, tickOf],
  );

  // With Seguir on, the view that keeps the followed player in it, on the pane of the floor they stand on.
  const followFrom = useCallback(
    (samples: readonly TacticalSample[], levels: ReadonlyMap<number, RadarLevel>, from: FloorView): FloorView => {
      const followed = followSlot === null ? undefined : samples.find((sample) => sample.slot === followSlot);
      if (followed === undefined || geometry === undefined) return from;
      const at = worldToRendered(geometry.calibration, followed.x, followed.y, 1);
      const level = bothFloors ? (levels.get(followed.slot) ?? levelForAltitude(geometry.calibration, followed.z)) : RADAR_LEVELS.default;
      const pane = panes.find((entry) => entry.level === level) ?? panes[0];
      return followFloors({ pane, view: from, point: { x: at.x, y: at.y } });
    },
    [bothFloors, followSlot, geometry, panes],
  );

  // Draws one moment on a canvas the size of the visible one: the screen, or a GIF frame moved on from `gifView`.
  const paint = useCallback(
    (canvas: HTMLCanvasElement, seconds: number, gifView?: FloorView): Painted | null => {
      if (!drawable || geometry === undefined || focused === undefined) return null;
      const ctx = canvas.getContext('2d', { alpha: false });
      if (ctx === null) return null;
      const style = styleRef.current ?? readRadarStyle(canvas);
      styleRef.current = style;
      const { dpr } = box;
      const reader = readers.get(focused.key);
      const cursor = frameCursor(focused.frames, tickOf(focused, seconds));
      const samples = cursor === undefined || reader === undefined ? [] : reader.samples(cursor);
      const levels = levelsOf(focused, cursor, samples);
      const rawLevel = (sample: TacticalSample): RadarLevel => levels.get(sample.slot) ?? levelForAltitude(geometry.calibration, sample.z);
      const dominant = dominantLevel(geometry.calibration, samples, { previous: dominantRef.current, levelOf: rawLevel });
      // A GIF frame is not the screen: only the screen's own frames move the floor the one-floor plate shows.
      if (gifView === undefined) dominantRef.current = dominant;
      const frameView = gifView === undefined ? view : followFrom(samples, levels, gifView);
      const at = sameFloorView(frameView, view) ? framing : framingOf(frameView);

      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.fillStyle = '#0b0c0e';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      const others = rounds.filter((round) => round !== focused);
      const drawn: DrawnPlayer[] = [];
      const throws: ThrowMarker[] = [];
      // The throw points are drawn with the arcs, so they exist only while trajectories show.
      const visuals = show.trajectories ? utilityVisuals(focused.events, timelineSeconds(focused.timeline, tickOf(focused, seconds)), (at) => timelineSeconds(focused.timeline, at)) : [];
      panes.forEach((pane, index) => {
        const level = bothFloors ? pane.level : dominant;
        const paneAtView = at.projections[index];
        const other = panes[1 - index];
        const away = bothFloors && other !== undefined ? directionTo(pane, other) : null;
        ctx.save();
        if (bothFloors) {
          ctx.beginPath();
          ctx.rect(pane.x * dpr, pane.y * dpr, pane.width * dpr, pane.height * dpr);
          ctx.clip();
        }
        const baked = plate(level, style, at, index);
        const left = paneAtView.offsetX + at.shown[index].x * paneAtView.size;
        const top = paneAtView.offsetY + at.shown[index].y * paneAtView.size;
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
            layerCtx.setTransform(dpr, 0, 0, dpr, paneAtView.offsetX * dpr, paneAtView.offsetY * dpr);
            for (const round of others) drawRound(layerCtx, round, { seconds, style, level, at, index, away });
            ctx.globalAlpha = OVERLAY_ALPHA;
            ctx.drawImage(layer, 0, 0);
            ctx.globalAlpha = 1;
          }
        }
        ctx.setTransform(dpr, 0, 0, dpr, paneAtView.offsetX * dpr, paneAtView.offsetY * dpr);
        for (const player of drawRound(ctx, focused, { seconds, style, level, at, index, away })) drawn.push({ ...player, x: player.x + paneAtView.offsetX, y: player.y + paneAtView.offsetY });
        const markers = throwMarkers({ visuals, calibration: geometry.calibration, size: paneAtView.size, level, offFloorAlpha: bothFloors ? 0 : OFF_FLOOR_ALPHA, throwLevels: throwLevels.get(focused.key) });
        for (const marker of markers) throws.push({ ...marker, x: marker.x + paneAtView.offsetX, y: marker.y + paneAtView.offsetY });
        ctx.restore();
      });
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      return { drawn, throws, samples, levels, view: frameView, panes: panes.map((pane, index) => ({ level: bothFloors ? pane.level : undefined, projection: at.projections[index] })) };
    },
    [bothFloors, box, drawRound, drawable, followFrom, focused, framing, framingOf, geometry, levelsOf, panes, plate, readers, rounds, show.trajectories, throwLevels, tickOf, view],
  );

  const render = useCallback(
    (seconds: number) => {
      const canvas = canvasRef.current;
      if (canvas === null || geometry === undefined) return;
      const painted = paint(canvas, seconds);
      if (painted === null) return;
      const { drawn } = painted;
      drawnRef.current = drawn;
      throwsRef.current = painted.throws;
      samplesRef.current = painted.samples;

      const tip = tipRef.current;
      const hovered = hover === null ? undefined : drawn.find((player) => player.slot === hover);
      if (tip !== null && hovered !== undefined) placeTooltip(tip, hovered.x, hovered.y, box);
      const next = followFrom(painted.samples, painted.levels, view);
      if (!sameFloorView(next, view)) setView(next);
    },
    [box, followFrom, geometry, hover, paint, setView, view],
  );

  useEffect(() => {
    if (captureRef === undefined) return;
    if (!drawable || geometry === undefined) {
      captureRef.current = null;
      return;
    }
    const frame = document.createElement('canvas');
    const { calibration } = geometry;
    // Each GIF frame moves on from the last one's view, as the screen does while it follows.
    let last = view;
    captureRef.current = {
      box: { width: box.width, height: box.height },
      calibration,
      token: (name) => (canvasRef.current === null ? '' : getComputedStyle(canvasRef.current).getPropertyValue(name).trim()),
      fontFamily: styleRef.current?.fontFamily ?? 'sans-serif',
      paint: (seconds) => {
        const visible = canvasRef.current;
        if (visible === null) return null;
        if (frame.width !== visible.width) frame.width = visible.width;
        if (frame.height !== visible.height) frame.height = visible.height;
        const painted = paint(frame, seconds, last);
        if (painted === null) return null;
        last = painted.view;
        return { canvas: frame, panes: painted.panes };
      },
    };
    return () => {
      captureRef.current = null;
    };
  }, [box.height, box.width, captureRef, drawable, geometry, paint, view]);

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
  const zoomSpeed = ZOOM_SPEED[settings.zoom];
  const zoomStateRef = useRef({ panes, view, setView, zoomSpeed });
  zoomStateRef.current = { panes, view, setView, zoomSpeed };
  useEffect(() => {
    const container = containerRef.current;
    if (container === null) return;
    const onWheel = (event: WheelEvent): void => {
      const state = zoomStateRef.current;
      if (!(event.deltaY < 0 || (event.deltaY > 0 && state.view.zoom > 1))) return;
      event.preventDefault();
      const rect = container.getBoundingClientRect();
      // The wheel zooms on the cursor inside the pane it is over (the closest one in the gap); both panes share the view.
      const hit = nearestPane(state.panes, event.clientX - rect.left, event.clientY - rect.top);
      const cursor = { x: Math.min(Math.max(hit.x, 0), hit.pane.width), y: Math.min(Math.max(hit.y, 0), hit.pane.height) };
      state.setView(zoomFloors({ pane: hit.pane, view: state.view, cursor, factor: WHEEL_STEP ** (-event.deltaY * state.zoomSpeed) }));
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

  const throwAtPoint = (x: number, y: number): number | null => throwAt({ markers: throwsRef.current, x, y, reach: THROW_REACH });

  useEffect(() => onHoverSlot(hover), [hover, onHoverSlot]);

  const dragRef = useRef<{ x: number; y: number; moved: boolean; id: number; level: RadarLevel } | null>(null);
  const local = (event: { clientX: number; clientY: number }): { x: number; y: number } => {
    const rect = containerRef.current?.getBoundingClientRect();
    return rect === undefined ? { x: 0, y: 0 } : { x: event.clientX - rect.left, y: event.clientY - rect.top };
  };

  const zoomBy = (factor: number): void => {
    const pane = panes[0];
    setView(zoomFloors({ pane, view, cursor: { x: pane.width / 2, y: pane.height / 2 }, factor }));
  };

  let cursorClass = 'cursor-grab';
  if (grabbing) cursorClass = 'cursor-grabbing';
  else if (hover !== null || overThrow) cursorClass = 'cursor-pointer';
  const floorsNote = bothFloors ? ' Piso superior a la izquierda e inferior a la derecha.' : '';
  const label = mode === 'multi' ? `${rounds.length} rondas superpuestas desde el final del tiempo de compra${floorsNote}` : `Mapa de la ronda ${focused?.round.number ?? ''}${floorsNote}`;
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
          dragRef.current = { ...at, moved: false, id: event.pointerId, level: nearestPane(panes, at.x, at.y).pane.level };
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
            setOverThrow(false);
            drag.x = at.x;
            drag.y = at.y;
            const pane = panes.find((entry) => entry.level === drag.level) ?? panes[0];
            setView(panFloors({ pane, view: viewRef.current, dx, dy }));
            return;
          }
          const slot = playerAt(at.x, at.y);
          setHover((current) => (current === slot ? current : slot));
          setOverThrow(slot === null && onCopyThrow !== undefined && throwAtPoint(at.x, at.y) !== null);
        }}
        onPointerUp={(event) => {
          const drag = dragRef.current;
          dragRef.current = null;
          setGrabbing(false);
          if (drag === null || drag.moved) return;
          const at = local(event);
          const slot = playerAt(at.x, at.y);
          if (slot !== null) {
            onPickSlot(slot, event.shiftKey);
            return;
          }
          const thrown = throwAtPoint(at.x, at.y);
          const entry = thrown === null ? undefined : focused?.events[thrown];
          if (entry !== undefined && onCopyThrow !== undefined) onCopyThrow(entry.event);
        }}
        onPointerLeave={() => {
          setHover(null);
          setOverThrow(false);
        }}
        onContextMenu={(event) => {
          const at = local(event);
          const slot = playerAt(at.x, at.y);
          if (slot === null) return;
          event.preventDefault();
          onCopySlot(slot);
        }}
        onDoubleClick={() => setView(WHOLE_FLOORS)}
      >
        <canvas ref={canvasRef} aria-hidden="true" className="block h-full w-full" />
      </div>
      {drawable && geometry !== undefined && !bothFloors && zones !== undefined && zones.length > 0 ? (
        <ZoneLayer geometry={geometry} projection={projection} zones={zones} />
      ) : null}
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
              {zones === undefined || zones.length === 0 ? null : (
                <ZoneLayer geometry={geometry} projection={localProjection(pane, projections[index])} zones={zones} level={pane.level} />
              )}
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
        {navFailed ? (
          <p className="pointer-events-auto max-w-72 rounded-md bg-surface-0/90 px-2 py-1 text-meta text-fg-2">
            No se pudo leer el mapa de tu instalación de CS2: se dibuja con las posiciones de la demo.
          </p>
        ) : null}
      </div>

      {barSlot === null
        ? null
        : createPortal(<MapZoomControls zoom={view.zoom} zoomed={zoomed} onZoomBy={zoomBy} onReset={() => setView(WHOLE_FLOORS)} className="border-0 bg-transparent p-0" />, barSlot)}

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
