import {
  BufferAttribute,
  BufferGeometry,
  CircleGeometry,
  Color,
  Group,
  Line,
  LineDashedMaterial,
  Mesh,
  MeshBasicMaterial,
  MeshLambertMaterial,
  OctahedronGeometry,
  SphereGeometry,
} from 'three';
import { TACTICAL_EVENT_KINDS } from '@/lib/api/tactical';
import type { TacticalEventKind } from '@/lib/api/tactical';
import { grenadeArc } from '@/lib/replay/three/utility-visuals';
import type { BombVisual, UtilityVisual } from '@/lib/replay/three/utility-visuals';
import type { Rgb, SceneColors } from './scene-colors';

const ARC_STEPS = 32;
const SMOKE_RADIUS = 144;
const SMOKE_HEIGHT = 110;
const FIRE_RADIUS = 120;
const PULSE_RADIUS = 220;

function color(rgb: Rgb): Color {
  return new Color().setRGB(rgb[0], rgb[1], rgb[2], 'srgb');
}

function toneOf(kind: TacticalEventKind, colors: SceneColors): Rgb {
  if (kind === TACTICAL_EVENT_KINDS.flash) return colors.flash;
  if (kind === TACTICAL_EVENT_KINDS.he || kind === TACTICAL_EVENT_KINDS.molotov) return colors.bomb;
  return colors.smoke;
}

/** One grenade's objects: the effect volume, the estimated path and the grenade in flight. */
class GrenadeObject {
  readonly effect: Mesh<SphereGeometry | CircleGeometry, MeshBasicMaterial | MeshLambertMaterial>;
  readonly arc: Line<BufferGeometry, LineDashedMaterial> | null = null;
  readonly projectile: Mesh<SphereGeometry, MeshBasicMaterial>;
  readonly #arcPoints: number;
  used = true;

  constructor(readonly kind: TacticalEventKind, visual: UtilityVisual, shared: { sphere: SphereGeometry; disc: CircleGeometry; dot: SphereGeometry }, tone: Color) {
    const flat = kind === TACTICAL_EVENT_KINDS.molotov;
    // Smoke is lit so the cloud shades like a volume; fire and pops stay flat and bright.
    const material = kind === TACTICAL_EVENT_KINDS.smoke
      ? new MeshLambertMaterial({ color: tone, transparent: true, depthWrite: false, emissive: tone, emissiveIntensity: 0.25 })
      : new MeshBasicMaterial({ color: tone, transparent: true, depthWrite: false });
    this.effect = new Mesh(flat ? shared.disc : shared.sphere, material);
    this.effect.renderOrder = 3;
    this.projectile = new Mesh(shared.dot, new MeshBasicMaterial({ color: tone }));
    this.#arcPoints = ARC_STEPS + 1;
    if (visual.from !== null) {
      const points = grenadeArc(visual.from, visual.at, ARC_STEPS);
      const positions = new Float32Array(points.length * 3);
      points.forEach((point, i) => positions.set([point.x, point.y, point.z], i * 3));
      const geometry = new BufferGeometry();
      geometry.setAttribute('position', new BufferAttribute(positions, 3));
      // Dashed: the path is an estimate between two recorded points, not a recording.
      const line = new Line(geometry, new LineDashedMaterial({ color: tone, dashSize: 18, gapSize: 12, transparent: true }));
      line.computeLineDistances();
      line.frustumCulled = false;
      this.arc = line;
    }
  }

  get objects(): (Mesh | Line)[] {
    return this.arc === null ? [this.effect, this.projectile] : [this.effect, this.projectile, this.arc];
  }

  update(visual: UtilityVisual, strength: number): void {
    this.used = true;
    const { effect } = this;
    if (visual.effect === null) effect.visible = false;
    else {
      effect.visible = true;
      const grow = visual.effect.grow;
      if (this.kind === TACTICAL_EVENT_KINDS.smoke) {
        effect.position.set(visual.at.x, visual.at.y, visual.at.z + SMOKE_HEIGHT * 0.45 * grow);
        effect.scale.set(SMOKE_RADIUS * grow, SMOKE_RADIUS * grow, SMOKE_HEIGHT * grow);
        effect.material.opacity = 0.62 * visual.effect.strength * strength;
      } else if (this.kind === TACTICAL_EVENT_KINDS.molotov) {
        effect.position.set(visual.at.x, visual.at.y, visual.at.z + 2);
        effect.scale.setScalar(FIRE_RADIUS * grow);
        effect.material.opacity = 0.5 * visual.effect.strength * strength;
      } else {
        effect.position.set(visual.at.x, visual.at.y, visual.at.z + 24);
        effect.scale.setScalar(PULSE_RADIUS * grow);
        effect.material.opacity = 0.45 * visual.effect.strength * strength;
      }
    }
    const arc = visual.arc;
    if (this.arc === null || arc === null) {
      if (this.arc !== null) this.arc.visible = false;
      this.projectile.visible = false;
      return;
    }
    this.arc.visible = true;
    this.arc.material.opacity = 0.75 * arc.fade * strength;
    this.arc.geometry.setDrawRange(0, Math.max(2, Math.ceil(arc.progress * this.#arcPoints)));
    const flying = arc.progress < 1;
    this.projectile.visible = flying;
    if (flying) {
      const point = grenadeArc(arc.from, arc.to, ARC_STEPS)[Math.round(arc.progress * ARC_STEPS)];
      if (point !== undefined) this.projectile.position.set(point.x, point.y, point.z);
    }
  }

  hide(): void {
    for (const object of this.objects) object.visible = false;
  }

  dispose(): void {
    this.effect.material.dispose();
    this.projectile.material.dispose();
    if (this.arc !== null) {
      this.arc.geometry.dispose();
      this.arc.material.dispose();
    }
  }
}

/** Every round's grenades and the bomb on the ground, pooled by round and event. */
export class UtilityLayer {
  readonly group = new Group();
  readonly #sphere = new SphereGeometry(1, 32, 20);
  readonly #disc = new CircleGeometry(1, 40);
  readonly #dot = new SphereGeometry(5, 8, 6);
  readonly #bombGeometry = new OctahedronGeometry(14);
  readonly #pool = new Map<string, GrenadeObject>();
  readonly #bombs = new Map<string, { marker: Mesh<OctahedronGeometry, MeshBasicMaterial>; blast: Mesh<SphereGeometry, MeshBasicMaterial> }>();
  #colors: SceneColors;

  constructor(colors: SceneColors) {
    this.#colors = colors;
  }

  /** Call before a frame's updates; `endFrame` hides what the frame did not touch. */
  beginFrame(): void {
    for (const object of this.#pool.values()) object.used = false;
    for (const bomb of this.#bombs.values()) {
      bomb.marker.visible = false;
      bomb.blast.visible = false;
    }
  }

  grenades(roundKey: string, visuals: readonly UtilityVisual[], strength: number): void {
    for (const visual of visuals) {
      const key = `${roundKey}|${visual.index}`;
      let object = this.#pool.get(key);
      if (object === undefined) {
        object = new GrenadeObject(visual.kind, visual, { sphere: this.#sphere, disc: this.#disc, dot: this.#dot }, color(toneOf(visual.kind, this.#colors)));
        this.#pool.set(key, object);
        this.group.add(...object.objects);
      }
      object.update(visual, strength);
    }
  }

  bomb(roundKey: string, visual: BombVisual | null, strength: number): void {
    if (visual === null) return;
    let bomb = this.#bombs.get(roundKey);
    if (bomb === undefined) {
      bomb = {
        marker: new Mesh(this.#bombGeometry, new MeshBasicMaterial({ transparent: true })),
        blast: new Mesh(this.#sphere, new MeshBasicMaterial({ color: color(this.#colors.bomb), transparent: true, depthWrite: false })),
      };
      this.#bombs.set(roundKey, bomb);
      this.group.add(bomb.marker, bomb.blast);
    }
    const { marker, blast } = bomb;
    if (visual.state === 'exploded') {
      blast.visible = true;
      blast.position.set(visual.at.x, visual.at.y, visual.at.z + 20);
      blast.scale.setScalar(420 * visual.effect.grow);
      blast.material.opacity = 0.5 * visual.effect.strength * strength;
      return;
    }
    marker.visible = true;
    marker.material.color.copy(color(visual.state === 'defused' ? this.#colors.defuse : this.#colors.bomb));
    marker.material.opacity = strength;
    marker.scale.setScalar(visual.state === 'dropped' ? 0.7 : 1);
    marker.position.set(visual.at.x, visual.at.y, visual.at.z + 16);
  }

  endFrame(): void {
    for (const object of this.#pool.values()) if (!object.used) object.hide();
  }

  /** Drops pooled objects of rounds no longer shown. */
  keepRounds(keys: ReadonlySet<string>): void {
    for (const [key, object] of this.#pool) {
      if (keys.has(key.slice(0, key.lastIndexOf('|')))) continue;
      this.group.remove(...object.objects);
      object.dispose();
      this.#pool.delete(key);
    }
    for (const [key, bomb] of this.#bombs) {
      if (keys.has(key)) continue;
      this.group.remove(bomb.marker, bomb.blast);
      bomb.marker.material.dispose();
      bomb.blast.material.dispose();
      this.#bombs.delete(key);
    }
  }

  setColors(colors: SceneColors): void {
    this.#colors = colors;
    this.keepRounds(new Set());
  }

  dispose(): void {
    this.keepRounds(new Set());
    this.#sphere.dispose();
    this.#disc.dispose();
    this.#dot.dispose();
    this.#bombGeometry.dispose();
  }
}
