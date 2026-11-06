import { Matrix3, Vector2, Vector4 } from 'three';
import type { CompressedCubeTexture } from 'three';
import type { MapScene, SceneFogRamp } from '@/lib/api/map-scene';

/**
 * CS2's fog, as ValveResourceFormat's fog shader reproduces it: a gradient fog
 * of one colour, then a cubemap fog coloured by the sky, blurred where the fog
 * is thin. Each is the distance term times the height term, both clamped and
 * raised to their exponents, times the fog's opacity. It runs on the linear
 * colour before exposure; `lift` pushes it back for a camera high over the map.
 */
export const CS2_FOG = `
uniform vec4 uGradFog;
uniform vec2 uGradExp;
uniform vec4 uGradColor;
uniform vec4 uCubeFog;
uniform vec4 uCubeFogHeight;
uniform vec2 uCubeFogColor;
uniform samplerCube uFogSky;
uniform mat3 uFogSkyBasis;
uniform float uFogStrength;

vec3 cs2Fog(vec3 color, vec3 world, float lift) {
  vec3 rel = world - cameraPosition;
  float d = max(0.0, length(rel) - lift);
  if (uGradColor.a > 0.0) {
    vec2 t = pow(clamp(vec2(d, world.z) * uGradFog.zw + uGradFog.xy, 0.0001, 1.0), uGradExp);
    color = mix(color, uGradColor.rgb, t.x * t.y * uGradColor.a * uFogStrength);
  }
  if (uCubeFogColor.x > 0.0) {
    float near = pow(max(d * uCubeFog.y + uCubeFog.x, 0.0001), uCubeFog.w);
    float high = pow(max(world.z * uCubeFogHeight.y + uCubeFogHeight.x, 0.0001), uCubeFogHeight.z);
    float blend = clamp(near, 0.0, 1.0) * clamp(high, 0.0, 1.0);
    float lod = clamp(1.0 - blend * uCubeFog.z, 0.0, 1.0) * uCubeFogHeight.w;
    vec3 sky = textureLod(uFogSky, uFogSkyBasis * normalize(rel), lod).rgb * uCubeFogColor.y;
    color = mix(color, sky, blend * uCubeFogColor.x * uFogStrength);
  }
  return color;
}
`;

export type Cs2FogUniforms = {
  uGradFog: { value: Vector4 };
  uGradExp: { value: Vector2 };
  uGradColor: { value: Vector4 };
  uCubeFog: { value: Vector4 };
  uCubeFogHeight: { value: Vector4 };
  uCubeFogColor: { value: Vector2 };
  uFogSky: { value: CompressedCubeTexture | null };
  uFogSkyBasis: { value: Matrix3 };
  uFogStrength: { value: number };
};

/** Scale and bias of a distance ramp: 0 at its start, 1 at its end. */
function distanceRamp(ramp: SceneFogRamp): [number, number] {
  const scale = 1 / Math.max(1e-3, ramp.end - ramp.start);
  return [scale, -ramp.start * scale];
}

/** Scale and bias of a height ramp: 1 at its start, 0 at its end; no range keeps every height in. */
function heightRamp(ramp: SceneFogRamp): [number, number] {
  if (ramp.start === ramp.end) return [0, 1];
  const scale = 1 / (ramp.start - ramp.end);
  return [scale, -ramp.end * scale];
}

/** The fog uniforms of `scene`; the cubemap fog takes the sky cube and its turn, and stays off without one. */
export function cs2FogUniforms(options: { scene: MapScene; sky: { cube: CompressedCubeTexture; basis: Matrix3; size: number } | null }): Cs2FogUniforms {
  const { scene, sky } = options;
  const gradient = scene.fog?.gradient ?? null;
  const cube = sky === null ? null : (scene.fog?.cube ?? null);
  const [gScale, gBias] = gradient === null ? [0, 0] : distanceRamp(gradient.distance);
  const [ghScale, ghBias] = gradient === null ? [0, 1] : heightRamp(gradient.height);
  const [cScale, cBias] = cube === null ? [0, 0] : distanceRamp(cube.distance);
  const [chScale, chBias] = cube === null ? [0, 1] : heightRamp(cube.height);
  const color = gradient?.color ?? [0, 0, 0];
  return {
    uGradFog: { value: new Vector4(gBias, ghBias, gScale, ghScale) },
    uGradExp: { value: new Vector2(gradient?.distance.exponent ?? 1, ghScale === 0 ? 0 : (gradient?.height.exponent ?? 1)) },
    uGradColor: { value: new Vector4(color[0], color[1], color[2], gradient?.max ?? 0) },
    uCubeFog: { value: new Vector4(cBias, cScale, cube?.lodBias ?? 0, cube?.distance.exponent ?? 1) },
    uCubeFogHeight: { value: new Vector4(chBias, chScale, chScale === 0 ? 0 : (cube?.height.exponent ?? 1), Math.max(0, Math.log2(sky?.size ?? 8) - 3)) },
    uCubeFogColor: { value: new Vector2(cube?.max ?? 0, scene.sky?.brightness ?? 1) },
    uFogSky: { value: sky?.cube ?? null },
    uFogSkyBasis: { value: sky?.basis ?? new Matrix3() },
    uFogStrength: { value: 1 },
  };
}
