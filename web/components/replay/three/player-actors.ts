import {
  BoxGeometry,
  BufferAttribute,
  BufferGeometry,
  CapsuleGeometry,
  Color,
  ConeGeometry,
  Group,
  Line,
  LineBasicMaterial,
  Mesh,
  MeshBasicMaterial,
  MeshLambertMaterial,
  RingGeometry,
} from 'three';
import type { Object3D } from 'three';
import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '@/lib/api/tactical';
import type { TacticalSample } from '@/lib/api/tactical';
import { isAlive } from '@/lib/tactical-replay';
import { STANDING_EYE, STANDING_HEIGHT, eyeHeight, hullHeight } from '@/lib/replay/three/scene-math';
import type { Vec3 } from '@/lib/replay/three/orbit';
import type { Rgb, SceneColors } from './scene-colors';

const CAPSULE_RADIUS = 18;
const TRAIL_CAPACITY = 128;
/** Superposed rounds sit under the focused one at this strength, as on the radar. */
export const OVERLAY_OPACITY = 0.38;

type Side = 'ct' | 't';

function sideOf(sample: TacticalSample): Side {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT) ? 't' : 'ct';
}

function color(rgb: Rgb): Color {
  return new Color().setRGB(rgb[0], rgb[1], rgb[2], 'srgb');
}

/** Geometry and materials every player shares; built once per scene. */
export class ActorKit {
  readonly capsule: CapsuleGeometry;
  readonly cone: ConeGeometry;
  readonly bar: BoxGeometry;
  readonly bombBox: BoxGeometry;
  readonly ring: RingGeometry;
  readonly #lambert = new Map<string, MeshLambertMaterial>();
  readonly #basic = new Map<string, MeshBasicMaterial>();
  readonly #lines = new Map<string, LineBasicMaterial>();
  #colors: SceneColors;

  constructor(colors: SceneColors) {
    this.#colors = colors;
    this.capsule = new CapsuleGeometry(CAPSULE_RADIUS, STANDING_HEIGHT - 2 * CAPSULE_RADIUS, 4, 12);
    this.capsule.rotateX(Math.PI / 2);
    this.capsule.translate(0, 0, STANDING_HEIGHT / 2);
    // A cone pointing along +X, starting at the capsule's edge.
    this.cone = new ConeGeometry(11, 56, 16);
    this.cone.rotateZ(-Math.PI / 2);
    this.cone.translate(CAPSULE_RADIUS + 28, 0, 0);
    this.bar = new BoxGeometry(44, 7, 3);
    this.bombBox = new BoxGeometry(14, 10, 10);
    this.ring = new RingGeometry(CAPSULE_RADIUS + 6, CAPSULE_RADIUS + 13, 40);
  }

  body(side: Side, faded: boolean): MeshLambertMaterial {
    return cached(this.#lambert, `${side}|body|${faded}`, () => new MeshLambertMaterial({ color: this.#color(side), transparent: faded, opacity: faded ? OVERLAY_OPACITY : 1 }));
  }

  coneMaterial(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|cone|${faded}`, () => new MeshBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.2 : 0.55, depthWrite: false }));
  }

  cross(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|cross|${faded}`, () => new MeshBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.3 : 0.85 }));
  }

  trail(side: Side, faded: boolean): LineBasicMaterial {
    return cached(this.#lines, `${side}|trail|${faded}`, () => new LineBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.25 : 0.7 }));
  }

  ringMaterial(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|ring|${faded}`, () => new MeshBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.3 : 0.9, depthWrite: false }));
  }

  bomb(faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `bomb|body|${faded}`, () => new MeshBasicMaterial({ color: this.#color('bomb'), transparent: faded, opacity: faded ? OVERLAY_OPACITY : 1 }));
  }

  setColors(colors: SceneColors): void {
    this.#colors = colors;
    for (const map of [this.#lambert, this.#basic, this.#lines]) {
      for (const [key, material] of map) material.color.copy(this.#color(key.split('|')[0] ?? 'bomb'));
    }
  }

  dispose(): void {
    this.capsule.dispose();
    this.cone.dispose();
    this.bar.dispose();
    this.bombBox.dispose();
    this.ring.dispose();
    for (const map of [this.#lambert, this.#basic, this.#lines]) {
      for (const material of map.values()) material.dispose();
      map.clear();
    }
  }

  #color(tone: string): Color {
    if (tone === 't') return color(this.#colors.t);
    if (tone === 'ct') return color(this.#colors.ct);
    return color(this.#colors.bomb);
  }
}

function cached<M>(map: Map<string, M>, key: string, make: () => M): M {
  const found = map.get(key);
  if (found !== undefined) return found;
  const made = make();
  map.set(key, made);
  return made;
}

/** One player of one round: capsule, view cone, death cross, bomb box, trail and name tag. */
export class PlayerActor {
  readonly group = new Group();
  readonly body: Mesh;
  readonly label: HTMLSpanElement;
  readonly #cone: Mesh;
  readonly #cross: Group;
  readonly #bomb: Mesh;
  readonly #ring: Mesh;
  readonly #trail: Line<BufferGeometry, LineBasicMaterial>;
  readonly #trailPositions = new Float32Array(TRAIL_CAPACITY * 3);
  readonly #anchor: Vec3 = { x: 0, y: 0, z: 0 };
  #side: Side | null = null;
  #shown = false;

  constructor(
    readonly slot: number,
    readonly kit: ActorKit,
    readonly faded: boolean,
    overlay: HTMLElement,
    name: string,
  ) {
    this.body = new Mesh(kit.capsule, kit.body('ct', faded));
    this.#cone = new Mesh(kit.cone, kit.coneMaterial('ct', faded));
    this.#cone.position.z = STANDING_EYE;
    const a = new Mesh(kit.bar, kit.cross('ct', faded));
    const b = new Mesh(kit.bar, kit.cross('ct', faded));
    a.rotation.z = Math.PI / 4;
    b.rotation.z = -Math.PI / 4;
    this.#cross = new Group();
    this.#cross.add(a, b);
    this.#cross.position.z = 2;
    this.#bomb = new Mesh(kit.bombBox, kit.bomb(faded));
    this.#ring = new Mesh(kit.ring, kit.ringMaterial('ct', faded));
    this.#ring.position.z = 1.5;
    this.#ring.renderOrder = 2;
    this.group.add(this.body, this.#cone, this.#cross, this.#bomb, this.#ring);
    const trailGeometry = new BufferGeometry();
    trailGeometry.setAttribute('position', new BufferAttribute(this.#trailPositions, 3));
    this.#trail = new Line(trailGeometry, kit.trail('ct', faded));
    this.#trail.frustumCulled = false;
    this.label = document.createElement('span');
    this.label.className = 'absolute top-0 left-0 hidden max-w-40 truncate rounded-sm border-l-2 bg-[#0b0c0e]/85 px-1.5 text-meta leading-5 font-semibold text-white shadow-[var(--elev-2)] will-change-transform';
    this.label.textContent = name;
    overlay.append(this.label);
    this.#setSide('ct');
    this.hide();
  }

  /** Objects to add to the scene once. */
  get objects(): Object3D[] {
    return [this.group, this.#trail];
  }

  get shown(): boolean {
    return this.#shown;
  }

  /** Where the name tag hangs, over the head. */
  get anchor(): Vec3 {
    return this.#anchor;
  }

  hide(): void {
    this.#shown = false;
    this.group.visible = false;
    this.#trail.visible = false;
    this.label.classList.add('hidden');
  }

  /** Places the player at a sample; `self` hides the body for the first-person camera. */
  update(sample: TacticalSample, yaw: number, options: { self: boolean; bombLayer: boolean; scale: number }): void {
    this.#shown = true;
    const side = sideOf(sample);
    if (side !== this.#side) this.#setSide(side);
    const alive = isAlive(sample);
    this.group.visible = true;
    this.group.position.set(sample.x, sample.y, sample.z);
    this.group.rotation.z = (yaw * Math.PI) / 180;
    this.group.scale.set(options.scale, options.scale, 1);
    const height = hullHeight(sample);
    this.body.visible = alive && !options.self;
    this.body.scale.set(1, 1, height / STANDING_HEIGHT);
    this.#cone.visible = alive && !options.self;
    this.#cone.position.z = eyeHeight(sample);
    this.#cross.visible = !alive;
    this.#ring.visible = alive;
    this.#bomb.visible = alive && !options.self && options.bombLayer && hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb);
    this.#bomb.position.set(-CAPSULE_RADIUS - 4, 0, height * 0.6);
    this.#anchor.x = sample.x;
    this.#anchor.y = sample.y;
    this.#anchor.z = sample.z + (alive ? height + 14 : 12);
  }

  setTrail(points: readonly Vec3[] | undefined): void {
    if (points === undefined || points.length < 2) {
      this.#trail.visible = false;
      return;
    }
    const count = Math.min(TRAIL_CAPACITY, points.length);
    const start = points.length - count;
    for (let i = 0; i < count; i += 1) {
      const point = points[start + i];
      this.#trailPositions[i * 3] = point.x;
      this.#trailPositions[i * 3 + 1] = point.y;
      this.#trailPositions[i * 3 + 2] = point.z + 4;
    }
    const attribute = this.#trail.geometry.getAttribute('position');
    attribute.needsUpdate = true;
    this.#trail.geometry.setDrawRange(0, count);
    this.#trail.visible = true;
  }

  dispose(): void {
    this.#trail.geometry.dispose();
    this.label.remove();
  }

  #setSide(side: Side): void {
    this.#side = side;
    this.body.material = this.kit.body(side, this.faded);
    this.#cone.material = this.kit.coneMaterial(side, this.faded);
    this.#ring.material = this.kit.ringMaterial(side, this.faded);
    for (const child of this.#cross.children) if (child instanceof Mesh) child.material = this.kit.cross(side, this.faded);
    this.#trail.material = this.kit.trail(side, this.faded);
    this.label.style.borderLeftColor = side === 'ct' ? 'var(--side-ct)' : 'var(--side-t)';
  }
}
