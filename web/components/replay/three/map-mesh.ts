import { BufferAttribute, BufferGeometry, Color, DoubleSide, Mesh, MeshLambertMaterial, Plane, Vector3 } from 'three';
import type { MapGeometry } from '@/lib/api/map-geometry';
import { vertexColors, zRange } from '@/lib/replay/three/mesh';
import type { Rgb } from './scene-colors';

/**
 * The map: flat shaded so every face reads from its own slope, double sided
 * because the collision mesh's winding is not uniform, and cut by `plane` so
 * roofs can go while walls stay.
 */
export class MapMesh {
  readonly mesh: Mesh<BufferGeometry, MeshLambertMaterial>;
  readonly plane = new Plane(new Vector3(0, 0, -1), 0);
  readonly #vertices: Float32Array;
  readonly #range: [number, number];

  constructor(geometry: MapGeometry, color: Rgb) {
    this.#vertices = geometry.vertices;
    this.#range = zRange(geometry.vertices);
    const buffer = new BufferGeometry();
    buffer.setAttribute('position', new BufferAttribute(geometry.vertices, 3));
    buffer.setAttribute('color', new BufferAttribute(this.#colors(color), 3));
    buffer.setIndex(new BufferAttribute(geometry.triangles, 1));
    buffer.computeBoundingSphere();
    const material = new MeshLambertMaterial({
      vertexColors: true,
      flatShading: true,
      side: DoubleSide,
      clippingPlanes: [this.plane],
    });
    this.mesh = new Mesh(buffer, material);
    this.mesh.renderOrder = 0;
  }

  /** Hides everything above `z`. */
  setCut(z: number): void {
    this.plane.constant = z;
  }

  setColor(color: Rgb): void {
    const attribute = this.mesh.geometry.getAttribute('color');
    if (attribute instanceof BufferAttribute) {
      attribute.set(this.#colors(color));
      attribute.needsUpdate = true;
    }
  }

  dispose(): void {
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
  }

  #colors(color: Rgb): Float32Array {
    // Vertex colours are in the linear working space; the tokens are sRGB.
    const linear = new Color().setRGB(color[0], color[1], color[2], 'srgb');
    return vertexColors(this.#vertices, [linear.r, linear.g, linear.b], this.#range);
  }
}
