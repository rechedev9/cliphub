import { NoToneMapping, OrthographicCamera, Scene, WebGLRenderer } from 'three';
import type { RadarCalibration, RadarLevel } from '@/lib/api/tactical';
import { NAV_RADAR_PALETTE } from '@/lib/replay/nav-radar';
import { buildFloorField } from '@/lib/replay/three/floor-field';
import type { RadarRect } from '@/lib/tactical-transform';
import { loadMapScene, loadNavGeometry } from '../three/geometry-cache';
import { disposeUniforms, mapUniforms } from '../three/map-shader';
import type { MapUniforms } from '../three/map-shader';
import { SLAB_CUT, WALL_CUT } from '../three/map-mesh';
import { DEFAULT_LOOK, RealMap } from '../three/real-map';

/** The floor field's cell, as the 3D view cuts roofs with. */
const FIELD_CELL = 32;
/** The camera sits this far over the map's top and sees this far under its bottom. */
const CAMERA_MARGIN = 2000;
/** The lower level is cut as if the action stood this far under the split. */
const LOWER_DEPTH = 64;

/**
 * The real CS2 map seen straight from above, for the 2D view: the same
 * surfaces, baked light and colour grading as the 3D view, with roofs and
 * ceilings over the playable floor cut away so the floor plan shows. It draws
 * only the window on screen, at device resolution, so it stays sharp at any zoom.
 */
export class RealPlate {
  readonly #renderer: WebGLRenderer;
  readonly #scene = new Scene();
  readonly #map: RealMap;
  readonly #cut: MapUniforms;
  readonly #top: number;
  readonly #depth: number;

  private constructor(renderer: WebGLRenderer, map: RealMap, cut: MapUniforms, range: { top: number; depth: number }) {
    this.#renderer = renderer;
    this.#map = map;
    this.#cut = cut;
    this.#top = range.top;
    this.#depth = range.depth;
    this.#scene.add(map.group);
  }

  /** Reads the map from CS2 through the orchestrator; rejects when it has no scene or WebGL is missing. */
  static async load(map: string): Promise<RealPlate> {
    const [scene, nav] = await Promise.all([loadMapScene(map), loadNavGeometry(map)]);
    const [minX, minY, minZ] = scene.min;
    const [maxX, maxY, maxZ] = scene.max;
    const field = nav === null ? null : buildFloorField(nav, { minX, minY, maxX, maxY }, FIELD_CELL);
    const cut = mapUniforms(field, { wall: WALL_CUT, slab: SLAB_CUT });
    const renderer = new WebGLRenderer({ antialias: true, preserveDrawingBuffer: true, powerPreference: 'high-performance' });
    renderer.toneMapping = NoToneMapping;
    renderer.setClearColor(NAV_RADAR_PALETTE.void);
    try {
      const real = new RealMap(scene, cut, renderer);
      real.setSurroundingsVisible(false);
      cut.uPlayableOnly.value = 1;
      real.setLook({ ...DEFAULT_LOOK, fogStrength: 0 });
      return new RealPlate(renderer, real, cut, { top: maxZ + CAMERA_MARGIN, depth: maxZ - minZ + 2 * CAMERA_MARGIN });
    } catch (error) {
      renderer.dispose();
      disposeUniforms(cut);
      throw error;
    }
  }

  /**
   * Draws the `view` window of a `size` px radar square onto `ctx` at `dpr`,
   * with the device transform reset; `level` picks the floor a stacked map is cut over.
   */
  draw(ctx: CanvasRenderingContext2D, options: { calibration: RadarCalibration; level: RadarLevel; size: number; dpr: number; view: RadarRect }): void {
    const { calibration, level, size, dpr, view } = options;
    const width = Math.max(1, Math.round(view.width * size * dpr));
    const height = Math.max(1, Math.round(view.height * size * dpr));
    const units = calibration.size * calibration.scale;
    const left = calibration.pos_x + view.x * units;
    const right = calibration.pos_x + (view.x + view.width) * units;
    const top = calibration.pos_y - view.y * units;
    const bottom = calibration.pos_y - (view.y + view.height) * units;
    const camera = new OrthographicCamera((left - right) / 2, (right - left) / 2, (top - bottom) / 2, (bottom - top) / 2, 1, this.#depth);
    camera.position.set((left + right) / 2, (top + bottom) / 2, this.#top);
    camera.updateMatrixWorld();
    const split = calibration.lower_altitude_max;
    this.#cut.uActionZ.value = level === 'lower' && split !== undefined ? split - LOWER_DEPTH : 1e9;
    this.#renderer.setPixelRatio(1);
    this.#renderer.setSize(width, height, false);
    this.#renderer.render(this.#scene, camera);
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.drawImage(this.#renderer.domElement, 0, 0);
  }

  dispose(): void {
    this.#map.dispose();
    disposeUniforms(this.#cut);
    this.#renderer.dispose();
    this.#renderer.forceContextLoss();
  }
}
