import { Vector3 } from 'three';
import type { Data3DTexture, DataTexture } from 'three';
import type { MapScene, SceneLight } from '@/lib/api/map-scene';
import { CELL_ITEMS, boxGrid, rowTable } from './scene-grid';

/**
 * Texels per light: eye and falloff, colour and shadow channel, forward and near,
 * right and far, up and whether it is an omni, the frustum, the skirt and distance fades, the angle fade.
 */
const LIGHT_TEXELS = 8;

/**
 * CS2's stationary barn and omni lights, after ValveResourceFormat's lighting_barn
 * shader: a barn light reaches what its frustum holds, faded in from its near plane
 * and out toward its far one; an omni fades over its range and its cone. Both fall
 * off with the squared distance and are dimmed by their baked shadow channel.
 * Needs cs2SunSpecular from CS2_REFLECT.
 */
export const CS2_LIGHTS = `
uniform sampler2D uLights;
uniform sampler3D uLightGrid;
uniform vec3 uLightGridMin;
uniform vec3 uLightGridScale;
uniform float uUseLights;

vec4 cs2Light(int light, int texel) {
  return texelFetch(uLights, ivec2(texel, light), 0);
}

float cs2LightFade(float x) {
  return smoothstep(0.0, 1.0, x);
}

// Adds the lights' diffuse (before albedo) and specular at world; baked holds the lightmap's shadow channels.
void cs2Lights(inout vec3 diffuse, inout vec3 specular, vec3 world, vec3 N, vec3 V, float roughness, vec3 f0, vec4 baked) {
  if (uUseLights < 0.5) return;
  vec4 ids = texture(uLightGrid, (world - uLightGridMin) * uLightGridScale) * 255.0;
  for (int k = 0; k < ${CELL_ITEMS}; k++) {
    int id = int(ids[k] + 0.5) - 1;
    if (id < 0) break;
    vec4 colorShadow = cs2Light(id, 1);
    float visibility = 1.0;
    if (colorShadow.w > -0.5) visibility = 1.0 - clamp(baked[int(colorShadow.w + 0.5) & 3], 0.0, 1.0);
    if (visibility <= 0.0) continue;
    vec4 eye = cs2Light(id, 0);
    vec4 forward = cs2Light(id, 2);
    vec4 right = cs2Light(id, 3);
    vec4 up = cs2Light(id, 4);
    vec4 fades = cs2Light(id, 6);
    vec3 toLight = eye.xyz - world;
    float attenuation = 1.0;
    if (up.w < 0.5) {
      vec3 local = vec3(dot(-toLight, right.xyz), dot(-toLight, up.xyz), dot(-toLight, forward.xyz));
      vec4 frustum = cs2Light(id, 5);
      vec3 ndc;
      if (forward.w > 0.0) {
        if (local.z <= 0.0) continue;
        ndc = vec3((forward.w * local.xy / local.z - frustum.zw) / frustum.xy, right.w * (local.z - forward.w) / ((right.w - forward.w) * local.z));
      } else {
        ndc = vec3(local.xy / frustum.xy, local.z / right.w);
      }
      if (ndc.z <= 0.0 || ndc.z >= 1.0 || abs(ndc.x) >= 1.0 || abs(ndc.y) >= 1.0) continue;
      if (fades.x > 0.0) attenuation *= cs2LightFade(ndc.z * fades.x);
      if (fades.y > 0.0) attenuation *= cs2LightFade((1.0 - ndc.z) * fades.y);
    }
    vec3 L = -forward.xyz;
    if (eye.w > 0.0) {
      float distSq = max(dot(toLight, toLight), 1e-4);
      float dist = sqrt(distSq);
      L = toLight / dist;
      vec2 angle = cs2Light(id, 7).xy;
      attenuation *= eye.w / max(distSq, eye.w);
      attenuation *= cs2LightFade(fades.z + fades.w * dist);
      attenuation *= clamp(angle.x + angle.y * dot(forward.xyz, L), 0.0, 1.0);
    }
    vec3 color = colorShadow.rgb * visibility * attenuation;
    if (dot(color, color) <= 0.0) continue;
    diffuse += color * max(dot(N, L), 0.0);
    specular += color * cs2SunSpecular(N, V, L, roughness, f0);
  }
}
`;

export type Cs2LightsUniforms = {
  uLights: { value: DataTexture | null };
  uLightGrid: { value: Data3DTexture | null };
  uLightGridMin: { value: Vector3 };
  uLightGridScale: { value: Vector3 };
  uUseLights: { value: number };
};

function lightTable(lights: SceneLight[]): DataTexture {
  return rowTable(lights, LIGHT_TEXELS, (data, row, l) => {
    data.set([...l.eye, l.falloff], row);
    data.set([...l.color, l.shadow], row + 4);
    data.set([...l.forward, l.near], row + 8);
    data.set([...l.right, l.far], row + 12);
    data.set([...l.up, l.omni ? 1 : 0], row + 16);
    data.set(l.frustum, row + 20);
    data.set([...l.skirt, ...l.fade], row + 24);
    data.set([...l.angle, 0, 0], row + 28);
  });
}

/** The light uniforms of `scene`; brighter lights take a grid cell first when more than it holds reach it. */
export function cs2LightsUniforms(scene: MapScene): Cs2LightsUniforms {
  const lights = [...scene.lights].sort((a, b) => Math.max(...b.color) - Math.max(...a.color)).slice(0, 255);
  if (lights.length === 0) {
    return { uLights: { value: null }, uLightGrid: { value: null }, uLightGridMin: { value: new Vector3() }, uLightGridScale: { value: new Vector3() }, uUseLights: { value: 0 } };
  }
  const grid = boxGrid(scene, lights.map((l) => ({ lo: l.min, hi: l.max })));
  return { uLights: { value: lightTable(lights) }, uLightGrid: { value: grid.texture }, uLightGridMin: { value: grid.min }, uLightGridScale: { value: grid.scale }, uUseLights: { value: 1 } };
}

export function disposeLights(uniforms: Cs2LightsUniforms): void {
  uniforms.uLights.value?.dispose();
  uniforms.uLightGrid.value?.dispose();
}
