import { CS2_FOG } from './cs2-fog';
import { CS2_LIGHTS } from './cs2-lights';
import { CS2_POST } from './cs2-post';
import { CS2_REFLECT } from './cs2-reflect';
import { MAP_DECLARE } from './map-shader';

/**
 * The shading of the map's real surfaces, after CS2's own: albedo times the
 * baked irradiance plus the sun through its baked shadow channel. Props carry
 * no lightmap, so they take the sky's ambient and the sun through a shadow
 * map rendered once from the sun. The normal and roughness texture shapes the
 * sun's highlight, the reflections (cs2-reflect.ts) and the stationary lights
 * (cs2-lights.ts). Then the map's fog,
 * exposure and a filmic curve. Defines: USE_COLOR_MAP, USE_LAYER2,
 * USE_BLEND_MOD, USE_TINT, USE_LIGHTMAP, USE_NORMAL_MAP, USE_NORMAL_MAP2,
 * ALPHA_TEST, UNLIT.
 */

/** The plain map's cutaway without its void: the real map keeps the scenery round the playable area. */
const REAL_SURFACE_CUT = `
vec3 mapFacet = normalize(cross(dFdx(vMapPos), dFdy(vMapPos)));
float mapFlat = step(0.7, abs(mapFacet.z));
vec4 mapCell = texture2D(uField, (vMapPos.xy - uFieldRect.xy) * uFieldRect.zw);
float mapCutting = uUseField * uCutaway * (1.0 - step(0.5, mapCell.b));
float mapRise = vMapPos.z - (mapCutting > 0.5 ? mapFloor(mapCell, vMapPos) : mapCell.r);
if (vMapPos.z > uCeiling) discard;
if (uUseField * uPlayableOnly > 0.5 && mapCell.b > 0.45) discard;
if (mapCutting > 0.5 && (mapRise > uWallCut || (mapFlat > 0.5 && mapRise > uSlabCut))) discard;
`;

export const REAL_MAP_VERTEX = `
attribute vec2 aLightmapUv;
attribute vec4 aTint;
attribute vec4 aBlend;
uniform vec4 uUvTransform;
uniform vec4 uUv2Transform;
uniform vec2 uLightmapScale;
uniform mat4 uSunMatrix;
varying vec3 vMapPos;
varying vec3 vWorldNormal;
varying vec2 vUv;
varying vec2 vUv2;
varying vec2 vLightmapUv;
varying vec4 vTint;
varying float vBlend;
varying vec4 vSunCoord;

void main() {
  vec4 world = modelMatrix * vec4(position, 1.0);
  vMapPos = world.xyz;
  // Normals are stored in world space; the chunk's scale only unpacks positions.
  vWorldNormal = normal;
  vUv = uv * uUvTransform.xy + uUvTransform.zw;
  vUv2 = uv * uUv2Transform.xy + uUv2Transform.zw;
#ifdef USE_LIGHTMAP
  vLightmapUv = aLightmapUv * uLightmapScale;
#else
  vLightmapUv = vec2(0.0);
#endif
#ifdef USE_TINT
  vTint = aTint;
#else
  vTint = vec4(1.0);
#endif
#ifdef USE_LAYER2
  vBlend = aBlend.r;
#else
  vBlend = 0.0;
#endif
  vSunCoord = uSunMatrix * world;
  gl_Position = projectionMatrix * viewMatrix * world;
}
`;

export const REAL_MAP_FRAGMENT = `
${MAP_DECLARE}
uniform sampler2D uColor;
uniform sampler2D uColor2;
uniform sampler2D uBlendMod;
uniform sampler2D uNormalMap;
uniform sampler2D uNormalMap2;
uniform float uMetalness;
uniform sampler2D uIrradiance;
uniform sampler2D uShadows;
uniform vec4 uShadowMask;
uniform float uSunScale;
uniform sampler2D uSunDepth;
uniform vec3 uTint;
uniform vec3 uTint2;
uniform vec3 uAverage;
uniform float uAlphaRef;
uniform float uBlendSoftness;
uniform float uBlendMode;
uniform vec3 uSunDirection;
uniform vec3 uSunColor;
uniform vec3 uAmbient;
uniform float uFogLift;
uniform float uExposure;
uniform float uSunDepthTexel;
uniform float uUseSunDepth;
uniform float uIrradianceScale;
uniform float uOpacity;
// 1 where the texture's alpha is the surface's coverage: decals, overlays and glass.
uniform float uUseAlpha;
uniform float uMeter;
varying vec3 vWorldNormal;
varying vec2 vUv;
varying vec2 vUv2;
varying vec2 vLightmapUv;
varying vec4 vTint;
varying float vBlend;
varying vec4 vSunCoord;

vec3 srgbToLinear(vec3 c) {
  return mix(c / 12.92, pow((c + 0.055) / 1.055, vec3(2.4)), step(vec3(0.04045), c));
}

${CS2_POST}
${CS2_FOG}
${CS2_REFLECT}
${CS2_LIGHTS}

// A hemi-octahedral tangent normal (DirectX green) turned into world space along the uv's own axes.
vec3 cs2NormalMap(vec2 rg, vec3 N, vec2 uv) {
  vec2 e = vec2(rg.x + rg.y - 1.003922, rg.x - rg.y);
  vec3 t = normalize(vec3(e, 1.0 - abs(e.x) - abs(e.y)));
  t.y = -t.y;
  vec3 dp1 = dFdx(vMapPos);
  vec3 dp2 = dFdy(vMapPos);
  vec2 duv1 = dFdx(uv);
  vec2 duv2 = dFdy(uv);
  vec3 dp2perp = cross(dp2, N);
  vec3 dp1perp = cross(N, dp1);
  vec3 T = dp2perp * duv1.x + dp1perp * duv2.x;
  vec3 B = dp2perp * duv1.y + dp1perp * duv2.y;
  float scale = inversesqrt(max(max(dot(T, T), dot(B, B)), 1e-20));
  return normalize(mat3(T * scale, B * scale, N) * t);
}

float sunShadowMap(vec3 normal) {
  if (uUseSunDepth < 0.5) return 1.0;
  vec3 coord = vSunCoord.xyz / vSunCoord.w * 0.5 + 0.5;
  if (coord.x <= 0.0 || coord.y <= 0.0 || coord.x >= 1.0 || coord.y >= 1.0 || coord.z >= 1.0) return 1.0;
  float slope = 1.0 - clamp(dot(normal, uSunDirection), 0.0, 1.0);
  float bias = 0.0006 + 0.0025 * slope;
  float lit = 0.0;
  for (int x = -1; x <= 1; x++) {
    for (int y = -1; y <= 1; y++) {
      float depth = texture2D(uSunDepth, coord.xy + vec2(float(x), float(y)) * uSunDepthTexel).r;
      lit += coord.z - bias <= depth ? 1.0 : 0.0;
    }
  }
  return lit / 9.0;
}

void main() {
${REAL_SURFACE_CUT}
#ifdef USE_COLOR_MAP
  vec4 albedo = texture2D(uColor, vUv);
#else
  vec4 albedo = vec4(uAverage, 1.0);
#endif
#ifdef ALPHA_TEST
  if (albedo.a < uAlphaRef) discard;
#endif
  albedo.rgb = srgbToLinear(albedo.rgb) * srgbToLinear(uTint);
  float layerMix = 0.0;
#ifdef USE_LAYER2
  vec4 layer2 = texture2D(uColor2, vUv2);
  float factor = vBlend;
#ifdef USE_BLEND_MOD
  vec4 modulation = texture2D(uBlendMod, vUv);
  float softness = uBlendMode == 1.0 ? modulation.r : uBlendSoftness;
  float mask = uBlendMode == 3.0 ? modulation.a : modulation.g;
  factor = smoothstep(max(0.0, mask - softness), min(1.0, mask + softness), vBlend);
#endif
  albedo.rgb = mix(albedo.rgb, srgbToLinear(layer2.rgb) * srgbToLinear(uTint2), factor);
  layerMix = factor;
#endif
  albedo.rgb *= srgbToLinear(vTint.rgb);

  vec3 geometric = normalize(vWorldNormal);
  if (!gl_FrontFacing) geometric = -geometric;
  vec3 normal = geometric;
  float roughness = 1.0;
#ifdef USE_NORMAL_MAP
  vec4 normalRoughness = texture2D(uNormalMap, vUv);
#ifdef USE_NORMAL_MAP2
  normalRoughness = mix(normalRoughness, texture2D(uNormalMap2, vUv2), layerMix);
#endif
  normal = cs2NormalMap(normalRoughness.rg, geometric, vUv);
  roughness = normalRoughness.b;
#endif
  // CS2 lights opaque surfaces no smoother than this.
  roughness = max(roughness, 0.05);
  vec3 color;
#ifdef UNLIT
  color = albedo.rgb;
#else
  float sunAngle = max(dot(normal, uSunDirection), 0.0);
#ifdef USE_LIGHTMAP
  vec3 irradiance = texture2D(uIrradiance, vLightmapUv).rgb * uIrradianceScale;
  vec4 baked = texture2D(uShadows, vLightmapUv);
  float visibility = 1.0 - clamp(dot(baked, uShadowMask), 0.0, 1.0);
  float lightmapped = 1.0;
#else
  // The map's average baked light stands in for the light probes CS2 lights props with.
  vec3 irradiance = uAmbient * uIrradianceScale * (0.7 + 0.3 * normal.z);
  float visibility = sunShadowMap(geometric);
  vec4 baked = vec4(0.0);
  float lightmapped = 0.0;
#endif
  vec3 sun = uSunColor * uSunScale * visibility;
  vec3 view = normalize(cameraPosition - vMapPos);
  vec3 f0 = mix(vec3(0.04), albedo.rgb, uMetalness);
  vec3 specular;
  cs2Indirect(irradiance, specular, vMapPos, normal, view, roughness, uMetalness, f0, lightmapped);
  vec3 lightDiffuse = vec3(0.0);
  vec3 lightSpecular = vec3(0.0);
  cs2Lights(lightDiffuse, lightSpecular, vMapPos, normal, view, roughness, f0, baked);
  color = albedo.rgb * (1.0 - uMetalness) * (irradiance + sun * sunAngle + lightDiffuse) + specular + lightSpecular + sun * cs2SunSpecular(normal, view, uSunDirection, roughness, f0);
#endif
  color = cs2Fog(color, vMapPos, uFogLift);
  if (uMeter > 0.5) {
    gl_FragColor = vec4(vec3(dot(color, vec3(0.2126, 0.7152, 0.0722))), 1.0);
    return;
  }
  color = cs2Tone(color * uExposure);
  // A pale rim where the cutaway opens a wall, as on the plain map.
  if (uUseField > 0.5 && mapCutting > 0.5 && mapFlat < 0.5) {
    float rim = 1.5 * length(vec2(dFdx(vMapPos.z), dFdy(vMapPos.z)));
    if (mapRise > uWallCut - rim) color = vec3(0.62, 0.65, 0.7);
  }
  gl_FragColor = vec4(cs2Grade(color), uOpacity * mix(1.0, albedo.a, uUseAlpha));
  #include <colorspace_fragment>
}
`;

/** Writes depth only, for the sun's shadow map; alpha tested leaves cut their holes. */
export const SUN_DEPTH_VERTEX = `
varying vec2 vUv;
uniform vec4 uUvTransform;
void main() {
  vUv = uv * uUvTransform.xy + uUvTransform.zw;
  gl_Position = projectionMatrix * viewMatrix * modelMatrix * vec4(position, 1.0);
}
`;

export const SUN_DEPTH_FRAGMENT = `
uniform sampler2D uColor;
uniform float uAlphaRef;
varying vec2 vUv;
void main() {
#ifdef ALPHA_TEST
  if (texture2D(uColor, vUv).a < uAlphaRef) discard;
#endif
  gl_FragColor = vec4(1.0);
}
`;
