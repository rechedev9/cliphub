import { Data3DTexture, LinearFilter, RGBAFormat, UnsignedByteType, Vector2, Vector4 } from 'three';
import type { MapScene, ScenePost, SceneTonemap } from '@/lib/api/map-scene';

/**
 * CS2's post processing, as ValveResourceFormat reproduces it: the exposed
 * colour times 2.8, clamped to the white point, through the map's Uncharted
 * curve, then sRGB encoded and graded by the map's colour LUT. The shaders
 * apply `cs2Tone` before fog and `cs2Grade` last; three encodes the result.
 */
export const CS2_POST = `
uniform vec4 uToneA;
uniform vec4 uToneB;
uniform sampler3D uLut;
uniform float uUseLut;
uniform vec2 uLutRange;

vec3 cs2Tone(vec3 x) {
  x = min(x * 2.8, vec3(uToneB.z));
  vec3 num = x * (uToneA.x * x + uToneA.y * uToneA.z) + uToneB.x * uToneA.w;
  vec3 den = x * (uToneA.x * x + uToneA.y) + uToneB.y * uToneA.w;
  return clamp((num / den - uToneB.x / uToneB.y) * uToneB.w, 0.0, 1.0);
}

vec3 cs2Encode(vec3 c) {
  return mix(c * 12.92, 1.055 * pow(c, vec3(1.0 / 2.4)) - 0.055, step(vec3(0.0031308), c));
}

vec3 cs2Decode(vec3 c) {
  return mix(c / 12.92, pow((c + 0.055) / 1.055, vec3(2.4)), step(vec3(0.04045), c));
}

vec3 cs2Grade(vec3 c) {
  if (uUseLut < 0.5) return c;
  vec3 graded = texture(uLut, cs2Encode(clamp(c, 0.0, 1.0)) * uLutRange.x + uLutRange.y).rgb;
  return cs2Decode(graded);
}
`;

/** The engine's curve where a map has no post processing volume. */
const DEFAULT_TONEMAP: SceneTonemap = { exposureBias: 0, shoulder: 0.15, linearStrength: 0.5, linearAngle: 0.1, toeStrength: 0.2, toeNum: 0.02, toeDenom: 0.3, whitePoint: 4 };

/** The grey auto exposure brings the frame's average luminance to. */
const MIDDLE_GREY = 0.18;

/** The albedo luminance of a typical map surface, for the average luminance estimate. */
const AVERAGE_ALBEDO = 0.35;

function curve(t: SceneTonemap, x: number): number {
  const num = x * (t.shoulder * x + t.linearStrength * t.linearAngle) + t.toeNum * t.toeStrength;
  const den = x * (t.shoulder * x + t.linearStrength) + t.toeDenom * t.toeStrength;
  return num / den - t.toeNum / t.toeDenom;
}

function luminance(c: readonly number[]): number {
  return 0.2126 * (c[0] ?? 0) + 0.7152 * (c[1] ?? 0) + 0.0722 * (c[2] ?? 0);
}

/**
 * The exposure CS2's auto exposure settles on, from an estimate of the frame's
 * average light, held to the volume's range; an inverted range keeps its minimum.
 */
export function cs2Exposure(scene: MapScene, ambient: readonly number[]): number {
  const post = scene.post;
  const min = post?.exposureMin ?? 0.25;
  const max = post?.exposureMax ?? 8;
  const light = AVERAGE_ALBEDO * (luminance(ambient) + 0.5 * luminance(scene.sun?.color ?? [0, 0, 0]));
  const raw = MIDDLE_GREY / Math.max(1e-3, light);
  const exposure = raw < min ? min : Math.min(max, raw);
  return exposure * 2 ** (post?.tonemap.exposureBias ?? 0);
}

/** The uniforms the shaders that include CS2_POST share. */
export type Cs2PostUniforms = {
  uToneA: { value: Vector4 };
  uToneB: { value: Vector4 };
  uLut: { value: Data3DTexture };
  uUseLut: { value: number };
  uLutRange: { value: Vector2 };
};

export class Cs2Post {
  readonly uniforms: Cs2PostUniforms;
  readonly #lut: Data3DTexture | null;
  #blank: Data3DTexture | null = null;

  constructor(post: ScenePost | null) {
    const t = post?.tonemap ?? DEFAULT_TONEMAP;
    const white = 2 ** t.whitePoint;
    this.#lut = post?.lut === null || post?.lut === undefined ? null : lutTexture(post.lut, post.lutSize);
    const size = post?.lutSize ?? 32;
    this.uniforms = {
      uToneA: { value: new Vector4(t.shoulder, t.linearStrength, t.linearAngle, t.toeStrength) },
      uToneB: { value: new Vector4(t.toeNum, t.toeDenom, white, 1 / curve(t, white)) },
      uLut: { value: this.#lut ?? (this.#blank = blankLut()) },
      uUseLut: { value: this.#lut === null ? 0 : 1 },
      uLutRange: { value: new Vector2(1 - 1 / size, 0.5 / size) },
    };
  }

  dispose(): void {
    this.#lut?.dispose();
    this.#blank?.dispose();
  }
}

/** A 1-texel LUT for shaders whose map has none; uUseLut keeps it unread. */
function blankLut(): Data3DTexture {
  return lutTexture(new Uint8Array([0, 0, 0, 255]), 1);
}

function lutTexture(data: Uint8Array, size: number): Data3DTexture {
  const texture = new Data3DTexture(data, size, size, size);
  texture.format = RGBAFormat;
  texture.type = UnsignedByteType;
  texture.minFilter = LinearFilter;
  texture.magFilter = LinearFilter;
  texture.unpackAlignment = 1;
  texture.needsUpdate = true;
  return texture;
}
