import { PerspectiveCamera, Quaternion, Vector3 } from 'three';
import type { Camera } from 'three';
import { easeFactor, easeInOut, focusDistance } from '@/lib/replay/three/action-camera';
import type { ActionFocus } from '@/lib/replay/three/action-camera';
import {
  ORBIT_MAX_DISTANCE,
  ORBIT_MIN_DISTANCE,
  framingOrbit,
  framingTop,
  orbitDrag,
  orbitPan,
  orbitPosition,
  orbitZoom,
  topPan,
  topZoom,
  unitsPerPixelAt,
} from '@/lib/replay/three/orbit';
import type { OrbitState, TopState, Vec3, WorldBox } from '@/lib/replay/three/orbit';

export type CameraMode = 'orbit' | 'top' | 'player';

const ORBIT_FOV = 50;
/** CS2's default 90 degree horizontal field of view at 16:9. */
const PLAYER_FOV = 74;
/** Movement under this many pixels between press and release is a click. */
const CLICK_SLOP = 5;
/** How long a switch between cameras glides, in milliseconds. */
const TRANSITION_MS = 450;
/** Time constant of the camera following the action, in seconds. */
const FOLLOW_TAU = 0.45;

/** Players draw this much wider in the top view, as on a radar. */
const TOP_ACTOR_SCALE = 2.4;
/** In the orbit, players grow past this camera distance, up to the max scale. */
const ORBIT_ACTOR_DISTANCE = 1500;
const ORBIT_MAX_ACTOR_SCALE = 2.4;

type Pose = { position: Vector3; quaternion: Quaternion; fov: number };

/**
 * The three cameras and the pointer controls: left drag orbits (pans in the
 * top view), right drag pans, the wheel zooms. The orbit follows the living
 * players until the user moves it; switching cameras glides between them.
 * The top view is a perspective camera straight down, so the glide is continuous.
 */
export class CameraRig {
  readonly orbitCamera = new PerspectiveCamera(ORBIT_FOV, 1, 4, 60000);
  readonly topCamera = new PerspectiveCamera(ORBIT_FOV, 1, 4, 80000);
  readonly playerCamera = new PerspectiveCamera(PLAYER_FOV, 1, 2, 40000);
  readonly #glideCamera = new PerspectiveCamera(ORBIT_FOV, 1, 2, 80000);
  #mode: CameraMode = 'orbit';
  #orbit: OrbitState = { target: { x: 0, y: 0, z: 0 }, distance: 4000, azimuth: 0, elevation: 0.8 };
  #top: TopState = { x: 0, y: 0, halfHeight: 2000 };
  #groundZ = 0;
  #width = 1;
  #height = 1;
  #auto = true;
  #focus: ActionFocus | null = null;
  #lastStep = 0;
  #glide: { from: Pose; start: number } | null = null;
  #drag: { id: number; button: number; x: number; y: number; moved: number } | null = null;
  readonly #canvas: HTMLCanvasElement;
  readonly #onChange: () => void;
  readonly #onClick: (x: number, y: number) => void;
  readonly #onAutoChange: (auto: boolean) => void;
  readonly #abort = new AbortController();

  constructor(canvas: HTMLCanvasElement, handlers: { onChange: () => void; onClick: (x: number, y: number) => void; onAutoChange: (auto: boolean) => void }) {
    this.#canvas = canvas;
    this.#onChange = handlers.onChange;
    this.#onClick = handlers.onClick;
    this.#onAutoChange = handlers.onAutoChange;
    for (const camera of [this.orbitCamera, this.playerCamera, this.#glideCamera]) camera.up.set(0, 0, 1);
    this.topCamera.up.set(0, 1, 0);
    const signal = this.#abort.signal;
    canvas.addEventListener('pointerdown', (event) => this.#down(event), { signal });
    canvas.addEventListener('pointermove', (event) => this.#move(event), { signal });
    canvas.addEventListener('pointerup', (event) => this.#up(event), { signal });
    canvas.addEventListener('pointercancel', () => {
      this.#drag = null;
    }, { signal });
    canvas.addEventListener('wheel', (event) => this.#wheel(event), { signal, passive: false });
    canvas.addEventListener('contextmenu', (event) => event.preventDefault(), { signal });
  }

  get mode(): CameraMode {
    return this.#mode;
  }

  /** Switches camera, gliding from where the current one is. */
  setMode(mode: CameraMode): void {
    if (mode === this.#mode) return;
    const current = this.active;
    this.#glide = {
      from: { position: current.position.clone(), quaternion: current.quaternion.clone(), fov: current instanceof PerspectiveCamera ? current.fov : ORBIT_FOV },
      start: performance.now(),
    };
    this.#mode = mode;
  }

  /** Whether the orbit follows the living players; any drag or wheel in the orbit turns it off. */
  setAuto(auto: boolean): void {
    if (this.#auto === auto) return;
    this.#auto = auto;
    this.#onAutoChange(auto);
  }

  /** How big the players draw: wider from far away so they stay readable over the map. */
  get actorScale(): number {
    if (this.#mode === 'top') return TOP_ACTOR_SCALE;
    if (this.#mode === 'player') return 1;
    return Math.min(ORBIT_MAX_ACTOR_SCALE, Math.max(1, this.#orbit.distance / ORBIT_ACTOR_DISTANCE));
  }

  setFocus(focus: ActionFocus | null): void {
    this.#focus = focus;
  }

  get active(): Camera {
    return this.#glide === null ? this.#modeCamera() : this.#glideCamera;
  }

  #modeCamera(): PerspectiveCamera {
    if (this.#mode === 'top') return this.topCamera;
    if (this.#mode === 'player') return this.playerCamera;
    return this.orbitCamera;
  }

  /** Points both overview cameras at a box. */
  frame(box: WorldBox): void {
    const aspect = this.#width / this.#height;
    this.#orbit = framingOrbit(box, ORBIT_FOV, aspect);
    this.#top = framingTop(box, aspect);
    this.#groundZ = box.min.z;
    this.#apply();
  }

  resize(width: number, height: number): void {
    this.#width = Math.max(1, width);
    this.#height = Math.max(1, height);
    const aspect = this.#width / this.#height;
    for (const camera of [this.orbitCamera, this.topCamera, this.playerCamera, this.#glideCamera]) {
      camera.aspect = aspect;
      camera.updateProjectionMatrix();
    }
    this.#apply();
  }

  /** The player camera at an eye, looking along a unit direction. */
  placeEye(eye: Vec3, direction: Vec3): void {
    this.playerCamera.position.set(eye.x, eye.y, eye.z);
    this.playerCamera.lookAt(new Vector3(eye.x + direction.x, eye.y + direction.y, eye.z + direction.z));
  }

  /** Advances the follow and the glide; true while the camera is still moving. */
  step(now: number): boolean {
    const dt = this.#lastStep === 0 ? 0 : Math.min(0.1, (now - this.#lastStep) / 1000);
    this.#lastStep = now;
    let moving = false;
    const focus = this.#focus;
    if (this.#auto && focus !== null && this.#mode === 'orbit') {
      const k = easeFactor(dt, FOLLOW_TAU);
      const target = this.#orbit.target;
      const distance = Math.min(ORBIT_MAX_DISTANCE, Math.max(ORBIT_MIN_DISTANCE, focusDistance(focus.radius, ORBIT_FOV)));
      const next = {
        x: target.x + (focus.centre.x - target.x) * k,
        y: target.y + (focus.centre.y - target.y) * k,
        z: target.z + (focus.centre.z - target.z) * k,
      };
      const nextDistance = this.#orbit.distance + (distance - this.#orbit.distance) * k;
      moving = Math.hypot(next.x - focus.centre.x, next.y - focus.centre.y) > 4 || Math.abs(nextDistance - distance) > 4;
      this.#orbit = { ...this.#orbit, target: next, distance: nextDistance };
      this.#apply();
    }
    const glide = this.#glide;
    if (glide !== null) {
      const t = (now - glide.start) / TRANSITION_MS;
      if (t >= 1) this.#glide = null;
      else {
        const to = this.#modeCamera();
        const e = easeInOut(t);
        this.#glideCamera.position.lerpVectors(glide.from.position, to.position, e);
        this.#glideCamera.quaternion.slerpQuaternions(glide.from.quaternion, to.quaternion, e);
        this.#glideCamera.fov = glide.from.fov + (to.fov - glide.from.fov) * e;
        this.#glideCamera.updateProjectionMatrix();
        moving = true;
      }
    }
    return moving;
  }

  dispose(): void {
    this.#abort.abort();
  }

  #apply(): void {
    const at = orbitPosition(this.#orbit);
    this.orbitCamera.position.set(at.x, at.y, at.z);
    this.orbitCamera.lookAt(this.#orbit.target.x, this.#orbit.target.y, this.#orbit.target.z);
    const height = this.#top.halfHeight / Math.tan((ORBIT_FOV * Math.PI) / 360);
    this.topCamera.position.set(this.#top.x, this.#top.y, this.#groundZ + height);
    this.topCamera.lookAt(this.#top.x, this.#top.y, this.#groundZ);
  }

  #down(event: PointerEvent): void {
    if (event.button !== 0 && event.button !== 2) return;
    this.#canvas.setPointerCapture(event.pointerId);
    this.#drag = { id: event.pointerId, button: event.button, x: event.clientX, y: event.clientY, moved: 0 };
  }

  #move(event: PointerEvent): void {
    const drag = this.#drag;
    if (drag === null || drag.id !== event.pointerId) return;
    const dx = event.clientX - drag.x;
    const dy = event.clientY - drag.y;
    drag.x = event.clientX;
    drag.y = event.clientY;
    drag.moved += Math.abs(dx) + Math.abs(dy);
    if (drag.moved < CLICK_SLOP || this.#mode === 'player') return;
    if (this.#mode === 'top') this.#top = topPan(this.#top, dx, dy, this.#height);
    else if (drag.button === 2 || event.shiftKey) {
      this.setAuto(false);
      this.#orbit = orbitPan(this.#orbit, dx, dy, unitsPerPixelAt(this.#orbit.distance, ORBIT_FOV, this.#height));
    } else this.#orbit = orbitDrag(this.#orbit, dx, dy);
    this.#apply();
    this.#onChange();
  }

  #up(event: PointerEvent): void {
    const drag = this.#drag;
    if (drag === null || drag.id !== event.pointerId) return;
    this.#drag = null;
    if (this.#canvas.hasPointerCapture(event.pointerId)) this.#canvas.releasePointerCapture(event.pointerId);
    if (drag.button === 0 && drag.moved < CLICK_SLOP) {
      const box = this.#canvas.getBoundingClientRect();
      this.#onClick(event.clientX - box.left, event.clientY - box.top);
    }
  }

  #wheel(event: WheelEvent): void {
    if (this.#mode === 'player') return;
    event.preventDefault();
    if (this.#mode === 'top') this.#top = topZoom(this.#top, event.deltaY);
    else {
      this.setAuto(false);
      this.#orbit = orbitZoom(this.#orbit, event.deltaY);
    }
    this.#apply();
    this.#onChange();
  }
}
