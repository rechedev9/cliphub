import { AmbientLight, Color, DirectionalLight, HemisphereLight, Mesh, MeshBasicMaterial, PlaneGeometry, Raycaster, SRGBColorSpace, Scene, Texture, Vector2, Vector3, WebGLRenderer } from 'three';
import type { Object3D } from 'three';
import type { MapGeometry } from '@/lib/api/map-geometry';
import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '@/lib/api/tactical';
import type { RadarCalibration, TacticalSample } from '@/lib/api/tactical';
import { docNames } from '@/lib/replay/centre-round';
import type { CentreRound } from '@/lib/replay/centre-round';
import { layerSamples, layerTrails } from '@/lib/replay/layers';
import type { ReplayLayers } from '@/lib/replay/layers';
import type { Vec3, WorldBox } from '@/lib/replay/three/orbit';
import { eyeHeight, lookYaw, viewDirection } from '@/lib/replay/three/scene-math';
import { actionFocus, liveCut } from '@/lib/replay/three/action-camera';
import { layoutTags } from '@/lib/replay/tag-layout';
import { createTrail3dReader } from '@/lib/replay/three/trails';
import { bombVisual, utilityVisuals } from '@/lib/replay/three/utility-visuals';
import { createInterpolatedSampleReader, frameCursor } from '@/lib/tactical-replay';
import type { FrameCursor } from '@/lib/tactical-replay';
import { TRAIL_SECONDS, timelineSeconds } from '@/lib/tactical-timeline';
import { CameraRig } from './camera-rig';
import type { CameraMode } from './camera-rig';
import { MapMesh } from './map-mesh';
import { ActorKit, OVERLAY_OPACITY, PlayerActor } from './player-actors';
import type { SceneColors } from './scene-colors';
import { UtilityLayer } from './utility-layer';

export type SceneInput = {
  rounds: readonly CentreRound[];
  tickOf: (round: CentreRound, seconds: number) => number;
  layers: ReplayLayers;
};

type RoundState = {
  round: CentreRound;
  samples: (cursor: FrameCursor) => TacticalSample[];
  trails: (cursor: FrameCursor) => ReadonlyMap<number, readonly Vec3[]>;
  actors: Map<number, PlayerActor>;
  names: Map<number, string>;
};

/** Name tags this close to the screen edge or behind the camera are hidden. */
const LABEL_EDGE = 4;

/**
 * The 3D replay scene: the map, every round's players and grenades, three
 * cameras. It renders on demand: a clock tick, a drag or a resize asks for a
 * frame, and at most one frame is drawn per animation frame.
 */
export class ReplayScene {
  readonly #renderer: WebGLRenderer;
  readonly #scene = new Scene();
  readonly #rig: CameraRig;
  readonly #kit: ActorKit;
  readonly #utility: UtilityLayer;
  readonly #overlay: HTMLElement;
  readonly #rounds = new Map<string, RoundState>();
  readonly #bodies = new Map<Object3D, number>();
  readonly #raycaster = new Raycaster();
  readonly #projected = new Vector3();
  #map: MapMesh | null = null;
  #radar: Mesh<PlaneGeometry, MeshBasicMaterial> | null = null;
  #input: SceneInput = { rounds: [], tickOf: () => 0, layers: { utility: true, bomb: true, trails: true, names: true, hiddenSlots: [] } };
  #seconds = 0;
  #cut: number | null = null;
  #meshZ = { minZ: 0, maxZ: 0 };
  readonly #labelSpots = new Map<number, number>();
  #follow: number | null = null;
  #frame = 0;
  #width = 1;
  #height = 1;
  readonly #onPick: (slot: number | null) => void;

  constructor(options: { canvas: HTMLCanvasElement; overlay: HTMLElement; colors: SceneColors; onPick: (slot: number | null) => void; onAutoChange: (auto: boolean) => void }) {
    this.#renderer = new WebGLRenderer({ canvas: options.canvas, antialias: true, powerPreference: 'high-performance' });
    this.#renderer.localClippingEnabled = true;
    this.#overlay = options.overlay;
    this.#onPick = options.onPick;
    this.#kit = new ActorKit(options.colors);
    this.#utility = new UtilityLayer(options.colors);
    this.#scene.add(this.#utility.group);
    // A cool sky over a warm bounce, and a low sun from the south-west so walls read apart.
    const sky = new HemisphereLight(0xdfe8ff, 0x3a3428, 1.5);
    sky.position.set(0, 0, 1);
    this.#scene.add(sky);
    this.#scene.add(new AmbientLight(0xffffff, 0.3));
    const sun = new DirectionalLight(0xfff4e6, 1.6);
    sun.position.set(-0.55, -0.8, 1);
    this.#scene.add(sun);
    this.#rig = new CameraRig(options.canvas, { onChange: () => this.requestRender(), onClick: (x, y) => this.#pick(x, y), onAutoChange: options.onAutoChange });
    this.setColors(options.colors);
  }

  setMap(geometry: MapGeometry, box: WorldBox, colors: SceneColors): void {
    if (this.#map !== null) {
      this.#scene.remove(...this.#map.objects);
      this.#map.dispose();
    }
    this.#map = new MapMesh(geometry, colors.background);
    this.#meshZ = { minZ: geometry.bounds.min[2], maxZ: geometry.bounds.max[2] };
    this.#scene.add(...this.#map.objects);
    this.#rig.frame(box);
    this.requestRender();
  }

  /**
   * The game's overview laid flat under the map. The top-down camera shows it
   * instead of the mesh, so that view reads like the radar players know.
   */
  setRadar(image: HTMLImageElement | null, calibration: RadarCalibration | null, floorZ: number): void {
    if (this.#radar !== null) {
      this.#scene.remove(this.#radar);
      this.#radar.geometry.dispose();
      this.#radar.material.map?.dispose();
      this.#radar.material.dispose();
      this.#radar = null;
    }
    if (image !== null && calibration !== null && calibration.scale > 0) {
      const side = calibration.scale * image.naturalWidth;
      const texture = new Texture(image);
      texture.colorSpace = SRGBColorSpace;
      texture.anisotropy = 4;
      texture.needsUpdate = true;
      const plane = new Mesh(new PlaneGeometry(side, side), new MeshBasicMaterial({ map: texture }));
      plane.position.set(calibration.pos_x + side / 2, calibration.pos_y - side / 2, floorZ - 4);
      plane.visible = false;
      this.#radar = plane;
      this.#scene.add(plane);
    }
    this.requestRender();
  }

  /** A fixed roof cut, or null to keep it just over the focused round's living players. */
  setCut(z: number | null): void {
    this.#cut = z;
    if (z !== null) this.#map?.setCut(z);
    this.requestRender();
  }

  setMode(mode: CameraMode): void {
    this.#rig.setMode(mode);
    this.requestRender();
  }

  /** Whether the orbit camera follows the living players. */
  setAuto(auto: boolean): void {
    this.#rig.setAuto(auto);
    this.requestRender();
  }

  setFollow(slot: number | null): void {
    this.#follow = slot;
    this.requestRender();
  }

  setColors(colors: SceneColors): void {
    this.#scene.background = new Color().setRGB(colors.background[0], colors.background[1], colors.background[2], 'srgb');
    this.#map?.setColor(colors.background);
    this.#kit.setColors(colors);
    this.#utility.setColors(colors);
    this.requestRender();
  }

  setInput(input: SceneInput): void {
    this.#input = input;
    const keep = new Set(input.rounds.map((round) => round.key));
    for (const [key, state] of this.#rounds) {
      const next = input.rounds.find((round) => round.key === key);
      if (next !== undefined && next.frames === state.round.frames && next.focused === state.round.focused) continue;
      this.#dropRound(key, state);
    }
    for (const round of input.rounds) {
      const known = this.#rounds.get(round.key);
      if (known !== undefined) {
        known.round = round;
        continue;
      }
      this.#rounds.set(round.key, {
        round,
        samples: createInterpolatedSampleReader(round.frames),
        trails: createTrail3dReader(round.frames, round.timeline.tickrate, TRAIL_SECONDS),
        actors: new Map(),
        names: docNames(round.doc),
      });
    }
    this.#utility.keepRounds(keep);
    this.requestRender();
  }

  showAt(seconds: number): void {
    this.#seconds = seconds;
    this.requestRender();
  }

  resize(width: number, height: number): void {
    this.#width = Math.max(1, width);
    this.#height = Math.max(1, height);
    this.#renderer.setPixelRatio(window.devicePixelRatio > 0 ? Math.min(2, window.devicePixelRatio) : 1);
    this.#renderer.setSize(this.#width, this.#height, false);
    this.#rig.resize(this.#width, this.#height);
    this.#render();
  }

  requestRender(): void {
    if (this.#frame !== 0) return;
    this.#frame = requestAnimationFrame(() => {
      this.#frame = 0;
      this.#render();
    });
  }

  dispose(): void {
    cancelAnimationFrame(this.#frame);
    this.#rig.dispose();
    for (const [key, state] of this.#rounds) this.#dropRound(key, state);
    this.#utility.dispose();
    this.#kit.dispose();
    this.#map?.dispose();
    this.setRadar(null, null, 0);
    this.#renderer.dispose();
    this.#renderer.forceContextLoss();
  }

  #dropRound(key: string, state: RoundState): void {
    for (const actor of state.actors.values()) {
      this.#scene.remove(...actor.objects);
      this.#bodies.delete(actor.body);
      actor.dispose();
    }
    this.#rounds.delete(key);
  }

  #actor(state: RoundState, slot: number): PlayerActor {
    const found = state.actors.get(slot);
    if (found !== undefined) return found;
    const actor = new PlayerActor(slot, this.#kit, !state.round.focused, this.#overlay, state.names.get(slot) ?? `Jugador ${slot}`);
    state.actors.set(slot, actor);
    this.#scene.add(...actor.objects);
    if (state.round.focused) this.#bodies.set(actor.body, slot);
    return actor;
  }

  #render(): void {
    const { layers, tickOf } = this.#input;
    this.#utility.beginFrame();
    let eye: { at: Vec3; direction: Vec3 } | null = null;
    let focusedSamples: TacticalSample[] = [];
    for (const state of this.#rounds.values()) {
      const { round } = state;
      for (const actor of state.actors.values()) actor.hide();
      const tick = tickOf(round, this.#seconds);
      const nowSeconds = timelineSeconds(round.timeline, tick);
      const strength = round.focused ? 1 : OVERLAY_OPACITY;
      const cursor = frameCursor(round.frames, tick);
      let carried = false;
      if (cursor !== undefined) {
        const samples = layerSamples(state.samples(cursor), layers);
        if (round.focused) focusedSamples = samples;
        const trails = layerTrails(state.trails(cursor), layers);
        const previous = round.frames[cursor.index - 1]?.samples;
        for (const sample of samples) {
          if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb)) carried = true;
          const yaw = lookYaw(sample, previous?.find((candidate) => candidate.slot === sample.slot));
          const self = round.focused && this.#rig.mode === 'player' && this.#follow === sample.slot;
          const actor = this.#actor(state, sample.slot);
          actor.update(sample, yaw, { self, bombLayer: layers.bomb, scale: this.#rig.actorScale });
          actor.setTrail(trails.get(sample.slot));
          if (self) {
            eye = { at: { x: sample.x, y: sample.y, z: sample.z + eyeHeight(sample) }, direction: viewDirection(yaw) };
          }
        }
      }
      if (layers.utility) this.#utility.grenades(round.key, utilityVisuals(round.events, nowSeconds, (at) => timelineSeconds(round.timeline, at)), strength);
      if (layers.bomb) this.#utility.bomb(round.key, bombVisual(round.events, nowSeconds, carried), strength);
    }
    this.#utility.endFrame();
    if (eye !== null) this.#rig.placeEye(eye.at, eye.direction);
    if (this.#cut === null) this.#map?.setCut(liveCut(this.#meshZ, focusedSamples));
    this.#rig.setFocus(actionFocus(focusedSamples));
    if (this.#rig.step(performance.now())) this.requestRender();
    const camera = this.#rig.active;
    const flat = this.#radar !== null && this.#rig.mode === 'top';
    if (this.#radar !== null) this.#radar.visible = flat;
    this.#map?.setVisible(!flat);
    this.#renderer.render(this.#scene, camera);
    this.#placeLabels();
  }

  /** Name tags over the heads, laid out like the 2D tags so none covers another. */
  #placeLabels(): void {
    const { layers } = this.#input;
    const camera = this.#rig.active;
    const shown: { actor: PlayerActor; x: number; y: number }[] = [];
    for (const state of this.#rounds.values()) {
      for (const actor of state.actors.values()) {
        const self = this.#rig.mode === 'player' && this.#follow === actor.slot && state.round.focused;
        if (!actor.shown || !state.round.focused || !layers.names || self) {
          actor.label.classList.add('hidden');
          continue;
        }
        const { x, y, z } = actor.anchor;
        const projected = this.#projected.set(x, y, z).project(camera);
        const sx = ((projected.x + 1) / 2) * this.#width;
        const sy = ((1 - projected.y) / 2) * this.#height;
        const outside = projected.z > 1 || projected.z < -1 || sx < LABEL_EDGE || sy < LABEL_EDGE || sx > this.#width - LABEL_EDGE || sy > this.#height;
        if (outside) {
          actor.label.classList.add('hidden');
          continue;
        }
        actor.label.classList.remove('hidden');
        shown.push({ actor, x: sx, y: sy });
      }
    }
    const placed = layoutTags({
      tags: shown.map(({ actor, x, y }) => ({ id: actor.slot, x, y, width: actor.label.offsetWidth, height: actor.label.offsetHeight })),
      discs: shown,
      radius: 3,
      bounds: { x: 0, y: 0, width: this.#width, height: this.#height },
      prefer: this.#labelSpots,
    });
    this.#labelSpots.clear();
    for (const tag of placed) {
      this.#labelSpots.set(tag.id, tag.spot);
      const entry = shown.find((candidate) => candidate.actor.slot === tag.id);
      entry?.actor.label.style.setProperty('transform', `translate(${tag.rect.x.toFixed(1)}px, ${tag.rect.y.toFixed(1)}px)`);
    }
  }

  #pick(x: number, y: number): void {
    const pointer = new Vector2((x / this.#width) * 2 - 1, -(y / this.#height) * 2 + 1);
    this.#raycaster.setFromCamera(pointer, this.#rig.active);
    const bodies = [...this.#bodies.keys()].filter((body) => body.visible && body.parent?.visible === true);
    const hit = this.#raycaster.intersectObjects(bodies, false)[0];
    const slot = hit === undefined ? undefined : this.#bodies.get(hit.object);
    if (slot !== undefined) this.#onPick(slot);
  }
}
