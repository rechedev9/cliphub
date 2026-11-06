import {
  BoxGeometry,
  BufferAttribute,
  BufferGeometry,
  CapsuleGeometry,
  Color,
  ConeGeometry,
  GreaterDepth,
  Group,
  Line,
  LineBasicMaterial,
  Mesh,
  MeshBasicMaterial,
  MeshLambertMaterial,
  RingGeometry,
  SphereGeometry,
} from 'three';
import type { Object3D } from 'three';
import { equipmentIconForWeapon, equipmentIconUrl } from '@/lib/api/game-assets';
import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '@/lib/api/tactical';
import type { TacticalSample } from '@/lib/api/tactical';
import { isAlive } from '@/lib/tactical-replay';
import { STANDING_HEIGHT, eyeHeight, hullHeight } from '@/lib/replay/three/scene-math';
import type { Vec3 } from '@/lib/replay/three/orbit';
import type { Rgb, SceneColors } from './scene-colors';

const BODY_RADIUS = 14;
const HEAD_RADIUS = 9;
/** The body capsule stops under the head; both scale with the hull when ducking. */
const BODY_TOP = STANDING_HEIGHT - 2 * HEAD_RADIUS - 2;
const RING_RADIUS = 20;
const CONE_LENGTH = 74;
const TRAIL_CAPACITY = 128;
/** Superposed rounds sit under the focused one at this strength, as on the radar. */
export const OVERLAY_OPACITY = 0.38;

type Side = 'ct' | 't';
type Tone = Side | 'bomb' | 'ct-head' | 't-head';

function sideOf(sample: TacticalSample): Side {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT) ? 't' : 'ct';
}

function color(rgb: Rgb): Color {
  return new Color().setRGB(rgb[0], rgb[1], rgb[2], 'srgb');
}

/** Geometry and materials every player shares; built once per scene. */
export class ActorKit {
  readonly body: CapsuleGeometry;
  readonly head: SphereGeometry;
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
    this.body = new CapsuleGeometry(BODY_RADIUS, BODY_TOP - 2 * BODY_RADIUS, 4, 14);
    this.body.rotateX(Math.PI / 2);
    this.body.translate(0, 0, BODY_TOP / 2);
    this.head = new SphereGeometry(HEAD_RADIUS, 16, 12);
    // An open cone along +X from the eye: the direction the player aims, pitch included.
    this.cone = new ConeGeometry(18, CONE_LENGTH, 20, 1, true);
    this.cone.rotateZ(Math.PI / 2);
    this.cone.translate(CONE_LENGTH / 2 + HEAD_RADIUS, 0, 0);
    this.bar = new BoxGeometry(36, 6, 2);
    this.bombBox = new BoxGeometry(14, 10, 10);
    this.ring = new RingGeometry(RING_RADIUS, RING_RADIUS + 6, 48);
  }

  lit(tone: Tone, faded: boolean): MeshLambertMaterial {
    return cached(this.#lambert, `${tone}|lit|${faded}`, () => new MeshLambertMaterial({ color: this.#color(tone), transparent: faded, opacity: faded ? OVERLAY_OPACITY : 1 }));
  }

  /** The figure where a wall hides it: drawn only behind the map, so a visible player keeps its shading. */
  ghost(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|ghost|${faded}`, () => new MeshBasicMaterial({
      color: this.#color(side),
      transparent: true,
      opacity: faded ? 0.15 : 0.55,
      depthFunc: GreaterDepth,
      depthWrite: false,
    }));
  }

  coneMaterial(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|cone|${faded}`, () => new MeshBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.08 : 0.26, depthWrite: false }));
  }

  cross(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|cross|${faded}`, () => new MeshBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.25 : 0.7, depthTest: false, depthWrite: false }));
  }

  trail(side: Side, faded: boolean): LineBasicMaterial {
    return cached(this.#lines, `${side}|trail|${faded}`, () => new LineBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.25 : 0.7 }));
  }

  ringMaterial(side: Side, faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `${side}|ring|${faded}`, () => new MeshBasicMaterial({ color: this.#color(side), transparent: true, opacity: faded ? 0.3 : 0.95, depthTest: false, depthWrite: false }));
  }

  bomb(faded: boolean): MeshBasicMaterial {
    return cached(this.#basic, `bomb|body|${faded}`, () => new MeshBasicMaterial({ color: this.#color('bomb'), transparent: faded, opacity: faded ? OVERLAY_OPACITY : 1 }));
  }

  setColors(colors: SceneColors): void {
    this.#colors = colors;
    for (const map of [this.#lambert, this.#basic, this.#lines]) {
      for (const [key, material] of map) material.color.copy(this.#color(toneOf(key.split('|')[0] ?? 'bomb')));
    }
  }

  dispose(): void {
    for (const geometry of [this.body, this.head, this.cone, this.bar, this.bombBox, this.ring]) geometry.dispose();
    for (const map of [this.#lambert, this.#basic, this.#lines]) {
      for (const material of map.values()) material.dispose();
      map.clear();
    }
  }

  #color(tone: Tone): Color {
    if (tone === 'bomb') return color(this.#colors.bomb);
    const side = tone === 't' || tone === 't-head' ? this.#colors.t : this.#colors.ct;
    const base = color(side);
    // The head a shade lighter than the body, so the figure reads from above.
    return tone.endsWith('-head') ? base.lerp(new Color(1, 1, 1), 0.35) : base;
  }
}

function toneOf(key: string): Tone {
  return key === 'ct' || key === 't' || key === 'ct-head' || key === 't-head' ? key : 'bomb';
}

function cached<M>(map: Map<string, M>, key: string, make: () => M): M {
  const found = map.get(key);
  if (found !== undefined) return found;
  const made = make();
  map.set(key, made);
  return made;
}

/** One player of one round: body, head, aim cone, ring, death cross, bomb, trail and name tag with the gun in hand. */
export class PlayerActor {
  readonly group = new Group();
  readonly body: Mesh;
  readonly label: HTMLSpanElement;
  readonly #head: Mesh;
  readonly #ghostBody: Mesh;
  readonly #ghostHead: Mesh;
  readonly #aim = new Group();
  readonly #cone: Mesh;
  readonly #cross: Group;
  readonly #bomb: Mesh;
  readonly #ring: Mesh;
  readonly #trail: Line<BufferGeometry, LineBasicMaterial>;
  readonly #trailPositions = new Float32Array(TRAIL_CAPACITY * 3);
  readonly #anchor: Vec3 = { x: 0, y: 0, z: 0 };
  readonly #name: HTMLSpanElement;
  readonly #weapon: HTMLImageElement;
  #weaponIcon: string | null = null;
  #side: Side | null = null;
  #shown = false;

  constructor(
    readonly slot: number,
    readonly kit: ActorKit,
    readonly faded: boolean,
    overlay: HTMLElement,
    name: string,
  ) {
    this.body = new Mesh(kit.body, kit.lit('ct', faded));
    this.#head = new Mesh(kit.head, kit.lit('ct-head', faded));
    this.#ghostBody = new Mesh(kit.body, kit.ghost('ct', faded));
    this.#ghostHead = new Mesh(kit.head, kit.ghost('ct', faded));
    this.#ghostBody.renderOrder = 10;
    this.#ghostHead.renderOrder = 10;
    this.#cone = new Mesh(kit.cone, kit.coneMaterial('ct', faded));
    this.#cone.renderOrder = 4;
    this.#aim.add(this.#cone);
    const a = new Mesh(kit.bar, kit.cross('ct', faded));
    const b = new Mesh(kit.bar, kit.cross('ct', faded));
    a.rotation.z = Math.PI / 4;
    b.rotation.z = -Math.PI / 4;
    this.#cross = new Group();
    this.#cross.add(a, b);
    this.#cross.position.z = 3;
    this.#cross.renderOrder = 9;
    this.#bomb = new Mesh(kit.bombBox, kit.bomb(faded));
    this.#ring = new Mesh(kit.ring, kit.ringMaterial('ct', faded));
    this.#ring.position.z = 1.5;
    this.#ring.renderOrder = 9;
    this.group.add(this.body, this.#head, this.#ghostBody, this.#ghostHead, this.#aim, this.#cross, this.#bomb, this.#ring);
    const trailGeometry = new BufferGeometry();
    trailGeometry.setAttribute('position', new BufferAttribute(this.#trailPositions, 3));
    this.#trail = new Line(trailGeometry, kit.trail('ct', faded));
    this.#trail.frustumCulled = false;
    this.label = document.createElement('span');
    this.label.className = 'absolute top-0 left-0 hidden max-w-44 items-center gap-1 rounded-sm border-l-2 bg-[#0b0c0e]/85 px-1.5 text-meta leading-5 font-semibold text-white shadow-[var(--elev-2)] will-change-transform';
    this.#name = document.createElement('span');
    this.#name.className = 'min-w-0 truncate';
    this.#name.textContent = name;
    this.#weapon = document.createElement('img');
    this.#weapon.alt = '';
    this.#weapon.className = 'hidden h-3 w-auto max-w-9 shrink-0 object-contain opacity-90';
    this.#weapon.addEventListener('error', () => this.#weapon.classList.add('hidden'));
    this.#weapon.addEventListener('load', () => this.#weapon.classList.remove('hidden'));
    this.label.append(this.#name, this.#weapon);
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
    this.label.classList.remove('flex');
  }

  /** Shows or hides the name tag; the tag is a flex row while shown. */
  showLabel(shown: boolean): void {
    this.label.classList.toggle('hidden', !shown);
    this.label.classList.toggle('flex', shown);
  }

  /** Places the player at a sample; `self` hides the figure for the first-person camera. */
  update(sample: TacticalSample, yaw: number, options: { self: boolean; bombLayer: boolean; scale: number }): void {
    this.#shown = true;
    const side = sideOf(sample);
    if (side !== this.#side) this.#setSide(side);
    const alive = isAlive(sample);
    const figure = alive && !options.self;
    this.group.visible = true;
    this.group.position.set(sample.x, sample.y, sample.z);
    this.group.rotation.z = (yaw * Math.PI) / 180;
    // Far away the figure grows to stay readable, less in height so it never towers over the cut walls.
    const tall = Math.sqrt(options.scale);
    this.group.scale.set(options.scale, options.scale, tall);
    const height = hullHeight(sample);
    const squash = height / STANDING_HEIGHT;
    this.body.visible = figure;
    this.body.scale.set(1, 1, squash);
    this.#head.visible = figure;
    this.#head.position.z = height - HEAD_RADIUS;
    this.#ghostBody.visible = figure;
    this.#ghostBody.scale.copy(this.body.scale);
    this.#ghostHead.visible = figure;
    this.#ghostHead.position.copy(this.#head.position);
    this.#aim.visible = figure;
    this.#aim.position.z = eyeHeight(sample);
    this.#aim.rotation.y = ((sample.pitch ?? 0) * Math.PI) / 180;
    this.#cross.visible = !alive;
    this.#ring.visible = alive && !options.self;
    this.#bomb.visible = figure && options.bombLayer && hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb);
    this.#bomb.position.set(-BODY_RADIUS - 5, 0, height * 0.55);
    this.#anchor.x = sample.x;
    this.#anchor.y = sample.y;
    this.#anchor.z = sample.z + (alive ? height * tall + 12 : 12);
    this.#setWeapon(alive ? (sample.activeWeapon ?? null) : null);
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

  #setWeapon(weapon: string | null): void {
    const icon = weapon === null ? null : equipmentIconForWeapon(weapon);
    if (icon === this.#weaponIcon) return;
    this.#weaponIcon = icon;
    if (icon === null) {
      this.#weapon.classList.add('hidden');
      this.#weapon.removeAttribute('src');
      return;
    }
    this.#weapon.src = equipmentIconUrl(icon);
  }

  #setSide(side: Side): void {
    this.#side = side;
    this.body.material = this.kit.lit(side, this.faded);
    this.#head.material = this.kit.lit(side === 't' ? 't-head' : 'ct-head', this.faded);
    this.#ghostBody.material = this.kit.ghost(side, this.faded);
    this.#ghostHead.material = this.kit.ghost(side, this.faded);
    this.#cone.material = this.kit.coneMaterial(side, this.faded);
    this.#ring.material = this.kit.ringMaterial(side, this.faded);
    for (const child of this.#cross.children) if (child instanceof Mesh) child.material = this.kit.cross(side, this.faded);
    this.#trail.material = this.kit.trail(side, this.faded);
    this.label.style.borderLeftColor = side === 'ct' ? 'var(--side-ct)' : 'var(--side-t)';
  }
}
