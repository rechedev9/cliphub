/**
 * The ambient depth field behind the Studio shell: a slow volumetric fog with a
 * perspective floor receding to a horizon, drawn by hand in WebGL1.
 *
 * This module is the pure half — buffer sizing, the degradation decision, the
 * noise texture and the GL wiring. The React lifecycle lives in
 * `components/shell/studio-ambient.tsx`; keeping the two apart is what lets
 * `lib/studio-ambient.test.ts` assert the rules that actually matter (half
 * resolution, DPR clamped to 1, one static frame under every degradation
 * signal) on `node:test`, without a GPU.
 *
 * Budget, from audit-motion §4.4: ~1 MB VRAM, ~0.4 ms GPU per frame at half
 * resolution and 30 fps, one `drawArrays` and two uniform writes of CPU work.
 */

/** Half resolution. The content is low-frequency fog; the CSS upscale hides it. */
export const AMBIENT_RENDER_SCALE = 0.5;
/** The drift is ~0.01 Hz. 30 fps is indistinguishable from 60 and costs half. */
export const AMBIENT_FRAME_MS = 1000 / 30;
/** Side of the square CPU-generated noise texture, in texels. */
const NOISE_SIZE = 256;
/** A 4K monitor at 150% scaling would otherwise quadruple the pixel count. */
const MAX_PIXEL_RATIO = 1;

/**
 * The four signals that decide whether the field may animate. They are read
 * from `<html>` attributes and `matchMedia` by the component; this module only
 * knows what they mean.
 */
export interface AmbientSignals {
  readonly reducedMotion: boolean;
  readonly efficiency: boolean;
  readonly captureActive: boolean;
  readonly windowActive: boolean;
}

/**
 * `animated` runs the capped rAF loop; `static` draws exactly one frame at
 * t = 0 and never schedules another. Depth is kept in both modes — only motion
 * is degraded — because a flat backdrop is the problem this field solves.
 */
export type AmbientMode = 'animated' | 'static';

export interface AmbientBufferSize {
  readonly width: number;
  readonly height: number;
}

export interface AmbientScene {
  /**
   * Draws one frame and reports whether it reached the GPU. `timeSeconds` is 0
   * in static mode. Returns false while the context is released or lost, which
   * is what stops the caller from cross-fading an empty canvas over the CSS
   * fallback before any frame exists.
   */
  draw(timeSeconds: number, mode: AmbientMode): boolean;
  /** Resizes the drawing buffer to the current CSS box. Returns false if unchanged. */
  resize(cssWidth: number, cssHeight: number, pixelRatio: number): boolean;
  /**
   * Hands the GL context back to the driver. Studio sits open for hours next to
   * a capture, so a backdrop nobody is looking at should not be holding VRAM.
   * Reversible: `restore()` brings the same canvas back.
   */
  release(): void;
  restore(): void;
  /** Permanent teardown: unhooks the loss listeners and drops the context. */
  dispose(): void;
}

export function ambientMode(signals: AmbientSignals): AmbientMode {
  if (signals.reducedMotion || signals.efficiency || signals.captureActive) return 'static';
  return signals.windowActive ? 'animated' : 'static';
}

/**
 * The drawing-buffer size for a CSS box: half resolution, device pixel ratio
 * clamped to 1, never zero (a 0-sized buffer makes `gl.viewport` a no-op and
 * the canvas keeps the previous frame stretched).
 */
export function ambientBufferSize(
  cssWidth: number,
  cssHeight: number,
  pixelRatio: number,
): AmbientBufferSize {
  const ratio = Math.min(Math.max(pixelRatio, 0.5), MAX_PIXEL_RATIO);
  const scale = ratio * AMBIENT_RENDER_SCALE;
  return {
    width: Math.max(1, Math.round(cssWidth * scale)),
    height: Math.max(1, Math.round(cssHeight * scale)),
  };
}

/**
 * A tileable value-noise field, one byte per texel, sampled as LUMINANCE by the
 * fragment shader's 3-octave fbm. Deterministic on purpose: the fog has to look
 * the same on every launch, and 32 `sin()` per pixel is the alternative.
 */
export function ambientNoise(size: number): Uint8Array {
  const lattice = 32;
  const grid = new Float32Array(lattice * lattice);
  for (let i = 0; i < grid.length; i += 1) {
    grid[i] = hash01(i);
  }

  const texels = new Uint8Array(size * size);
  for (let y = 0; y < size; y += 1) {
    for (let x = 0; x < size; x += 1) {
      // Wrap the lattice lookup so the texture tiles seamlessly; a visible seam
      // in a full-viewport backdrop is worse than no backdrop.
      const fx = (x / size) * lattice;
      const fy = (y / size) * lattice;
      const x0 = Math.floor(fx);
      const y0 = Math.floor(fy);
      const tx = smoothstep(fx - x0);
      const ty = smoothstep(fy - y0);
      const top = mix(latticeAt(grid, lattice, x0, y0), latticeAt(grid, lattice, x0 + 1, y0), tx);
      const bottom = mix(
        latticeAt(grid, lattice, x0, y0 + 1),
        latticeAt(grid, lattice, x0 + 1, y0 + 1),
        tx,
      );
      texels[y * size + x] = Math.round(mix(top, bottom, ty) * 255);
    }
  }
  return texels;
}

function latticeAt(grid: Float32Array, lattice: number, x: number, y: number): number {
  return grid[(y % lattice) * lattice + (x % lattice)] ?? 0;
}

function mix(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

function smoothstep(t: number): number {
  return t * t * (3 - 2 * t);
}

/** A cheap integer hash in [0,1). No Math.random: the field must be stable. */
function hash01(n: number): number {
  let h = Math.imul(n ^ 0x27d4eb2d, 0x165667b1);
  h ^= h >>> 15;
  h = Math.imul(h, 0x2545f491);
  h ^= h >>> 13;
  return (h >>> 0) / 0x100000000;
}

const VERTEX_SOURCE = `
attribute vec2 aPos;
void main() { gl_Position = vec4(aPos, 0.0, 1.0); }
`;

/**
 * Fog only — no floor grid.
 *
 * The reference shader carried a perspective grid receding to a horizon, but
 * `.shell-canvas::after` already draws exactly that in CSS, and CSS has to keep
 * drawing it (it is what survives no-WebGL, forced-colors and a lost context).
 * Running both stacked two floors on top of each other at slightly different
 * vanishing points — measured in the browser, and it read as a moiré, not as
 * depth. So the division is: CSS owns the room's geometry, WebGL owns its air.
 *
 * Total added luminance is bounded by 0.050 + 0.026 before the smoothstep
 * falloffs and the vignette, and the vignette is subtractive in the centre
 * where the panels sit, so realistic peak added Y is ~0.02 — under a 2% change
 * to any text contrast ratio on the canvas.
 */
const FRAGMENT_SOURCE = `
precision mediump float;

uniform vec2 uResolution;
uniform float uTime;
uniform float uEnergy;
uniform sampler2D uNoise;

float fbm(vec2 p) {
  float a = 0.5;
  float s = 0.0;
  for (int i = 0; i < 3; i++) {
    s += a * texture2D(uNoise, p * 0.15).r;
    p *= 2.03;
    a *= 0.5;
  }
  return s;
}

void main() {
  vec2 p = (gl_FragCoord.xy - 0.5 * uResolution) / uResolution.y;

  float f1 = fbm(p * 1.6 + vec2(uTime * 0.010 * uEnergy, 0.0));
  float f2 = fbm(p * 2.4 - vec2(0.0, uTime * 0.008 * uEnergy) + 7.3);

  vec3 canvasCol = vec3(0.028, 0.036, 0.062);
  vec3 cyan = vec3(0.129, 0.851, 0.933);
  vec3 violet = vec3(0.549, 0.353, 1.000);

  vec3 col = canvasCol;
  col += cyan * f1 * 0.050 * smoothstep(1.10, -0.20, length(p));
  col += violet * f2 * 0.026 * smoothstep(0.95, 0.00, length(p + vec2(0.6, 0.3)));
  col *= 1.0 - 0.32 * smoothstep(0.35, 1.25, length(p));

  gl_FragColor = vec4(col, 1.0);
}
`;

/**
 * Builds the scene, or returns null when WebGL is unavailable — in which case
 * the caller removes the canvas and the `.ff-ambient-fallback` gradients stay
 * as the permanent backdrop.
 */
interface AmbientResources {
  readonly program: WebGLProgram;
  readonly buffer: WebGLBuffer;
  readonly texture: WebGLTexture;
  readonly resolution: WebGLUniformLocation | null;
  readonly time: WebGLUniformLocation | null;
  readonly energy: WebGLUniformLocation | null;
}

export function createAmbientScene(canvas: HTMLCanvasElement): AmbientScene | null {
  const gl = canvas.getContext('webgl', {
    alpha: false,
    depth: false,
    stencil: false,
    antialias: false,
    preserveDrawingBuffer: false,
    powerPreference: 'low-power',
    desynchronized: true,
  });
  if (gl === null) return null;

  let resources = uploadResources(gl);
  if (resources === null) return null;

  // A released or driver-lost context invalidates every GL object. Swallowing
  // the default action is what makes the loss recoverable at all; without it
  // Chromium never fires `webglcontextrestored`.
  const onLost = (event: Event): void => {
    event.preventDefault();
    resources = null;
    canvas.removeAttribute('data-ready');
  };
  const onRestored = (): void => {
    resources = uploadResources(gl);
  };
  canvas.addEventListener('webglcontextlost', onLost);
  canvas.addEventListener('webglcontextrestored', onRestored);

  const lose = gl.getExtension('WEBGL_lose_context');

  return {
    resize(cssWidth: number, cssHeight: number, pixelRatio: number): boolean {
      const size = ambientBufferSize(cssWidth, cssHeight, pixelRatio);
      if (canvas.width === size.width && canvas.height === size.height) return false;
      canvas.width = size.width;
      canvas.height = size.height;
      if (resources === null) return true;
      gl.viewport(0, 0, size.width, size.height);
      gl.uniform2f(resources.resolution, size.width, size.height);
      return true;
    },
    draw(timeSeconds: number, mode: AmbientMode): boolean {
      if (resources === null) return false;
      gl.uniform1f(resources.time, mode === 'animated' ? timeSeconds : 0);
      gl.uniform1f(resources.energy, mode === 'animated' ? 1 : 0);
      gl.drawArrays(gl.TRIANGLES, 0, 3);
      return true;
    },
    release(): void {
      if (lose !== null && !gl.isContextLost()) lose.loseContext();
    },
    restore(): void {
      if (lose !== null && gl.isContextLost()) lose.restoreContext();
    },
    dispose(): void {
      canvas.removeEventListener('webglcontextlost', onLost);
      canvas.removeEventListener('webglcontextrestored', onRestored);
      if (resources !== null) {
        gl.deleteTexture(resources.texture);
        gl.deleteBuffer(resources.buffer);
        gl.deleteProgram(resources.program);
        resources = null;
      }
      if (lose !== null && !gl.isContextLost()) lose.loseContext();
    },
  };
}

/** Builds every GL object the shader needs. Re-run verbatim after a restore. */
function uploadResources(gl: WebGLRenderingContext): AmbientResources | null {
  const program = buildProgram(gl);
  if (program === null) return null;

  const buffer = gl.createBuffer();
  const texture = gl.createTexture();
  if (buffer === null || texture === null) {
    gl.deleteProgram(program);
    return null;
  }

  // One full-screen triangle rather than two: no shared edge to rasterise twice
  // and one vertex fewer to transform.
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
  const position = gl.getAttribLocation(program, 'aPos');
  gl.enableVertexAttribArray(position);
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);

  gl.activeTexture(gl.TEXTURE0);
  gl.bindTexture(gl.TEXTURE_2D, texture);
  gl.texImage2D(
    gl.TEXTURE_2D,
    0,
    gl.LUMINANCE,
    NOISE_SIZE,
    NOISE_SIZE,
    0,
    gl.LUMINANCE,
    gl.UNSIGNED_BYTE,
    ambientNoise(NOISE_SIZE),
  );
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);

  gl.useProgram(program);
  gl.uniform1i(gl.getUniformLocation(program, 'uNoise'), 0);
  gl.viewport(0, 0, gl.drawingBufferWidth, gl.drawingBufferHeight);

  const resolution = gl.getUniformLocation(program, 'uResolution');
  gl.uniform2f(resolution, gl.drawingBufferWidth, gl.drawingBufferHeight);

  return {
    program,
    buffer,
    texture,
    resolution,
    time: gl.getUniformLocation(program, 'uTime'),
    energy: gl.getUniformLocation(program, 'uEnergy'),
  };
}

function buildProgram(gl: WebGLRenderingContext): WebGLProgram | null {
  const vertex = buildShader(gl, gl.VERTEX_SHADER, VERTEX_SOURCE);
  const fragment = buildShader(gl, gl.FRAGMENT_SHADER, FRAGMENT_SOURCE);
  if (vertex === null || fragment === null) return null;

  const program = gl.createProgram();
  if (program === null) return null;
  gl.attachShader(program, vertex);
  gl.attachShader(program, fragment);
  gl.linkProgram(program);
  // The shaders are linked into the program; the objects themselves are dead
  // weight from here on.
  gl.deleteShader(vertex);
  gl.deleteShader(fragment);
  if (gl.getProgramParameter(program, gl.LINK_STATUS) !== true) {
    gl.deleteProgram(program);
    return null;
  }
  return program;
}

function buildShader(gl: WebGLRenderingContext, type: number, source: string): WebGLShader | null {
  const shader = gl.createShader(type);
  if (shader === null) return null;
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (gl.getShaderParameter(shader, gl.COMPILE_STATUS) !== true) {
    gl.deleteShader(shader);
    return null;
  }
  return shader;
}
