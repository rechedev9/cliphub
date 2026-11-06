import {
  BufferAttribute,
  BufferGeometry,
  DepthTexture,
  DoubleSide,
  Float16BufferAttribute,
  FrontSide,
  Group,
  Matrix4,
  Mesh,
  NearestFilter,
  OrthographicCamera,
  ShaderMaterial,
  Texture,
  UnsignedIntType,
  Vector2,
  Vector3,
  Vector4,
  WebGLRenderTarget,
} from 'three';
import type { WebGLRenderer } from 'three';
import type { MapScene, SceneChunk, SceneMaterial } from '@/lib/api/map-scene';
import { blankTexture } from './map-shader';
import type { MapUniforms } from './map-shader';
import { REAL_MAP_FRAGMENT, REAL_MAP_VERTEX, SUN_DEPTH_FRAGMENT, SUN_DEPTH_VERTEX } from './real-map-shader';
import { RealSky } from './real-sky';
import { toTexture } from './scene-textures';

/** Side of the sun's shadow map, in texels. */
const SUN_DEPTH_SIZE = 4096;

/** The look of the scene that does not come from the map file. */
export type RealMapLook = { exposure: number; fogStrength: number; irradiance: number };

export const DEFAULT_LOOK: RealMapLook = { exposure: 1.0, fogStrength: 0.5, irradiance: 1.0 };

/** The scene's light a mid exposure maps to mid grey, tuned on Inferno and Mirage. */
const EXPOSURE_KEY = 2.4;

/** The lightmaps store irradiance over pi; the sun's light is not divided, so it gains pi against them. */
const SUN_SCALE = Math.PI;

function luminance(c: readonly number[]): number {
  return 0.2126 * (c[0] ?? 0) + 0.7152 * (c[1] ?? 0) + 0.0722 * (c[2] ?? 0);
}

/** CS2 adapts exposure to each map's light; this sets it once from the sun and the baked average. */
function autoExposure(scene: MapScene, ambient: Vector3): number {
  const light = SUN_SCALE * luminance(scene.sun?.color ?? [0, 0, 0]) + 1.5 * luminance([ambient.x, ambient.y, ambient.z]);
  return Math.min(1.2, Math.max(0.1, EXPOSURE_KEY / Math.max(0.5, light)));
}

function chunkGeometry(chunk: SceneChunk): BufferGeometry {
  const geometry = new BufferGeometry();
  geometry.setAttribute('position', new BufferAttribute(chunk.position, 3, true));
  geometry.setAttribute('normal', new BufferAttribute(chunk.normal, 3, true));
  geometry.setAttribute('uv', new Float16BufferAttribute(chunk.uv, 2));
  if (chunk.uv2 !== null) geometry.setAttribute('aLightmapUv', new BufferAttribute(chunk.uv2, 2, true));
  if (chunk.color !== null) geometry.setAttribute('aTint', new BufferAttribute(chunk.color, 4, true));
  if (chunk.blend !== null) geometry.setAttribute('aBlend', new BufferAttribute(chunk.blend, 4, true));
  geometry.setIndex(new BufferAttribute(chunk.index, 1));
  geometry.computeBoundingSphere();
  return geometry;
}

type Shared = {
  irradiance: Texture | null;
  shadows: Texture | null;
  shadowMask: Vector4;
  sunDepth: DepthTexture | null;
  sunMatrix: Matrix4;
  lightmapScale: Vector2;
};

/**
 * The map as CS2 draws it: its render meshes with their textures, lit by the
 * baked lightmaps and the sun. It shares the plain map's cutaway uniforms, so
 * every cut the scene sets on that map applies here too.
 */
export class RealMap {
  readonly group = new Group();
  readonly #textures: (Texture | null)[];
  readonly #materials = new Map<string, ShaderMaterial>();
  readonly #depthMaterials = new Map<string, ShaderMaterial>();
  readonly #meshes: Mesh<BufferGeometry, ShaderMaterial>[] = [];
  readonly #shared: Shared;
  readonly #scene: MapScene;
  readonly #cut: MapUniforms;
  readonly #sky: RealSky | null;
  #sunTarget: WebGLRenderTarget | null = null;
  #look: RealMapLook = DEFAULT_LOOK;
  readonly #ambient: Vector3;
  readonly #exposure: number;
  readonly #fogLift = { value: 0 };
  readonly #sunScale = { value: SUN_SCALE };

  constructor(scene: MapScene, cut: MapUniforms, renderer: WebGLRenderer) {
    this.#scene = scene;
    this.#cut = cut;
    const sky = scene.sun?.sky ?? [0.55, 0.62, 0.75];
    this.#ambient = new Vector3(...(scene.lightmap?.average ?? [sky[0] * 0.55, sky[1] * 0.55, sky[2] * 0.55]));
    this.#exposure = autoExposure(scene, this.#ambient);
    const lightmapTextures = new Set([scene.lightmap?.irradiance, scene.lightmap?.shadows, scene.lightmap?.directional]);
    this.#textures = scene.textures.map((texture, index) => toTexture(texture, renderer, !lightmapTextures.has(index)));
    const lightmap = scene.lightmap;
    this.#shared = {
      irradiance: lightmap === null ? null : (this.#textures[lightmap.irradiance] ?? null),
      shadows: lightmap === null || lightmap.shadows < 0 ? null : (this.#textures[lightmap.shadows] ?? null),
      shadowMask: shadowMask(scene.sun?.shadowChannel ?? -1),
      sunDepth: null,
      sunMatrix: new Matrix4(),
      lightmapScale: new Vector2(lightmap?.uvScale[0] ?? 1, lightmap?.uvScale[1] ?? 1),
    };
    for (const chunk of scene.chunks) {
      const material = scene.materials[chunk.material];
      if (material === undefined) continue;
      const shader = this.#material(material, chunk);
      const mesh = new Mesh(chunkGeometry(chunk), shader);
      mesh.position.set(chunk.min[0], chunk.min[1], chunk.min[2]);
      mesh.scale.set(Math.max(1e-3, chunk.max[0] - chunk.min[0]), Math.max(1e-3, chunk.max[1] - chunk.min[1]), Math.max(1e-3, chunk.max[2] - chunk.min[2]));
      mesh.renderOrder = renderOrder(material);
      mesh.userData.material = material;
      this.#meshes.push(mesh);
      this.group.add(mesh);
    }
    this.#bakeSunShadow(renderer);
    this.#sky = RealSky.build(scene, renderer);
    this.#sky?.setExposure(this.#look.exposure);
    if (this.#sky !== null) this.group.add(this.#sky.mesh);
  }

  /** Whether the map's own sky draws behind it. */
  get hasSky(): boolean {
    return this.#sky !== null;
  }

  /** Whether the scene has baked lighting; without it every surface is lit like a prop. */
  get baked(): boolean {
    return this.#shared.irradiance !== null;
  }

  get triangles(): number {
    return this.#scene.chunks.reduce((sum, chunk) => sum + chunk.index.length / 3, 0);
  }

  /**
   * How high the camera is over the map's floor: the fog starts that much further
   * out, so it hazes the distance from above as it does at eye level.
   */
  setCameraHeight(z: number): void {
    this.#fogLift.value = Math.max(0, z - this.#scene.min[2]) * 1.2;
  }

  setVisible(visible: boolean): void {
    this.group.visible = visible;
  }

  setLook(look: RealMapLook): void {
    this.#look = look;
    for (const material of this.#materials.values()) this.#applyLook(material);
    this.#sky?.setExposure(look.exposure);
  }

  dispose(): void {
    for (const mesh of this.#meshes) mesh.geometry.dispose();
    for (const material of this.#materials.values()) material.dispose();
    for (const material of this.#depthMaterials.values()) material.dispose();
    for (const texture of this.#textures) texture?.dispose();
    this.#sunTarget?.dispose();
    this.#sky?.dispose();
    this.#shared.sunDepth?.dispose();
  }

  #applyLook(material: ShaderMaterial): void {
    const fog = this.#scene.fog;
    const u = material.uniforms;
    u.uExposure.value = this.#look.exposure * this.#exposure;
    u.uIrradianceScale.value = this.#look.irradiance;
    u.uFog.value.set(fog?.start ?? 0, fog?.end ?? 1, (fog?.max ?? 0) * this.#look.fogStrength);
  }

  #material(material: SceneMaterial, chunk: SceneChunk): ShaderMaterial {
    const color = material.color < 0 ? null : (this.#textures[material.color] ?? null);
    const color2 = material.color2 < 0 ? null : (this.#textures[material.color2] ?? null);
    const blendMod = material.blend < 0 ? null : (this.#textures[material.blend] ?? null);
    const layered = color2 !== null && chunk.blend !== null;
    const lightmapped = chunk.uv2 !== null && this.#shared.irradiance !== null;
    // Foliage keeps wind weights in its vertex colour, not a tint.
    const tinted = chunk.color !== null && !material.shader.includes('foliage');
    const key = `${material.name}|${lightmapped}|${tinted}|${layered}`;
    const found = this.#materials.get(key);
    if (found !== undefined) return found;
    const defines: Record<string, string> = {};
    if (color !== null) defines.USE_COLOR_MAP = '';
    if (layered) defines.USE_LAYER2 = '';
    if (layered && blendMod !== null && material.blendMode > 0) defines.USE_BLEND_MOD = '';
    if (tinted) defines.USE_TINT = '';
    if (lightmapped) defines.USE_LIGHTMAP = '';
    if (material.alphaTest && color !== null) defines.ALPHA_TEST = '';
    if (material.unlit) defines.UNLIT = '';
    const sun = this.#scene.sun;
    const sky = sun?.sky ?? [0.55, 0.62, 0.75];
    const sunColor = sun?.color ?? [2.2, 2.0, 1.8];
    const sunDirection = new Vector3(...(sun?.direction ?? [-0.4, -0.4, 0.8])).normalize();
    const shader = new ShaderMaterial({
      vertexShader: REAL_MAP_VERTEX,
      fragmentShader: REAL_MAP_FRAGMENT,
      defines,
      uniforms: {
        ...this.#cut,
        uColor: { value: color ?? blankTexture() },
        uColor2: { value: color2 ?? blankTexture() },
        uBlendMod: { value: blendMod ?? blankTexture() },
        uIrradiance: { value: this.#shared.irradiance ?? blankTexture() },
        uShadows: { value: this.#shared.shadows ?? blankTexture() },
        uShadowMask: { value: this.#shared.shadowMask },
        uSunScale: this.#sunScale,
        uSunDepth: { value: blankTexture() },
        uUseSunDepth: { value: 0 },
        uSunDepthTexel: { value: 1 / SUN_DEPTH_SIZE },
        uSunMatrix: { value: this.#shared.sunMatrix },
        uLightmapScale: { value: this.#shared.lightmapScale },
        uUvTransform: { value: new Vector4(material.uvScale[0], material.uvScale[1], material.uvOffset[0], material.uvOffset[1]) },
        uUv2Transform: { value: new Vector4(material.uv2Scale[0] * material.uvScale[0], material.uv2Scale[1] * material.uvScale[1], material.uv2Offset[0], material.uv2Offset[1]) },
        uTint: { value: new Vector3(...material.tint) },
        uTint2: { value: new Vector3(...material.tint2) },
        uAverage: { value: new Vector3(...material.average) },
        uAlphaRef: { value: material.alphaRef },
        uBlendSoftness: { value: material.blendSoftness },
        uBlendMode: { value: material.blendMode },
        uSunDirection: { value: sunDirection },
        uSunColor: { value: new Vector3(...sunColor) },
        uAmbient: { value: this.#ambient },
        uFogLift: this.#fogLift,
        uExposure: { value: 1 },
        uIrradianceScale: { value: 1 },
        uFogColor: { value: new Vector3(...fogColor(sky)) },
        uFog: { value: new Vector3() },
        uOpacity: { value: 1 },
      },
      side: material.doubleSided || !lightmapped ? DoubleSide : FrontSide,
      transparent: material.translucent || material.overlay,
      depthWrite: !(material.translucent || material.overlay),
      polygonOffset: material.overlay,
      polygonOffsetFactor: material.overlay ? -2 : 0,
      polygonOffsetUnits: material.overlay ? -2 : 0,
    });
    if (material.translucent) shader.uniforms.uOpacity.value = 0.55;
    this.#applyLook(shader);
    this.#materials.set(key, shader);
    return shader;
  }

  /**
   * Renders the map's depth from the sun once: the map does not move, so props
   * and the players' surroundings keep that shadow for the whole session.
   */
  #bakeSunShadow(renderer: WebGLRenderer): void {
    const sun = this.#scene.sun;
    const max = renderer.capabilities.maxTextureSize;
    if (sun === null || max < 4096) return;
    const size = Math.min(SUN_DEPTH_SIZE, max);
    const depth = new DepthTexture(size, size, UnsignedIntType);
    depth.minFilter = NearestFilter;
    depth.magFilter = NearestFilter;
    const target = new WebGLRenderTarget(size, size, { depthTexture: depth, depthBuffer: true });
    const direction = new Vector3(...sun.direction).normalize();
    const min = new Vector3(...this.#scene.min);
    const max3 = new Vector3(...this.#scene.max);
    const centre = min.clone().add(max3).multiplyScalar(0.5);
    const radius = max3.clone().sub(min).length() / 2;
    const camera = new OrthographicCamera(-radius, radius, radius, -radius, 1, radius * 4);
    camera.up.set(0, 0, 1);
    if (Math.abs(direction.z) > 0.99) camera.up.set(0, 1, 0);
    camera.position.copy(centre).addScaledVector(direction, radius * 2);
    camera.lookAt(centre);
    camera.updateMatrixWorld();
    camera.updateProjectionMatrix();
    const swaps: [Mesh<BufferGeometry, ShaderMaterial>, ShaderMaterial][] = [];
    for (const mesh of this.#meshes) {
      const material = mesh.userData.material;
      if (!isSceneMaterial(material) || material.translucent || material.overlay) {
        mesh.visible = false;
        swaps.push([mesh, mesh.material]);
        continue;
      }
      swaps.push([mesh, mesh.material]);
      mesh.material = this.#depthMaterial(material, mesh.material);
    }
    const previous = renderer.getRenderTarget();
    renderer.setRenderTarget(target);
    renderer.clear();
    renderer.render(this.group, camera);
    renderer.setRenderTarget(previous);
    for (const [mesh, material] of swaps) {
      mesh.material = material;
      mesh.visible = true;
    }
    this.#sunTarget = target;
    this.#shared.sunDepth = depth;
    this.#shared.sunMatrix.multiplyMatrices(camera.projectionMatrix, camera.matrixWorldInverse);
    for (const material of this.#materials.values()) {
      material.uniforms.uSunDepth.value = depth;
      material.uniforms.uUseSunDepth.value = 1;
      material.uniforms.uSunDepthTexel.value = 1 / size;
    }
  }

  #depthMaterial(material: SceneMaterial, shading: ShaderMaterial): ShaderMaterial {
    const alpha = 'ALPHA_TEST' in shading.defines;
    const key = alpha ? material.name : '';
    const found = this.#depthMaterials.get(key);
    if (found !== undefined) return found;
    const depth = new ShaderMaterial({
      vertexShader: SUN_DEPTH_VERTEX,
      fragmentShader: SUN_DEPTH_FRAGMENT,
      defines: alpha ? { ALPHA_TEST: '' } : {},
      uniforms: {
        uColor: shading.uniforms.uColor,
        uAlphaRef: shading.uniforms.uAlphaRef,
        uUvTransform: shading.uniforms.uUvTransform,
      },
      side: DoubleSide,
      colorWrite: false,
    });
    this.#depthMaterials.set(key, depth);
    return depth;
  }
}

/** Decals draw over the opaque surfaces, glass and water last. */
function renderOrder(material: SceneMaterial): number {
  if (material.overlay) return 1;
  return material.translucent ? 2 : 0;
}

/** The baked shadow map's channel of the sun as a mask; none when the sun bakes no shadow. */
function shadowMask(channel: number): Vector4 {
  return new Vector4(channel === 0 ? 1 : 0, channel === 1 ? 1 : 0, channel === 2 ? 1 : 0, channel === 3 ? 1 : 0);
}

function isSceneMaterial(value: unknown): value is SceneMaterial {
  return typeof value === 'object' && value !== null && 'translucent' in value && 'overlay' in value && 'name' in value;
}

/** A hazy horizon: the sky lightened toward white, in linear light. */
function fogColor(sky: readonly number[]): [number, number, number] {
  return [Math.min(1, sky[0] * 0.75 + 0.18), Math.min(1, sky[1] * 0.75 + 0.18), Math.min(1, sky[2] * 0.75 + 0.2)];
}
