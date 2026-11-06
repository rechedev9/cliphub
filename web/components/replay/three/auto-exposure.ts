import { FloatType, NearestFilter, RGBAFormat, WebGLRenderTarget } from 'three';
import type { Camera, Object3D, WebGLRenderer } from 'three';
import type { ScenePost } from '@/lib/api/map-scene';

/** The grey CS2's auto exposure brings the frame's average luminance to. */
const MIDDLE_GREY = 0.18;
/** The frame is metered at this size: enough for an average, cheap to read back. */
const METER_WIDTH = 64;
const METER_HEIGHT = 36;
/** At most one meter read this often, in ms; the readback stalls the GPU. */
const METER_EVERY = 150;
/** Seconds for the exposure to cover most of the way to its target, as the eye adapts. */
const ADAPT_SECONDS = 0.6;
/** Darker texels than this are left out of the average: the void and unlit holes. */
const DARKEST = 1e-4;

/**
 * CS2's auto exposure: the scene is rendered small with every surface writing
 * its unexposed luminance (`uMeter` = 1), the log average of that sets the
 * exposure that brings it to middle grey, held to the map's volume range, and
 * the shown exposure eases towards it over time.
 */
export class AutoExposure {
  readonly #target: WebGLRenderTarget | null;
  readonly #pixels = new Float32Array(METER_WIDTH * METER_HEIGHT * 4);
  readonly #min: number;
  readonly #max: number;
  readonly #bias: number;
  #goal: number;
  #value: number;
  #metered = -Infinity;
  #adapted = 0;

  constructor(options: { renderer: WebGLRenderer; post: ScenePost | null; start: number }) {
    const { renderer, post, start } = options;
    this.#min = post?.exposureMin ?? 0.25;
    this.#max = post?.exposureMax ?? 8;
    this.#bias = 2 ** (post?.tonemap.exposureBias ?? 0);
    this.#goal = start;
    this.#value = start;
    // A float target is what lets the meter see luminance over 1; without it the exposure stays fixed.
    this.#target = renderer.extensions.has('EXT_color_buffer_float')
      ? new WebGLRenderTarget(METER_WIDTH, METER_HEIGHT, { type: FloatType, format: RGBAFormat, minFilter: NearestFilter, magFilter: NearestFilter })
      : null;
  }

  get value(): number {
    return this.#value;
  }

  /**
   * Meters the frame when due and eases the exposure towards its goal; answers
   * true while it is still moving, so the caller draws another frame.
   */
  step(options: { renderer: WebGLRenderer; scene: Object3D; camera: Camera; meter: { value: number }; now: number }): boolean {
    const { renderer, scene, camera, meter, now } = options;
    if (this.#target !== null && (now - this.#metered >= METER_EVERY || now < this.#metered)) {
      this.#metered = now;
      const average = this.#measure(renderer, scene, camera, meter);
      if (average !== null) this.#goal = this.#clamp(MIDDLE_GREY / average) * this.#bias;
    }
    const dt = this.#adapted === 0 ? 0 : Math.min(0.25, Math.max(0, now - this.#adapted) / 1000);
    this.#adapted = now;
    this.#value += (this.#goal - this.#value) * (1 - Math.exp(-dt / ADAPT_SECONDS));
    return Math.abs(this.#goal - this.#value) > this.#goal * 0.01;
  }

  dispose(): void {
    this.#target?.dispose();
  }

  /** The volume's range; an inverted one, as some maps ship, keeps its minimum below it. */
  #clamp(exposure: number): number {
    return exposure < this.#min ? this.#min : Math.min(this.#max, exposure);
  }

  #measure(renderer: WebGLRenderer, scene: Object3D, camera: Camera, meter: { value: number }): number | null {
    const target = this.#target;
    if (target === null) return null;
    const previous = renderer.getRenderTarget();
    meter.value = 1;
    renderer.setRenderTarget(target);
    renderer.clear();
    renderer.render(scene, camera);
    renderer.readRenderTargetPixels(target, 0, 0, METER_WIDTH, METER_HEIGHT, this.#pixels);
    renderer.setRenderTarget(previous);
    meter.value = 0;
    let sum = 0;
    let count = 0;
    for (let i = 0; i < this.#pixels.length; i += 4) {
      const luminance = this.#pixels[i];
      if (!(luminance > DARKEST)) continue;
      sum += Math.log(luminance);
      count += 1;
    }
    return count === 0 ? null : Math.exp(sum / count);
  }
}
