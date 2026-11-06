import { BufferAttribute, BufferGeometry, DoubleSide, EdgesGeometry, LineBasicMaterial, LineSegments, Mesh, MeshLambertMaterial, Plane, Vector3 } from 'three';
import type { MapGeometry } from '@/lib/api/map-geometry';
import { mapPalette, surfaceColors, zRange } from '@/lib/replay/three/mesh';
import type { Rgb } from './scene-colors';

/** Faces meeting at more than this angle get an edge line: corners and ledges, not the tessellation. */
const EDGE_ANGLE = 38;

/**
 * The map: floors and walls in two tones shaded by height, flat shaded so every
 * face reads from its slope, with soft edge lines on corners. Double sided
 * because the collision mesh's winding is not uniform, and cut by `plane` so
 * roofs can go while walls stay.
 */
export class MapMesh {
  readonly mesh: Mesh<BufferGeometry, MeshLambertMaterial>;
  readonly edges: LineSegments<EdgesGeometry, LineBasicMaterial>;
  readonly plane = new Plane(new Vector3(0, 0, -1), 0);
  readonly #positions: Float32Array;
  readonly #range: [number, number];

  constructor(geometry: MapGeometry, background: Rgb) {
    this.#range = zRange(geometry.vertices);
    const indexed = new BufferGeometry();
    indexed.setAttribute('position', new BufferAttribute(geometry.vertices, 3));
    indexed.setIndex(new BufferAttribute(geometry.triangles, 1));
    // Unindexed so each face carries its own floor or wall tone.
    const buffer = indexed.toNonIndexed();
    indexed.dispose();
    const position = buffer.getAttribute('position');
    this.#positions = position instanceof BufferAttribute && position.array instanceof Float32Array ? position.array : new Float32Array();
    buffer.setAttribute('color', new BufferAttribute(surfaceColors(this.#positions, mapPalette(background), this.#range), 3));
    buffer.computeBoundingSphere();
    const material = new MeshLambertMaterial({
      vertexColors: true,
      flatShading: true,
      side: DoubleSide,
      clippingPlanes: [this.plane],
      polygonOffset: true,
      polygonOffsetFactor: 1,
      polygonOffsetUnits: 1,
    });
    this.mesh = new Mesh(buffer, material);
    this.mesh.renderOrder = 0;
    this.edges = new LineSegments(new EdgesGeometry(buffer, EDGE_ANGLE), new LineBasicMaterial({ transparent: true, depthWrite: false, clippingPlanes: [this.plane] }));
    this.setColor(background);
  }

  get objects(): (Mesh | LineSegments)[] {
    return [this.mesh, this.edges];
  }

  setVisible(visible: boolean): void {
    this.mesh.visible = visible;
    this.edges.visible = visible;
  }

  /** Hides everything above `z`. */
  setCut(z: number): void {
    this.plane.constant = z;
  }

  /** Re-tones the map for the page behind it, after a theme change. */
  setColor(background: Rgb): void {
    const palette = mapPalette(background);
    const attribute = this.mesh.geometry.getAttribute('color');
    if (attribute instanceof BufferAttribute) {
      attribute.set(surfaceColors(this.#positions, palette, this.#range));
      attribute.needsUpdate = true;
    }
    const dark = palette.floor[0] < 0.6;
    this.edges.material.color.setRGB(dark ? 0.04 : 0.18, dark ? 0.045 : 0.19, dark ? 0.05 : 0.22);
    this.edges.material.opacity = dark ? 0.55 : 0.4;
  }

  dispose(): void {
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
    this.edges.geometry.dispose();
    this.edges.material.dispose();
  }
}
