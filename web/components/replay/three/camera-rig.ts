import { OrthographicCamera, PerspectiveCamera, Vector3 } from 'three';
import type { Camera } from 'three';
import {
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

/**
 * The three cameras and the pointer controls: left drag orbits (pans in the
 * top view), right drag pans, the wheel zooms. The player camera is placed by
 * the scene each frame and ignores drags.
 */
export class CameraRig {
  readonly orbitCamera = new PerspectiveCamera(ORBIT_FOV, 1, 4, 60000);
  readonly topCamera = new OrthographicCamera(-1, 1, 1, -1, 1, 80000);
  readonly playerCamera = new PerspectiveCamera(PLAYER_FOV, 1, 2, 40000);
  mode: CameraMode = 'orbit';
  #orbit: OrbitState = { target: { x: 0, y: 0, z: 0 }, distance: 4000, azimuth: 0, elevation: 0.8 };
  #top: TopState = { x: 0, y: 0, halfHeight: 2000 };
  #topZ = 4000;
  #width = 1;
  #height = 1;
  #drag: { id: number; button: number; x: number; y: number; moved: number } | null = null;
  readonly #canvas: HTMLCanvasElement;
  readonly #onChange: () => void;
  readonly #onClick: (x: number, y: number) => void;
  readonly #abort = new AbortController();

  constructor(canvas: HTMLCanvasElement, handlers: { onChange: () => void; onClick: (x: number, y: number) => void }) {
    this.#canvas = canvas;
    this.#onChange = handlers.onChange;
    this.#onClick = handlers.onClick;
    for (const camera of [this.orbitCamera, this.playerCamera]) camera.up.set(0, 0, 1);
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

  get active(): Camera {
    if (this.mode === 'top') return this.topCamera;
    if (this.mode === 'player') return this.playerCamera;
    return this.orbitCamera;
  }

  /** Points both overview cameras at a box. */
  frame(box: WorldBox): void {
    const aspect = this.#width / this.#height;
    this.#orbit = framingOrbit(box, ORBIT_FOV, aspect);
    this.#top = framingTop(box, aspect);
    this.#topZ = box.max.z + 2000;
    this.#apply();
  }

  resize(width: number, height: number): void {
    this.#width = Math.max(1, width);
    this.#height = Math.max(1, height);
    const aspect = this.#width / this.#height;
    this.orbitCamera.aspect = aspect;
    this.orbitCamera.updateProjectionMatrix();
    this.playerCamera.aspect = aspect;
    this.playerCamera.updateProjectionMatrix();
    this.#apply();
  }

  /** The player camera at an eye, looking along a unit direction. */
  placeEye(eye: Vec3, direction: Vec3): void {
    this.playerCamera.position.set(eye.x, eye.y, eye.z);
    this.playerCamera.lookAt(new Vector3(eye.x + direction.x, eye.y + direction.y, eye.z + direction.z));
  }

  dispose(): void {
    this.#abort.abort();
  }

  #apply(): void {
    const at = orbitPosition(this.#orbit);
    this.orbitCamera.position.set(at.x, at.y, at.z);
    this.orbitCamera.lookAt(this.#orbit.target.x, this.#orbit.target.y, this.#orbit.target.z);
    const aspect = this.#width / this.#height;
    const half = this.#top.halfHeight;
    this.topCamera.left = -half * aspect;
    this.topCamera.right = half * aspect;
    this.topCamera.top = half;
    this.topCamera.bottom = -half;
    this.topCamera.position.set(this.#top.x, this.#top.y, this.#topZ);
    this.topCamera.lookAt(this.#top.x, this.#top.y, 0);
    this.topCamera.updateProjectionMatrix();
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
    if (drag.moved < CLICK_SLOP || this.mode === 'player') return;
    if (this.mode === 'top') this.#top = topPan(this.#top, dx, dy, this.#height);
    else if (drag.button === 2 || event.shiftKey) {
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
    if (this.mode === 'player') return;
    event.preventDefault();
    if (this.mode === 'top') this.#top = topZoom(this.#top, event.deltaY);
    else this.#orbit = orbitZoom(this.#orbit, event.deltaY);
    this.#apply();
    this.#onChange();
  }
}
