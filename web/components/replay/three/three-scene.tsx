'use client';

import { useEffect, useRef, type ReactNode } from 'react';
import type { MapGeometry } from '@/lib/api/map-geometry';
import type { MapScene } from '@/lib/api/map-scene';
import type { RadarCalibration } from '@/lib/api/tactical';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayClock } from '@/lib/replay/clock';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { WorldBox } from '@/lib/replay/three/orbit';
import type { CameraMode } from './camera-rig';
import { ReplayScene } from './replay-scene';
import { readSceneColors } from './scene-colors';

export type ThreeSceneProps = {
  geometry: MapGeometry;
  /** The walkable floor the roofs are cut over; null when it could not be read. */
  nav: MapGeometry | null;
  box: WorldBox;
  /** A fixed roof cut, or null to keep it over the living players. */
  cut: number | null;
  /** Whether the orbit follows the living players. */
  auto: boolean;
  onAutoChange: (auto: boolean) => void;
  mode: CameraMode;
  follow: number | null;
  rounds: readonly CentreRound[];
  tickOf: (round: CentreRound, seconds: number) => number;
  layers: ReplayLayers;
  clock: ReplayClock;
  onPick: (slot: number) => void;
  onFailure: () => void;
  label: string;
  /** The official overview, when CS2 serves it. */
  radar: { image: HTMLImageElement; calibration: RadarCalibration } | null;
  /** Whether the overview is laid on the floors. */
  radarOn: boolean;
  /** The map's real surfaces from CS2, once read. */
  scene: MapScene | null;
  /** Whether the real surfaces draw instead of the plain map. */
  realOn: boolean;
};

/**
 * The WebGL canvas and the name tags over it. Three.js lives only in this
 * module's chunk, so pages that never open the 3D view never load it.
 */
export function ThreeScene({ geometry, nav, radarOn, scene: realScene, realOn, box, cut, mode, follow, rounds, tickOf, layers, clock, onPick, onFailure, label, radar, auto, onAutoChange }: ThreeSceneProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<ReplayScene | null>(null);
  const pickRef = useRef(onPick);
  pickRef.current = onPick;
  const autoRef = useRef(onAutoChange);
  autoRef.current = onAutoChange;
  const failRef = useRef(onFailure);
  failRef.current = onFailure;
  const boxRef = useRef(box);
  boxRef.current = box;
  const labelRef = useRef(label);
  labelRef.current = label;

  useEffect(() => {
    const container = containerRef.current;
    const overlay = overlayRef.current;
    if (container === null || overlay === null) return;
    // A fresh canvas per mount: dispose loses the old context, and a remount must not reuse it.
    const canvas = document.createElement('canvas');
    canvas.setAttribute('role', 'img');
    canvas.setAttribute('aria-label', labelRef.current);
    canvas.className = 'block h-full w-full touch-none select-none';
    container.prepend(canvas);
    let scene: ReplayScene;
    try {
      scene = new ReplayScene({ canvas, overlay, colors: readSceneColors(container), onPick: (slot) => slot !== null && pickRef.current(slot), onAutoChange: (on) => autoRef.current(on) });
    } catch {
      canvas.remove();
      failRef.current();
      return;
    }
    sceneRef.current = scene;
    const measure = (): void => {
      const rect = container.getBoundingClientRect();
      scene.resize(rect.width, rect.height);
    };
    measure();
    const resize = new ResizeObserver(measure);
    resize.observe(container);
    const theme = new MutationObserver(() => scene.setColors(readSceneColors(container)));
    theme.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    const lost = (event: Event): void => {
      event.preventDefault();
      failRef.current();
    };
    canvas.addEventListener('webglcontextlost', lost);
    return () => {
      canvas.removeEventListener('webglcontextlost', lost);
      resize.disconnect();
      theme.disconnect();
      sceneRef.current = null;
      scene.dispose();
      canvas.remove();
    };
  }, []);

  useEffect(() => {
    containerRef.current?.querySelector('canvas')?.setAttribute('aria-label', label);
  }, [label]);

  useEffect(() => {
    const container = containerRef.current;
    if (container === null) return;
    sceneRef.current?.setMap(geometry, nav, boxRef.current, readSceneColors(container));
  }, [geometry, nav]);

  useEffect(() => {
    sceneRef.current?.setScene(realScene);
  }, [realScene, geometry]);

  useEffect(() => {
    sceneRef.current?.setRealOn(realOn);
  }, [realOn]);

  useEffect(() => {
    sceneRef.current?.setRadar(radar?.image ?? null, radar?.calibration ?? null);
  }, [radar]);

  useEffect(() => {
    sceneRef.current?.setRadarOn(radarOn);
  }, [radarOn]);

  useEffect(() => {
    sceneRef.current?.setCut(cut);
  }, [cut]);

  useEffect(() => {
    sceneRef.current?.setMode(mode);
  }, [mode]);

  useEffect(() => {
    sceneRef.current?.setAuto(auto);
  }, [auto]);

  useEffect(() => {
    sceneRef.current?.setFollow(follow);
  }, [follow]);

  useEffect(() => {
    sceneRef.current?.setInput({ rounds, tickOf, layers });
  }, [rounds, tickOf, layers]);

  useEffect(() => {
    const scene = sceneRef.current;
    if (scene === null) return;
    scene.showAt(clock.seconds);
    return clock.subscribe((seconds) => scene.showAt(seconds));
  }, [clock]);

  return (
    <div ref={containerRef} className="relative h-full w-full overflow-hidden">
      <div ref={overlayRef} aria-hidden="true" className="pointer-events-none absolute inset-0" />
    </div>
  );
}
