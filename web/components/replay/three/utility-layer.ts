import {
  AdditiveBlending,
  BufferAttribute,
  BufferGeometry,
  CircleGeometry,
  Color,
  Group,
  Line,
  LineDashedMaterial,
  Mesh,
  MeshBasicMaterial,
  OctahedronGeometry,
  RingGeometry,
  SRGBColorSpace,
  SphereGeometry,
  Sprite,
  SpriteMaterial,
  TextureLoader,
} from 'three';
import type { Object3D, Texture } from 'three';
import { equipmentIconForWeapon, equipmentIconUrl } from '@/lib/api/game-assets';
import { TACTICAL_EVENT_KINDS } from '@/lib/api/tactical';
import type { TacticalEventKind } from '@/lib/api/tactical';
import { flicker, grenadeArc, puffLayout } from '@/lib/replay/three/utility-visuals';
import type { BombVisual, Puff, UtilityVisual } from '@/lib/replay/three/utility-visuals';
import type { Rgb, SceneColors } from './scene-colors';
import { createUtilityTextures, disposeUtilityTextures } from './utility-textures';
import type { UtilityTextures } from './utility-textures';

const ARC_STEPS = 32;
const SMOKE_RADIUS = 144;
const SMOKE_HEIGHT = 128;
const SMOKE_PUFFS = 18;
const FIRE_RADIUS = 120;
const FIRE_TONGUES = 11;
const FLASH_RADIUS = 380;
const HE_RADIUS = 300;
/** On-screen height of a grenade in flight, as a share of the view's height over 2 tan(fov/2). */
const ICON_SCALE = 0.03;
/** Fire reads as fire on any floor tone: a fixed orange, blended additively. */
const FIRE_TONE: Rgb = [1, 0.5, 0.15];
const HE_TONE: Rgb = [1, 0.62, 0.3];
/** Smoke is a fixed cool white with a shadow under it, so it reads on light and dark floors alike. */
const SMOKE_TONE: Rgb = [0.88, 0.9, 0.94];

const ICON_NAMES: Partial<Record<TacticalEventKind, string>> = {
  [TACTICAL_EVENT_KINDS.smoke]: 'smokegrenade',
  [TACTICAL_EVENT_KINDS.flash]: 'flashbang',
  [TACTICAL_EVENT_KINDS.he]: 'hegrenade',
  [TACTICAL_EVENT_KINDS.molotov]: 'molotov',
  [TACTICAL_EVENT_KINDS.decoy]: 'decoy',
};

function color(rgb: Rgb): Color {
  return new Color().setRGB(rgb[0], rgb[1], rgb[2], 'srgb');
}

function toneOf(kind: TacticalEventKind, colors: SceneColors): Rgb {
  if (kind === TACTICAL_EVENT_KINDS.flash) return colors.flash;
  if (kind === TACTICAL_EVENT_KINDS.molotov) return FIRE_TONE;
  if (kind === TACTICAL_EVENT_KINDS.he) return HE_TONE;
  if (kind === TACTICAL_EVENT_KINDS.smoke) return SMOKE_TONE;
  return colors.smoke;
}

function sprite(map: Texture, tone: Color, additive: boolean): Sprite {
  const made = new Sprite(new SpriteMaterial({ map, color: tone, transparent: true, depthWrite: false, blending: additive ? AdditiveBlending : undefined }));
  made.renderOrder = 3;
  return made;
}

type Shared = { textures: UtilityTextures; disc: CircleGeometry; ring: RingGeometry; icon: (kind: TacticalEventKind) => Texture | null };

/**
 * One grenade: its effect (a smoke cloud of soft puffs, flame tongues over a
 * burning floor, a flash pop, an HE shockwave), the estimated throw and the
 * grenade's icon in flight.
 */
class GrenadeObject {
  readonly effect = new Group();
  readonly arc: Line<BufferGeometry, LineDashedMaterial> | null = null;
  readonly projectile: Sprite;
  readonly #shadow: Sprite;
  readonly #puffs: Puff[];
  readonly #sprites: Sprite[] = [];
  readonly #glow: Sprite | null = null;
  readonly #floor: Mesh<CircleGeometry | RingGeometry, MeshBasicMaterial> | null = null;
  readonly #path: ReturnType<typeof grenadeArc> = [];
  readonly #shared: Shared;
  #iconReady = false;
  used = true;

  constructor(readonly kind: TacticalEventKind, visual: UtilityVisual, shared: Shared, tone: Color) {
    this.#shared = shared;
    const { textures } = shared;
    this.#puffs = puffLayout(visual.index * 7 + 3, kind === TACTICAL_EVENT_KINDS.smoke ? SMOKE_PUFFS : FIRE_TONGUES);
    if (kind === TACTICAL_EVENT_KINDS.smoke) {
      this.#floor = new Mesh(shared.disc, new MeshBasicMaterial({ map: textures.glow, color: new Color(0, 0, 0), transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -4 }));
      this.#floor.renderOrder = 2;
      this.effect.add(this.#floor);
      for (const puff of this.#puffs) {
        // Lower puffs darker, so the cloud has a grey belly under a lit top.
        const shade = tone.clone().multiplyScalar(0.55 + 0.45 * Math.min(1, puff.z * 1.4));
        const made = sprite(textures.puff, shade, false);
        this.#sprites.push(made);
        this.effect.add(made);
      }
    } else if (kind === TACTICAL_EVENT_KINDS.molotov) {
      this.#floor = new Mesh(shared.disc, new MeshBasicMaterial({ map: textures.fireFloor, transparent: true, depthWrite: false, blending: AdditiveBlending, polygonOffset: true, polygonOffsetFactor: -4 }));
      this.#floor.renderOrder = 2;
      this.effect.add(this.#floor);
      for (let i = 0; i < this.#puffs.length; i += 1) {
        const made = sprite(textures.flame, tone, true);
        made.center.set(0.5, 0.05);
        this.#sprites.push(made);
        this.effect.add(made);
      }
      this.#glow = sprite(textures.glow, tone, true);
      this.effect.add(this.#glow);
    } else {
      this.#glow = sprite(textures.glow, tone, true);
      this.effect.add(this.#glow);
      if (kind !== TACTICAL_EVENT_KINDS.flash) {
        this.#floor = new Mesh(shared.ring, new MeshBasicMaterial({ color: tone, transparent: true, depthWrite: false, blending: AdditiveBlending }));
        this.effect.add(this.#floor);
      }
    }
    this.projectile = sprite(textures.dot, tone, false);
    this.projectile.material.sizeAttenuation = false;
    this.projectile.renderOrder = 12;
    this.#shadow = sprite(textures.glow, new Color(0, 0, 0), false);
    this.#shadow.material.sizeAttenuation = false;
    this.#shadow.material.opacity = 0.7;
    this.#shadow.renderOrder = 11;
    if (visual.from !== null) {
      this.#path = grenadeArc(visual.from, visual.at, ARC_STEPS);
      const positions = new Float32Array(this.#path.length * 3);
      this.#path.forEach((point, i) => positions.set([point.x, point.y, point.z], i * 3));
      const geometry = new BufferGeometry();
      geometry.setAttribute('position', new BufferAttribute(positions, 3));
      // Dashed: the path is an estimate between two recorded points, not a recording.
      const line = new Line(geometry, new LineDashedMaterial({ color: tone, dashSize: 18, gapSize: 12, transparent: true }));
      line.computeLineDistances();
      line.frustumCulled = false;
      this.arc = line;
    }
  }

  get objects(): Object3D[] {
    const objects: Object3D[] = [this.effect, this.#shadow, this.projectile];
    if (this.arc !== null) objects.push(this.arc);
    return objects;
  }

  update(visual: UtilityVisual, strength: number, now: number): void {
    this.used = true;
    const effect = visual.effect;
    this.effect.visible = effect !== null;
    if (effect !== null) {
      this.effect.position.set(visual.at.x, visual.at.y, visual.at.z);
      if (this.kind === TACTICAL_EVENT_KINDS.smoke) this.#smoke(effect.grow, effect.strength * strength, now);
      else if (this.kind === TACTICAL_EVENT_KINDS.molotov) this.#fire(effect.grow, effect.strength * strength, now);
      else this.#pop(effect.grow, effect.strength * strength);
    }
    this.#flight(visual, strength);
  }

  hide(): void {
    for (const object of this.objects) object.visible = false;
  }

  dispose(): void {
    for (const made of [...this.#sprites, this.projectile, this.#shadow]) made.material.dispose();
    this.#glow?.material.dispose();
    this.#floor?.material.dispose();
    if (this.arc !== null) {
      this.arc.geometry.dispose();
      this.arc.material.dispose();
    }
  }

  /** Puffs spread out and rise as the cloud blooms, drift slowly and thin out together at the end. */
  #smoke(grow: number, opacity: number, now: number): void {
    const spread = 0.45 + 0.55 * grow;
    this.#puffs.forEach((puff, i) => {
      const made = this.#sprites[i];
      const drift = Math.sin(now * 0.4 + puff.phase) * 6;
      made.position.set(puff.x * SMOKE_RADIUS * spread + drift, puff.y * SMOKE_RADIUS * spread - drift, 18 + puff.z * SMOKE_HEIGHT * grow);
      made.scale.setScalar(puff.size * SMOKE_RADIUS * (0.55 + 0.45 * grow));
      made.material.rotation = puff.phase + now * 0.06;
      made.material.opacity = 0.85 * opacity;
    });
    const floor = this.#floor;
    if (floor !== null) {
      floor.position.z = 3;
      floor.scale.setScalar(SMOKE_RADIUS * 1.5 * spread);
      floor.material.opacity = 0.55 * opacity;
    }
  }

  /** Flame tongues flicker over a burning floor that spreads, with a warm glow over all of it. */
  #fire(grow: number, opacity: number, now: number): void {
    const floor = this.#floor;
    if (floor !== null) {
      floor.position.z = 2;
      floor.scale.setScalar(FIRE_RADIUS * grow);
      floor.material.opacity = 0.8 * opacity;
    }
    this.#puffs.forEach((puff, i) => {
      const made = this.#sprites[i];
      const height = flicker(now, puff.phase);
      made.position.set(puff.x * FIRE_RADIUS * grow, puff.y * FIRE_RADIUS * grow, 2);
      made.scale.set(30 * puff.size, 70 * puff.size * height * (0.5 + 0.5 * grow), 1);
      made.material.opacity = 0.9 * opacity;
    });
    if (this.#glow !== null) {
      this.#glow.position.z = 36;
      this.#glow.scale.setScalar(FIRE_RADIUS * 3 * grow);
      this.#glow.material.opacity = 0.32 * opacity * flicker(now * 0.7, 0.4);
    }
  }

  /** A flash pops white; an HE flares and sends a ring along the floor. */
  #pop(grow: number, opacity: number): void {
    const flash = this.kind === TACTICAL_EVENT_KINDS.flash;
    if (this.#glow !== null) {
      this.#glow.position.z = flash ? 0 : 24;
      this.#glow.scale.setScalar((flash ? FLASH_RADIUS : HE_RADIUS * 0.8) * (0.4 + 0.6 * grow));
      this.#glow.material.opacity = (flash ? 1 : 0.8) * opacity;
    }
    const floor = this.#floor;
    if (floor !== null) {
      const reach = this.kind === TACTICAL_EVENT_KINDS.he ? HE_RADIUS : 90;
      floor.position.z = 4;
      floor.scale.setScalar(reach * grow);
      floor.material.opacity = opacity;
    }
  }

  #flight(visual: UtilityVisual, strength: number): void {
    const arc = visual.arc;
    if (this.arc === null || arc === null) {
      if (this.arc !== null) this.arc.visible = false;
      this.projectile.visible = false;
      this.#shadow.visible = false;
      return;
    }
    this.arc.visible = true;
    this.arc.material.opacity = 0.75 * arc.fade * strength;
    this.arc.geometry.setDrawRange(0, Math.max(2, Math.ceil(arc.progress * (ARC_STEPS + 1))));
    const flying = arc.progress < 1;
    this.projectile.visible = flying;
    this.#shadow.visible = flying;
    if (!flying) return;
    const at = arc.progress * ARC_STEPS;
    const a = this.#path[Math.floor(at)];
    const b = this.#path[Math.min(ARC_STEPS, Math.floor(at) + 1)];
    if (a === undefined || b === undefined) return;
    const t = at - Math.floor(at);
    this.projectile.position.set(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t, a.z + (b.z - a.z) * t);
    this.#shadow.position.copy(this.projectile.position);
    this.#useIcon();
    const image = this.projectile.material.map?.image;
    const aspect = this.#iconReady && image instanceof HTMLImageElement && image.naturalHeight > 0 ? image.naturalWidth / image.naturalHeight : 1;
    this.projectile.scale.set(ICON_SCALE * aspect, ICON_SCALE, 1);
    this.#shadow.scale.set(ICON_SCALE * 1.9 * Math.max(1, aspect), ICON_SCALE * 1.9, 1);
    this.projectile.material.opacity = strength;
  }

  #useIcon(): void {
    if (this.#iconReady) return;
    const icon = this.#shared.icon(this.kind);
    if (icon === null) return;
    this.#iconReady = true;
    this.projectile.material.map = icon;
    this.projectile.material.needsUpdate = true;
  }
}

/** Every round's grenades and the bomb on the ground, pooled by round and event. */
export class UtilityLayer {
  readonly group = new Group();
  readonly #textures = createUtilityTextures();
  readonly #disc = new CircleGeometry(1, 48);
  readonly #ring = new RingGeometry(0.86, 1, 64);
  readonly #sphere = new SphereGeometry(1, 32, 20);
  readonly #bombGeometry = new OctahedronGeometry(14);
  readonly #icons = new Map<string, Texture | null>();
  readonly #pool = new Map<string, GrenadeObject>();
  readonly #bombs = new Map<string, { marker: Mesh<OctahedronGeometry, MeshBasicMaterial>; blast: Mesh<SphereGeometry, MeshBasicMaterial> }>();
  readonly #onLoad: () => void;
  #colors: SceneColors;

  constructor(colors: SceneColors, onLoad: () => void) {
    this.#colors = colors;
    this.#onLoad = onLoad;
  }

  /** Call before a frame's updates; `endFrame` hides what the frame did not touch. */
  beginFrame(): void {
    for (const object of this.#pool.values()) object.used = false;
    for (const bomb of this.#bombs.values()) {
      bomb.marker.visible = false;
      bomb.blast.visible = false;
    }
  }

  grenades(roundKey: string, visuals: readonly UtilityVisual[], strength: number, now: number): void {
    for (const visual of visuals) {
      const key = `${roundKey}|${visual.index}`;
      let object = this.#pool.get(key);
      if (object === undefined) {
        const shared = { textures: this.#textures, disc: this.#disc, ring: this.#ring, icon: (kind: TacticalEventKind) => this.#icon(kind) };
        object = new GrenadeObject(visual.kind, visual, shared, color(toneOf(visual.kind, this.#colors)));
        this.#pool.set(key, object);
        this.group.add(...object.objects);
      }
      object.update(visual, strength, now);
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
    disposeUtilityTextures(this.#textures);
    for (const icon of this.#icons.values()) icon?.dispose();
    this.#disc.dispose();
    this.#ring.dispose();
    this.#sphere.dispose();
    this.#bombGeometry.dispose();
  }

  /** The game's icon for a grenade kind, loaded once; null until it arrives or when CS2 has none. */
  #icon(kind: TacticalEventKind): Texture | null {
    const name = ICON_NAMES[kind];
    if (name === undefined) return null;
    if (this.#icons.has(name)) return this.#icons.get(name) ?? null;
    this.#icons.set(name, null);
    const icon = equipmentIconForWeapon(name);
    if (icon === null) return null;
    new TextureLoader().load(
      equipmentIconUrl(icon),
      (texture) => {
        texture.colorSpace = SRGBColorSpace;
        this.#icons.set(name, texture);
        this.#onLoad();
      },
      undefined,
      () => undefined,
    );
    return null;
  }
}
