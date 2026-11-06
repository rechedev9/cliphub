import { Vector2, Vector3 } from 'three';
import type { Data3DTexture, DataTexture } from 'three';
import type { CompressedArrayTexture } from 'three';
import type { MapScene, SceneProbe } from '@/lib/api/map-scene';
import { CELL_ITEMS, boxGrid, rowTable } from './scene-grid';
import type { Box } from './scene-grid';

/**
 * Texels per probe in the probe table: the 3 rows of its transform, min and its cube,
 * max and whether it is a box, the edge fades, the projection box, the harmonic.
 */
const PROBE_TEXELS = 9;

/**
 * CS2's environment specular, after ValveResourceFormat's envmap and lighting
 * shaders: the reflection bent toward the normal as the surface roughens, box
 * projected into each baked probe the fragment is inside, blended over their
 * edge fades in priority order with the sky taking what is left. Rough
 * lightmapped non-metals reflect the lightmap instead. The result is scaled by
 * the split sum BRDF and darkened where the lightmap is darker than the probe
 * saw (specular occlusion). The sun adds its GGX highlight.
 */
export const CS2_REFLECT = `
uniform sampler2DArray uEnvArray;
uniform sampler2D uProbes;
uniform sampler3D uProbeGrid;
uniform vec3 uGridMin;
uniform vec3 uGridScale;
uniform vec2 uEnvSize;
uniform float uUseEnv;

float cs2Luma(vec3 c) {
  return dot(c, vec3(0.2126, 0.7152, 0.0722));
}

// The face and coordinates of a direction, as GL picks them from a cube map.
vec3 cs2CubeLayer(vec3 d) {
  vec3 a = abs(d);
  if (a.x >= a.y && a.x >= a.z) return d.x > 0.0 ? vec3(-d.z, -d.y, 0.0) / vec3(a.x, a.x, 1.0) : vec3(d.z, -d.y, 1.0) / vec3(a.x, a.x, 1.0);
  if (a.y >= a.z) return d.y > 0.0 ? vec3(d.x, d.z, 2.0) / vec3(a.y, a.y, 1.0) : vec3(d.x, -d.z, 3.0) / vec3(a.y, a.y, 1.0);
  return d.z > 0.0 ? vec3(d.x, -d.y, 4.0) / vec3(a.z, a.z, 1.0) : vec3(-d.x, -d.y, 5.0) / vec3(a.z, a.z, 1.0);
}

vec3 cs2EnvSample(vec3 dir, float cube, float lod) {
  vec3 f = cs2CubeLayer(dir);
  // Each face is its own layer, so keep half a texel of the mip inside it.
  float size = max(1.0, uEnvSize.x / exp2(floor(lod)));
  vec2 uv = clamp(f.xy * 0.5 + 0.5, 0.5 / size, 1.0 - 0.5 / size);
  return textureLod(uEnvArray, vec3(uv, cube * 6.0 + f.z), lod).rgb;
}

vec4 cs2Probe(int probe, int texel) {
  return texelFetch(uProbes, ivec2(texel, probe), 0);
}

// The probes' reflection along R for a surface at world with normal N; the alpha is
// the luminance the probes saw along N, for the specular occlusion.
vec4 cs2ProbeReflection(vec3 world, vec3 N, vec3 R, float roughness, float lod) {
  vec3 env = vec3(0.0);
  vec4 sh = vec4(0.0);
  float total = 0.0;
  vec4 ids = texture(uProbeGrid, (world - uGridMin) * uGridScale) * 255.0;
  for (int k = 0; k < ${CELL_ITEMS}; k++) {
    int id = int(ids[k] + 0.5) - 1;
    if (id < 0 || total > 0.99) break;
    mat3x4 toLocal = mat3x4(cs2Probe(id, 0), cs2Probe(id, 1), cs2Probe(id, 2));
    vec3 local = vec4(world, 1.0) * toLocal;
    vec4 lo = cs2Probe(id, 3);
    vec4 hi = cs2Probe(id, 4);
    vec3 invFade = cs2Probe(id, 5).xyz;
    vec3 fades = min(clamp((local - lo.xyz) * invFade, 0.0, 1.0), clamp((hi.xyz - local) * invFade, 0.0, 1.0));
    float fade = min(fades.x, min(fades.y, fades.z));
    if (fade <= 0.0) continue;
    float weight = smoothstep(0.0, 1.0, fade) * (1.0 - total);
    vec3 localR = vec4(R, 0.0) * toLocal;
    vec3 coords = localR;
    if (hi.w > 0.5) {
      // Where the ray leaves the projection box, giving way to the plain reflection as it roughens.
      vec3 far = max((cs2Probe(id, 6).xyz - local) / localR, (cs2Probe(id, 7).xyz - local) / localR);
      vec3 hit = local + localR * abs(min(far.x, min(far.y, far.z)));
      coords = mix(normalize(hit), localR, roughness);
    }
    env += cs2EnvSample(coords, lo.w, lod) * weight;
    sh += cs2Probe(id, 8) * weight;
    total += weight;
  }
  float sky = 1.0 - total;
  vec3 skyLight = textureLod(uFogSky, uFogSkyBasis * R, 5.0).rgb * uCubeFogColor.y;
  env += skyLight * sky;
  return vec4(env, dot(vec4(N, 1.0), sh) + cs2Luma(skyLight) * sky);
}

// Karis's fit of the split sum: x is the reflectance at F0 = 0, y at F0 = 1.
vec2 cs2EnvBrdf(float roughness, float NoV) {
  vec4 r = roughness * vec4(-1.0, -0.0275, -0.572, 0.022) + vec4(1.0, 0.0425, 1.04, -0.04);
  float a = min(r.x * r.x, exp2(-9.28 * NoV)) * r.x + r.y;
  vec2 ab = vec2(-1.04, 1.04) * a + r.zw;
  return vec2(ab.y, ab.x + ab.y);
}

// The sun's highlight: GGX without the 1/pi CS2's lights already carry, height-correlated Smith.
vec3 cs2SunSpecular(vec3 N, vec3 V, vec3 L, float roughness, vec3 f0) {
  float NoL = clamp(dot(N, L), 0.0, 1.0);
  if (NoL <= 0.0) return vec3(0.0);
  vec3 H = normalize(V + L);
  float NoH = clamp(dot(N, H), 0.0, 1.0);
  float NoV = clamp(dot(N, V), 1e-4, 1.0);
  float alpha = max(roughness * roughness, 1e-4);
  float k = min(alpha / (1.0 - NoH * NoH + NoH * NoH * alpha * alpha), 453.5);
  float vis = 0.5 / max(NoL * (NoV * (1.0 - alpha) + alpha) + NoV * (NoL * (1.0 - alpha) + alpha), 1e-5);
  vec3 F = f0 + (1.0 - f0) * pow(1.0 - clamp(dot(L, H), 0.0, 1.0), 5.0);
  return k * k * vis * NoL * F;
}

// Diffuse and specular indirect light; lightmapped is 1 where the irradiance is baked.
void cs2Indirect(inout vec3 diffuse, out vec3 specular, vec3 world, vec3 N, vec3 V, float roughness, float metalness, vec3 f0, float lightmapped) {
  float NoV = clamp(dot(N, V), 1e-4, 1.0);
  vec2 lut = cs2EnvBrdf(roughness, NoV);
  vec3 single = mix(vec3(lut.x), vec3(lut.y), f0);
  float rough2 = roughness * roughness;
  float smoothness = clamp(1.0 - rough2, 0.0, 1.0);
  vec3 R = normalize(mix(N, reflect(-V, N), smoothness * (sqrt(smoothness) + rough2)));
  float fromLightmap = clamp(-2.0 + 10.0 * roughness, 0.0, 1.0) * (1.0 - metalness) * lightmapped;
  vec4 env = vec4(diffuse, cs2Luma(diffuse));
  if (fromLightmap < 1.0 && uUseEnv > 0.5) {
    vec4 probes = cs2ProbeReflection(world, N, R, roughness, sqrt(roughness) * uEnvSize.y);
    env = mix(probes, env, fromLightmap);
  }
  specular = env.rgb * single;
  if (env.a > 0.0) specular *= min(cs2Luma(diffuse) / env.a, 1.0);
  // Multiple scattering: what one bounce misses comes back through the average Fresnel.
  float missing = 1.0 - lut.y;
  vec3 averageF = f0 + (1.0 - f0) / 21.0;
  vec3 multi = single * averageF / (1.0 - averageF * missing) * missing;
  specular += multi * diffuse;
  diffuse *= max(vec3(0.0), 1.0 - single - multi);
}
`;

export type Cs2ReflectUniforms = {
  uEnvArray: { value: CompressedArrayTexture | null };
  uProbes: { value: DataTexture | null };
  uProbeGrid: { value: Data3DTexture | null };
  uGridMin: { value: Vector3 };
  uGridScale: { value: Vector3 };
  uEnvSize: { value: Vector2 };
  uUseEnv: { value: number };
};

function probeTable(probes: SceneProbe[]): DataTexture {
  return rowTable(probes, PROBE_TEXELS, (data, row, p) => {
    data.set(p.worldToLocal, row);
    data.set([...p.min, p.cube], row + 12);
    data.set([...p.max, p.box ? 1 : 0], row + 16);
    data.set([...p.invFade, 0], row + 20);
    data.set([...p.projectMin, 0], row + 24);
    data.set([...p.projectMax, 0], row + 28);
    data.set(p.normalization, row + 32);
  });
}

/** The world box a probe's local box covers. */
function worldBox(p: SceneProbe): Box {
  const m = p.worldToLocal;
  // Local to world is the transposed rotation and the moved origin.
  const origin = [0, 1, 2].map((j) => -(m[j] * m[3] + m[4 + j] * m[7] + m[8 + j] * m[11]));
  const lo = [Infinity, Infinity, Infinity];
  const hi = [-Infinity, -Infinity, -Infinity];
  for (let c = 0; c < 8; c++) {
    const local = [c & 1 ? p.max[0] : p.min[0], c & 2 ? p.max[1] : p.min[1], c & 4 ? p.max[2] : p.min[2]];
    for (let j = 0; j < 3; j++) {
      const w = origin[j] + m[j] * local[0] + m[4 + j] * local[1] + m[8 + j] * local[2];
      lo[j] = Math.min(lo[j], w);
      hi[j] = Math.max(hi[j], w);
    }
  }
  return { lo, hi };
}

/** The reflection uniforms of `scene`; without a probe array only the sky and the lightmap are reflected. */
export function cs2ReflectUniforms(options: { scene: MapScene; array: CompressedArrayTexture | null }): Cs2ReflectUniforms {
  const { scene, array } = options;
  const reflections = scene.reflections;
  const source = reflections === null ? undefined : scene.textures[reflections.texture];
  if (reflections === null || array === null || source === undefined || reflections.probes.length === 0) {
    return { uEnvArray: { value: null }, uProbes: { value: null }, uProbeGrid: { value: null }, uGridMin: { value: new Vector3() }, uGridScale: { value: new Vector3() }, uEnvSize: { value: new Vector2(1, 0) }, uUseEnv: { value: 0 } };
  }
  const grid = boxGrid(scene, reflections.probes.map(worldBox));
  // Full roughness stops at the second smallest mip, as CS2's does.
  const lastLod = Math.max(0, source.levels.length - 2);
  return {
    uEnvArray: { value: array },
    uProbes: { value: probeTable(reflections.probes) },
    uProbeGrid: { value: grid.texture },
    uGridMin: { value: grid.min },
    uGridScale: { value: grid.scale },
    uEnvSize: { value: new Vector2(source.width, lastLod) },
    uUseEnv: { value: 1 },
  };
}

export function disposeReflect(uniforms: Cs2ReflectUniforms): void {
  uniforms.uEnvArray.value?.dispose();
  uniforms.uProbes.value?.dispose();
  uniforms.uProbeGrid.value?.dispose();
}
