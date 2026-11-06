import { BufferAttribute, BufferGeometry, DoubleSide, LineBasicMaterial, LineSegments, Mesh, MeshLambertMaterial, SRGBColorSpace, Texture } from 'three';
import type { MapGeometry } from '@/lib/api/map-geometry';
import type { RadarCalibration } from '@/lib/api/tactical';
import { creaseEdges } from '@/lib/replay/three/crease-edges';
import { markWalls } from '@/lib/replay/three/floor-field';
import type { FloorField } from '@/lib/replay/three/floor-field';
import { mapPalette, surfaceColors, zRange } from '@/lib/replay/three/mesh';
import type { Vec3 } from '@/lib/replay/three/orbit';
import { blankTexture, disposeUniforms, mapUniforms, patchLines, patchSurface } from './map-shader';
import type { MapUniforms } from './map-shader';
import type { Rgb } from './scene-colors';

/** Faces meeting at more than this angle get an edge line: corners and ledges, not the tessellation. */
const EDGE_ANGLE = 42;
const EDGE_MIN_LENGTH = 12;
/** Walls stop this high over the floor under them; flat faces higher than `SLAB_CUT` go. */
export const WALL_CUT = 160;
export const SLAB_CUT = 100;

/**
 * The map: floors toned by height band like the 2D esquema, walls a step
 * darker, flat shaded, with crease lines on corners and ledges. Double sided
 * because the collision mesh's winding is not uniform. With a floor field it
 * is cut per pixel over the local floor, so roofs go and rooms stay open.
 */
export class MapMesh {
  readonly mesh: Mesh<BufferGeometry, MeshLambertMaterial>;
  readonly edges: LineSegments<BufferGeometry, LineBasicMaterial>;
  readonly #uniforms: MapUniforms;
  readonly #positions: Float32Array;
  readonly #range: [number, number];

  constructor(geometry: MapGeometry, field: FloorField | null, background: Rgb) {
    const indexed = new BufferGeometry();
    indexed.setAttribute('position', new BufferAttribute(geometry.vertices, 3));
    indexed.setIndex(new BufferAttribute(geometry.triangles, 1));
    // Unindexed so each face carries its own floor or wall tone.
    const buffer = indexed.toNonIndexed();
    indexed.dispose();
    const position = buffer.getAttribute('position');
    this.#positions = position instanceof BufferAttribute && position.array instanceof Float32Array ? position.array : new Float32Array();
    this.#range = field === null ? zRange(geometry.vertices) : [field.lowZ, field.highZ];
    if (field !== null) markWalls(field, this.#positions);
    this.#uniforms = mapUniforms(field, { wall: WALL_CUT, slab: SLAB_CUT });
    buffer.setAttribute('color', new BufferAttribute(surfaceColors(this.#positions, mapPalette(background), this.#range), 3));
    buffer.computeBoundingSphere();
    const material = new MeshLambertMaterial({
      vertexColors: true,
      flatShading: true,
      side: DoubleSide,
      polygonOffset: true,
      polygonOffsetFactor: 1,
      polygonOffsetUnits: 1,
    });
    patchSurface(material, this.#uniforms);
    this.mesh = new Mesh(buffer, material);
    this.mesh.renderOrder = 0;
    const crease = creaseEdges(this.#positions, { angle: EDGE_ANGLE, minLength: EDGE_MIN_LENGTH });
    const lines = new BufferGeometry();
    lines.setAttribute('position', new BufferAttribute(crease.positions, 3));
    lines.setAttribute('aVertical', new BufferAttribute(crease.vertical, 1));
    lines.computeBoundingSphere();
    const lineMaterial = new LineBasicMaterial({ transparent: true, depthWrite: false });
    patchLines(lineMaterial, this.#uniforms);
    this.edges = new LineSegments(lines, lineMaterial);
    this.setColor(background);
  }

  /** The cutaway uniforms, for another surface that must cut the same way. */
  get uniforms(): MapUniforms {
    return this.#uniforms;
  }

  get objects(): (Mesh | LineSegments)[] {
    return [this.mesh, this.edges];
  }

  /** Whether the map is cut over a floor field, or only by the global ceiling. */
  get local(): boolean {
    return this.#uniforms.uUseField.value > 0.5;
  }

  setVisible(visible: boolean): void {
    this.mesh.visible = visible;
    this.edges.visible = visible;
  }

  /** Hides everything above `z`; null lifts the ceiling. */
  setCeiling(z: number | null): void {
    this.#uniforms.uCeiling.value = z ?? 1e9;
  }

  /** Cuts stacked floors over the level at height `z`; null keeps the top level everywhere. */
  setActionLevel(z: number | null): void {
    this.#uniforms.uActionZ.value = z ?? 1e9;
  }

  /** Keeps the surface under each of these feet, so no living player floats over a cut roof or box. */
  setFeet(feet: readonly Vec3[]): void {
    this.#uniforms.uFeet.value.forEach((slot, i) => {
      const foot = feet[i];
      if (foot === undefined) slot.set(0, 0, 0, 0);
      else slot.set(foot.x, foot.y, foot.z, 1);
    });
  }

  /** Lays the official radar on the floors from above; `mix` 0 hides it. */
  setRadar(image: HTMLImageElement | null, calibration: RadarCalibration | null): void {
    this.#uniforms.uRadar.value.dispose();
    if (image === null || calibration === null || !(calibration.scale > 0)) {
      this.#uniforms.uRadar.value = blankTexture();
      this.#uniforms.uRadarMix.value = 0;
      return;
    }
    const texture = new Texture(image);
    texture.colorSpace = SRGBColorSpace;
    texture.anisotropy = 8;
    texture.needsUpdate = true;
    this.#uniforms.uRadar.value = texture;
    this.#uniforms.uRadarRect.value.set(calibration.pos_x, calibration.pos_y, calibration.scale * image.naturalWidth, calibration.scale * image.naturalHeight);
  }

  /** Whether walls and roofs are cut over the floor; the first-person camera wants the whole map. */
  setCutaway(on: boolean): void {
    this.#uniforms.uCutaway.value = on ? 1 : 0;
  }

  setRadarMix(mix: number): void {
    this.#uniforms.uRadarMix.value = mix;
  }

  /** Re-tones the map for the page behind it, after a theme change. */
  setColor(background: Rgb): void {
    const palette = mapPalette(background);
    this.#uniforms.uVoid.value.set(background[0], background[1], background[2]);
    const attribute = this.mesh.geometry.getAttribute('color');
    if (attribute instanceof BufferAttribute) {
      attribute.set(surfaceColors(this.#positions, palette, this.#range));
      attribute.needsUpdate = true;
    }
    const dark = palette.low[0] < 0.5;
    this.edges.material.color.setRGB(dark ? 0.03 : 0.16, dark ? 0.035 : 0.17, dark ? 0.045 : 0.2);
    this.edges.material.opacity = dark ? 0.6 : 0.45;
  }

  dispose(): void {
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
    this.edges.geometry.dispose();
    this.edges.material.dispose();
    disposeUniforms(this.#uniforms);
  }
}
