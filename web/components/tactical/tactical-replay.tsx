'use client';

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import { Radar } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type {
  RadarLevel,
  TacticalDocument,
  TacticalFrame,
  TacticalRound,
  TacticalSide,
} from '@/lib/api/tactical';
import {
  SEEK_STEP_SECONDS,
  TRAIL_SECONDS,
  advanceTransport,
  clampToTimeline,
  roundClockLabelFor,
  roundTimeline,
  seekEventSeconds,
  timelineEvents,
  timelineTick,
} from '@/lib/tactical-timeline';
import type { RoundTimeline } from '@/lib/tactical-timeline';
import { dominantLevel, frameCursor, interpolatedSamples, sampleTrails } from '@/lib/tactical-replay';
import { isCalibrationUsable, radarViewRect } from '@/lib/tactical-transform';
import { drawTacticalScene, renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { TacticalEventList } from '@/components/tactical/tactical-event-list';
import { TacticalTimelineBar } from '@/components/tactical/tactical-timeline';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { cn } from '@/lib/utils';

/** Enough of the canvas box to draw one frame without measuring again. */
type CanvasMetrics = {
  cssWidth: number;
  cssHeight: number;
  dpr: number;
  /** Side of the virtual radar square; the box shows `view` of it. */
  size: number;
  offsetX: number;
  offsetY: number;
};

const INITIAL_METRICS: CanvasMetrics = {
  cssWidth: 1,
  cssHeight: 1,
  dpr: 1,
  size: 1,
  offsetX: 0,
  offsetY: 0,
};

/**
 * The plate's height ceiling; its width cap is this times the map's aspect.
 * Cropping the square to the play bounds made the plate show ~78% more map at
 * the same height, so the ceiling can come down and still draw a bigger map
 * than the uncropped square did: it now gives the column back ~80px, which is
 * what puts the scrubber and the transport on screen with the whole plate.
 */
const RADAR_MAX_HEIGHT_VH = 66;

/** A round with no frames still needs a timeline for the disabled transport. */
const EMPTY_TIMELINE = roundTimeline(
  { tick_start: 0, tick_freeze_end: 0, tick_end: 64, tick_official: 64 },
  64,
);

/** Reads the design tokens off the live canvas, so the radar follows the theme. */
function readRadarStyle(canvas: HTMLCanvasElement): RadarStyle {
  const computed = getComputedStyle(canvas);
  const token = (name: string, fallback: string): string => {
    const value = computed.getPropertyValue(name).trim();
    return value === '' ? fallback : value;
  };
  return {
    // --surface-0 is the ramp's void/well step, so the radar reads as a recess
    // in the --surface-2 panel instead of being painted in the panel's own
    // colour. `panel` is also the health-ring backing, which needs a dark ring
    // under it to separate stacked blips.
    background: token('--surface-0', '#04070f'),
    panel: token('--surface-0', '#04070f'),
    // Identity chips take the ramp's raised step rather than more alpha over
    // --surface-0, so a name reads on an opaque plate whatever is behind it.
    plate: token('--surface-3', '#141b2b'),
    // The occupancy grid is the map, not a signal: it stays neutral so cyan can
    // keep meaning "CT" on top of it.
    map: token('--fg-3', '#8998a7'),
    callout: token('--fg-3', '#8998a7'),
    ct: token('--primary', '#21d9ee'),
    t: token('--warning', '#fcb52c'),
    bomb: token('--destructive', '#fe545c'),
    defuse: token('--success', '#5de0b0'),
    utility: token('--fg-3', '#8998a7'),
    text: token('--fg-1', '#edf7fb'),
    // Resolved through the element, so the var() in --font-mono is already
    // substituted with the family next/font generated.
    fontFamily: computed.fontFamily || 'ui-monospace, monospace',
  };
}

/** The three-letter tag on a player's radar chip; the legend carries the full name. */
function shortLabel(name: string): string {
  return name.trim().slice(0, 3).toUpperCase();
}

/**
 * The 2D replay: the play-derived radar, the round's events and the transport.
 *
 * The canvas is a mirror of the `<input type="range">` below it, never the other
 * way round, and it is `aria-hidden`: the same round is rendered as text in the
 * event list beside it. Playback advances against the wall clock, so a dropped
 * frame costs smoothness and never synchronisation.
 */
export function TacticalReplay({
  doc,
  round,
  frames,
}: {
  doc: TacticalDocument;
  round: TacticalRound | undefined;
  frames: readonly TacticalFrame[];
}): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const scrubRef = useRef<HTMLInputElement>(null);
  const clockRef = useRef<HTMLSpanElement>(null);

  const metricsRef = useRef<CanvasMetrics>(INITIAL_METRICS);
  const styleRef = useRef<RadarStyle | null>(null);
  const backgroundsRef = useRef(new Map<string, HTMLCanvasElement>());
  const positionRef = useRef(0);
  const renderRef = useRef<(seconds: number) => void>(() => {});

  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState<number>(1);
  const [loop, setLoop] = useState(true);

  const timeline: RoundTimeline = useMemo(
    () => (round === undefined ? EMPTY_TIMELINE : roundTimeline(round, doc.demo.tickrate)),
    [round, doc.demo.tickrate],
  );

  const events = useMemo(
    () => (round === undefined ? [] : timelineEvents(timeline, round.events)),
    [round, timeline],
  );

  const names = useMemo(
    () => new Map(doc.players.map((player) => [player.slot, player.name])),
    [doc.players],
  );

  const labels = useMemo(
    () => new Map(doc.players.map((player) => [player.slot, shortLabel(player.name)])),
    [doc.players],
  );

  /**
   * The window of the native radar square this demo happened in. It decides the
   * plate's shape and how much of the square is on screen, never the transform:
   * the bake and every mark still run through one `size` under one translate, so
   * a blip lands on exactly the cell it lands on without a crop.
   */
  const view = useMemo(() => radarViewRect(doc.geometry), [doc.geometry]);

  /**
   * One measure for the plate and the transport that drives it. It is now the
   * map's own aspect rather than a square, so the 74vh ceiling only ever binds
   * on height. Typed rather than cast: `CSSProperties` has no index signature
   * for custom properties (the SHELL_VARS pattern in app/(app)/layout.tsx).
   */
  const radarVars = useMemo<CSSProperties & { '--radar-max': string }>(
    () => ({
      '--radar-max': `min(100%, ${((RADAR_MAX_HEIGHT_VH * view.width) / view.height).toFixed(2)}vh)`,
    }),
    [view],
  );

  /** The baked map bitmap for the current size, pixel ratio and level. */
  const background = useCallback(
    (level: RadarLevel, size: number, dpr: number, style: RadarStyle): HTMLCanvasElement => {
      const key = `${size}|${dpr}|${level}|${doc.geometry.map}|${view.x},${view.y},${view.width},${view.height}`;
      const cached = backgroundsRef.current.get(key);
      if (cached !== undefined) return cached;
      const baked = renderRadarBackground(doc.geometry, level, size, dpr, style, view);
      // A handful of sizes and two levels is all a session ever needs; drop the
      // cache wholesale rather than growing it after a long resize drag.
      if (backgroundsRef.current.size > 8) backgroundsRef.current.clear();
      backgroundsRef.current.set(key, baked);
      return baked;
    },
    [doc.geometry, view],
  );

  // Both the radar transform and the occupancy grid reject unusable numbers, and
  // they are the only thing standing between a bad calibration and a throwing
  // animation frame. Checked once here, so the canvas is simply not drawn.
  const drawable =
    isCalibrationUsable(doc.geometry.calibration) && doc.geometry.cell_size > 0;

  const renderAt = useCallback(
    (seconds: number) => {
      if (!drawable) return;
      const label = roundClockLabelFor(timeline, seconds);
      const scrub = scrubRef.current;
      if (scrub !== null) {
        scrub.value = String(seconds);
        scrub.setAttribute('aria-valuetext', label);
      }
      if (clockRef.current !== null) clockRef.current.textContent = label;

      const canvas = canvasRef.current;
      if (canvas === null) return;
      const context = canvas.getContext('2d', { alpha: false });
      if (context === null) return;
      const style = styleRef.current ?? readRadarStyle(canvas);
      const { cssWidth, cssHeight, dpr, size, offsetX, offsetY } = metricsRef.current;

      context.setTransform(dpr, 0, 0, dpr, 0, 0);
      context.fillStyle = style.background;
      context.fillRect(0, 0, cssWidth, cssHeight);
      context.translate(offsetX, offsetY);

      const cursor = frameCursor(frames, timelineTick(timeline, seconds));
      const samples = cursor === undefined ? [] : interpolatedSamples(frames, cursor);
      const level = dominantLevel(doc.geometry.calibration, samples);
      context.drawImage(
        background(level, size, dpr, style),
        view.x * size,
        view.y * size,
        view.width * size,
        view.height * size,
      );

      if (cursor !== undefined) {
        drawTacticalScene(context, {
          size,
          view,
          geometry: doc.geometry,
          activeLevel: level,
          samples,
          trails: sampleTrails(frames, cursor, timeline.tickrate, TRAIL_SECONDS),
          events: events.filter((entry) => entry.seconds <= seconds),
          nowSeconds: seconds,
          labels,
          style,
        });
      }
      context.setTransform(1, 0, 0, 1, 0, 0);
    },
    [background, doc.geometry, drawable, events, frames, labels, timeline, view],
  );

  useEffect(() => {
    renderRef.current = renderAt;
    renderAt(positionRef.current);
  }, [renderAt]);

  // A new round starts at its own beginning; the transport (playing, speed,
  // loop) deliberately survives the switch.
  useEffect(() => {
    positionRef.current = 0;
    renderRef.current(0);
  }, [round?.number]);

  useEffect(() => {
    backgroundsRef.current.clear();
  }, [doc.geometry]);

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
      // Writing either dimension clears the bitmap, so only touch them on a real
      // change and repaint straight after.
      if (canvas.width !== width) canvas.width = width;
      if (canvas.height !== height) canvas.height = height;
      // The square is scaled so its window fills the box, then panned so the
      // window's origin lands on the box's. With a full-square view this is
      // Math.floor(Math.min(cssWidth, cssHeight)) centred, exactly as before.
      const size = Math.max(1, Math.floor(Math.min(cssWidth / view.width, cssHeight / view.height)));
      metricsRef.current = {
        cssWidth,
        cssHeight,
        dpr,
        size,
        offsetX: Math.round((cssWidth - size * view.width) / 2 - size * view.x),
        offsetY: Math.round((cssHeight - size * view.height) / 2 - size * view.y),
      };
      styleRef.current = readRadarStyle(canvas);
      renderRef.current(positionRef.current);
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    // Zoom changes the pixel ratio without necessarily changing the CSS box.
    window.addEventListener('resize', measure);
    return () => {
      observer.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [view]);

  useEffect(() => {
    if (!playing) return;
    let frame = 0;
    let last = performance.now();

    const step = (now: number): void => {
      const next = advanceTransport(
        { position: positionRef.current, playing: true },
        { elapsedMs: now - last, speed, durationSeconds: timeline.durationSeconds, loop },
      );
      last = now;
      positionRef.current = next.position;
      renderRef.current(next.position);
      if (!next.playing) {
        setPlaying(false);
        return;
      }
      frame = requestAnimationFrame(step);
    };

    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [playing, speed, loop, timeline.durationSeconds]);

  const seekTo = useCallback(
    (seconds: number) => {
      positionRef.current = clampToTimeline(timeline, seconds);
      renderRef.current(positionRef.current);
    },
    [timeline],
  );

  const togglePlay = useCallback(() => {
    setPlaying((current) => {
      if (current) return false;
      // Pressing play on a finished round restarts it instead of doing nothing.
      if (positionRef.current >= timeline.durationSeconds) seekTo(0);
      return true;
    });
  }, [seekTo, timeline.durationSeconds]);

  const onKeyDown = useCallback(
    (event: KeyboardEvent<HTMLElement>) => {
      const onButton = event.target instanceof HTMLElement && event.target.tagName === 'BUTTON';
      if (event.key === ' ') {
        if (onButton) return;
        event.preventDefault();
        togglePlay();
        return;
      }
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
      const direction = event.key === 'ArrowRight' ? 1 : -1;
      event.preventDefault();
      if (event.shiftKey) {
        const target = seekEventSeconds(events, positionRef.current, direction);
        if (target !== undefined) seekTo(target);
        return;
      }
      seekTo(positionRef.current + direction * SEEK_STEP_SECONDS);
    },
    [events, seekTo, togglePlay],
  );

  if (round === undefined) {
    return (
      <section className="studio-panel flex min-h-[420px] flex-col items-center justify-center gap-3 px-6 text-center">
        <Radar className="size-6 text-muted-foreground" aria-hidden />
        <p className="text-body leading-6 text-muted-foreground">
          Selecciona una ronda para abrir la repetición.
        </p>
      </section>
    );
  }

  return (
    <section
      className="studio-panel @container"
      aria-label={`Repetición de la ronda ${round.number}`}
    >
      <header className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2 border-b border-border-subtle px-5 py-4">
        <h2 className="font-display text-title font-bold uppercase tracking-tight text-foreground">
          Ronda {round.number}
          <span className="pl-3 font-mono text-body-sm font-normal tabular-nums text-muted-foreground">
            {round.score_ct_before}:{round.score_t_before}
          </span>
        </h2>
        <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
          {buyLabel(round.economy.ct_buy)} vs {buyLabel(round.economy.t_buy)} · {tPatternLabel(round.class.t_side)} /{' '}
          {ctPatternLabel(round.class.ct_side)} · {siteLabel(round.class.site)}
        </p>
      </header>

      {/* The keyboard contract lives on the section so the transport answers
          wherever focus sits inside the replay. The split is a container query,
          not a viewport one: the panel's own width is what decides whether the
          radar can afford a column beside it. */}
      <div
        className="grid gap-5 p-4 sm:p-5 @[62rem]:grid-cols-[minmax(0,1fr)_300px]"
        onKeyDown={onKeyDown}
      >
        {/* One measure for the media and the transport that drives it, so the
            scrubber stops overhanging the radar on both sides. The measure is
            the map's own aspect now, not a square: --radar-max is 74vh times
            that aspect, so the ceiling still binds on height. */}
        <div className="flex min-w-0 flex-col gap-4" style={radarVars}>
          <div
            ref={containerRef}
            // The inverted --edge-shade/--edge-light pair reads as a recess
            // rather than a raised plate. The ratio is the crop window's, so the
            // box is the map instead of the 1024 square the map sits in.
            className="relative mx-auto w-full max-w-(--radar-max) overflow-hidden rounded-lg border border-border-strong bg-surface-0 shadow-[inset_0_1px_0_0_var(--edge-shade),inset_0_-1px_0_0_var(--edge-light)]"
            style={{ aspectRatio: `${view.width} / ${view.height}` }}
          >
            <canvas
              ref={canvasRef}
              aria-hidden="true"
              className="block h-full w-full font-mono"
            />
            {drawable ? null : (
              <p className="absolute inset-0 grid place-items-center px-6 text-center text-body-sm leading-5 text-muted-foreground">
                Este mapa no tiene una calibración de radar utilizable, así que no se puede dibujar la
                repetición. Los eventos y las tendencias siguen siendo válidos.
              </p>
            )}
          </div>

          {drawable && frames.length === 0 ? (
            <p className="rounded-md border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm leading-5 text-warning">
              Esta ronda no tiene posiciones en el blob: la repetición queda vacía, el resto del análisis no.
            </p>
          ) : null}

          <div className="mx-auto w-full max-w-(--radar-max)">
            <TacticalTimelineBar
              timeline={timeline}
              events={events}
              playing={playing}
              speed={speed}
              loop={loop}
              scrubRef={scrubRef}
              clockRef={clockRef}
              onTogglePlay={togglePlay}
              onSpeedChange={setSpeed}
              onToggleLoop={() => setLoop((current) => !current)}
              onSeek={seekTo}
            />
          </div>

          <RoundLegend round={round} names={names} labels={labels} />
        </div>

        <aside className="flex min-w-0 flex-col rounded-lg border border-border-strong bg-surface-1 @[62rem]:max-h-[62vh]">
          <h3 className="border-b border-border-subtle px-3 py-2.5 font-mono text-meta uppercase tracking-widest text-fg-3">
            Eventos de la ronda
          </h3>
          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
            <TacticalEventList timeline={timeline} events={events} names={names} onSeek={seekTo} />
          </div>
        </aside>
      </div>
    </section>
  );
}

/** Who is who on the radar: the three-letter tag, the name and the round's line. */
function RoundLegend({
  round,
  names,
  labels,
}: {
  round: TacticalRound;
  names: ReadonlyMap<number, string>;
  labels: ReadonlyMap<number, string>;
}): ReactNode {
  const sides: TacticalSide[] = [TACTICAL_SIDES.ct, TACTICAL_SIDES.t];
  return (
    <div className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
      {sides.map((side) => (
        <div key={side} className="flex min-w-0 flex-col gap-1.5">
          <span
            className={cn(
              'font-mono text-meta uppercase tracking-widest',
              side === TACTICAL_SIDES.ct ? 'text-primary' : 'text-warning',
            )}
          >
            {side}
          </span>
          {round.players
            .filter((player) => player.side === side)
            .map((player) => (
              <div
                key={player.slot}
                className="flex min-w-0 items-center gap-2 font-mono text-meta tracking-normal tabular-nums"
              >
                <span
                  className={cn(
                    'w-11 shrink-0 border-l-2 bg-surface-3 py-0.5 pr-1.5 pl-1 text-fg-1',
                    player.side === TACTICAL_SIDES.ct ? 'border-l-primary' : 'border-l-warning',
                  )}
                >
                  {labels.get(player.slot) ?? '···'}
                </span>
                <span className="min-w-0 flex-1 truncate text-foreground">
                  {names.get(player.slot) ?? `slot ${player.slot}`}
                </span>
                <span className="shrink-0 text-muted-foreground">
                  {player.kills}/{player.deaths}/{player.assists}
                </span>
                {player.survived ? null : <span className="shrink-0 text-fg-3">✕</span>}
              </div>
            ))}
        </div>
      ))}
    </div>
  );
}
