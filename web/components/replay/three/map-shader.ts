import { DataTexture, DataUtils, HalfFloatType, LinearFilter, RGBAFormat, Texture, Vector4 } from 'three';
import type { Material, WebGLProgramParametersWithUniforms } from 'three';
import type { FloorField } from '@/lib/replay/three/floor-field';

/**
 * The cutaway of the 3D map, done per pixel: every wall stops `wallCut` over
 * the floor under it, every flat face more than `slabCut` over that floor (a
 * roof, a ceiling, an awning) goes, and nothing shows above `ceiling`. The same
 * floor field darkens wall feet and the floor beside walls, and the radar
 * image can be laid on the floors from above.
 */

export type MapUniforms = {
  uField: { value: Texture };
  uFieldRect: { value: Vector4 };
  uUseField: { value: number };
  /** 1 cuts walls and roofs over the floor; 0 keeps the whole map, as the first-person camera wants. */
  uCutaway: { value: number };
  uWallCut: { value: number };
  uSlabCut: { value: number };
  uCeiling: { value: number };
  uRadar: { value: Texture };
  uRadarRect: { value: Vector4 };
  uRadarMix: { value: number };
};

export function blankTexture(): DataTexture {
  const texture = new DataTexture(new Uint8Array([0, 0, 0, 0]), 1, 1, RGBAFormat);
  texture.needsUpdate = true;
  return texture;
}

/** Floor height in R, wall nearness in G and how far outside the map in B, as a filtered half-float texture. */
export function fieldTexture(field: FloorField): DataTexture {
  const data = new Uint16Array(field.floor.length * 4);
  for (let i = 0; i < field.floor.length; i += 1) {
    data[i * 4] = DataUtils.toHalfFloat(field.floor[i]);
    data[i * 4 + 1] = DataUtils.toHalfFloat(field.walls[i]);
    data[i * 4 + 2] = DataUtils.toHalfFloat(field.outside[i]);
    data[i * 4 + 3] = DataUtils.toHalfFloat(1);
  }
  const texture = new DataTexture(data, field.cols, field.rows, RGBAFormat, HalfFloatType);
  texture.minFilter = LinearFilter;
  texture.magFilter = LinearFilter;
  texture.needsUpdate = true;
  return texture;
}

export function mapUniforms(field: FloorField | null, cuts: { wall: number; slab: number }): MapUniforms {
  return {
    uField: { value: field === null ? blankTexture() : fieldTexture(field) },
    uFieldRect: {
      value: field === null ? new Vector4(0, 0, 1, 1) : new Vector4(field.minX, field.minY, 1 / (field.cols * field.cell), 1 / (field.rows * field.cell)),
    },
    uUseField: { value: field === null ? 0 : 1 },
    uCutaway: { value: 1 },
    uWallCut: { value: cuts.wall },
    uSlabCut: { value: cuts.slab },
    uCeiling: { value: 1e9 },
    uRadar: { value: blankTexture() },
    uRadarRect: { value: new Vector4(0, 0, 1, 1) },
    uRadarMix: { value: 0 },
  };
}

const DECLARE = `
uniform sampler2D uField;
uniform vec4 uFieldRect;
uniform float uUseField;
uniform float uCutaway;
uniform float uWallCut;
uniform float uSlabCut;
uniform float uCeiling;
uniform sampler2D uRadar;
uniform vec4 uRadarRect;
uniform float uRadarMix;
varying vec3 vMapPos;
`;

const SURFACE_CUT = `
vec3 mapFacet = normalize(cross(dFdx(vMapPos), dFdy(vMapPos)));
float mapFlat = step(0.7, abs(mapFacet.z));
vec3 mapCell = texture2D(uField, (vMapPos.xy - uFieldRect.xy) * uFieldRect.zw).rgb;
float mapRise = vMapPos.z - mapCell.r;
float mapCutting = uUseField * uCutaway;
if (vMapPos.z > uCeiling) discard;
if (mapCutting > 0.5 && (mapRise > uWallCut || (mapFlat > 0.5 && mapRise > uSlabCut))) discard;
`;

const SURFACE_TONE = `
if (uRadarMix > 0.0 && mapFlat > 0.5) {
  vec2 mapUv = vec2((vMapPos.x - uRadarRect.x) / uRadarRect.z, 1.0 - (uRadarRect.y - vMapPos.y) / uRadarRect.w);
  vec4 radar = texture2D(uRadar, mapUv);
  float inside = step(0.0, mapUv.x) * step(mapUv.x, 1.0) * step(0.0, mapUv.y) * step(mapUv.y, 1.0);
  diffuseColor.rgb = mix(diffuseColor.rgb, radar.rgb * 1.45, radar.a * inside * uRadarMix);
}
if (uUseField > 0.5) {
  diffuseColor.rgb *= mapFlat > 0.5 ? 1.0 - 0.5 * mapCell.g : mix(0.45, 1.0, smoothstep(-8.0, 128.0, mapRise));
  // A pale rim where a wall is cut, so the cutaway reads as a clean section.
  float mapRim = max(4.0, 1.5 * fwidth(mapRise));
  if (mapCutting > 0.5 && mapFlat < 0.5 && mapRise > uWallCut - mapRim) diffuseColor.rgb = vec3(0.62, 0.65, 0.7);
  diffuseColor.rgb *= 1.0 - 0.65 * mapCell.b;
}
`;

const LINE_CUT = `
vec3 mapCell = texture2D(uField, (vMapPos.xy - uFieldRect.xy) * uFieldRect.zw).rgb;
float mapRise = vMapPos.z - mapCell.r;
if (vMapPos.z > uCeiling) discard;
if (uUseField * uCutaway > 0.5 && mapRise > mix(uSlabCut, uWallCut, vVertical) + 2.0) discard;
if (mapCell.b > 0.9) discard;
`;

function patchVertex(shader: WebGLProgramParametersWithUniforms, lines: boolean): void {
  shader.vertexShader = shader.vertexShader
    .replace('#include <common>', `#include <common>\nvarying vec3 vMapPos;${lines ? '\nattribute float aVertical;\nvarying float vVertical;' : ''}`)
    .replace('#include <begin_vertex>', `#include <begin_vertex>\nvMapPos = (modelMatrix * vec4(transformed, 1.0)).xyz;${lines ? '\nvVertical = aVertical;' : ''}`);
}

/** Patches a lit surface material of the map with the cutaway, the shading and the radar. */
export function patchSurface(material: Material, uniforms: MapUniforms): void {
  material.onBeforeCompile = (shader) => {
    Object.assign(shader.uniforms, uniforms);
    patchVertex(shader, false);
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\n${DECLARE}`)
      .replace('#include <clipping_planes_fragment>', `#include <clipping_planes_fragment>\n${SURFACE_CUT}`)
      .replace('#include <color_fragment>', `#include <color_fragment>\n${SURFACE_TONE}`);
  };
  material.customProgramCacheKey = () => 'map-surface';
}

/** Patches the edge lines' material with the same cutaway; `aVertical` picks the wall or the slab cut. */
export function patchLines(material: Material, uniforms: MapUniforms): void {
  material.onBeforeCompile = (shader) => {
    Object.assign(shader.uniforms, uniforms);
    patchVertex(shader, true);
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\n${DECLARE}\nvarying float vVertical;`)
      .replace('#include <clipping_planes_fragment>', `#include <clipping_planes_fragment>\n${LINE_CUT}`);
  };
  material.customProgramCacheKey = () => 'map-lines';
}

export function disposeUniforms(uniforms: MapUniforms): void {
  uniforms.uField.value.dispose();
  uniforms.uRadar.value.dispose();
}
