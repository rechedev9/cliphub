import { BackSide, BoxGeometry, Matrix3, Mesh, ShaderMaterial, Vector3 } from 'three';
import type { CompressedCubeTexture, WebGLRenderer } from 'three';
import type { MapScene } from '@/lib/api/map-scene';
import { TONEMAP } from './real-map-shader';
import { toCubeTexture } from './scene-textures';

/** The sky's average light the tonemap maps to a bright mid tone. */
const SKY_KEY = 0.9;

const SKY_VERTEX = `
varying vec3 vDirection;
void main() {
  vDirection = position;
  // Only the camera's turn moves the sky; z = w keeps it behind everything.
  vec4 clip = projectionMatrix * vec4(mat3(viewMatrix) * position, 1.0);
  gl_Position = clip.xyww;
}
`;

const SKY_FRAGMENT = `
uniform samplerCube uSky;
uniform mat3 uBasis;
uniform vec3 uTint;
uniform float uExposure;
varying vec3 vDirection;

${TONEMAP}

void main() {
  vec3 sky = textureCube(uSky, uBasis * normalize(vDirection)).rgb * uTint;
  gl_FragColor = vec4(filmic(sky * uExposure), 1.0);
  #include <colorspace_fragment>
}
`;

/** The map's sky cube, drawn round the camera behind every surface. */
export class RealSky {
  readonly mesh: Mesh<BoxGeometry, ShaderMaterial>;
  readonly #cube: CompressedCubeTexture;
  readonly #brightness: number;

  private constructor(cube: CompressedCubeTexture, scene: MapScene, brightness: number) {
    this.#cube = cube;
    this.#brightness = brightness;
    const sky = scene.sky;
    const yaw = ((sky?.yaw ?? 0) * Math.PI) / 180;
    const material = new ShaderMaterial({
      vertexShader: SKY_VERTEX,
      fragmentShader: SKY_FRAGMENT,
      uniforms: {
        uSky: { value: cube },
        uBasis: { value: skyBasis(yaw) },
        uTint: { value: new Vector3(...(sky?.tint ?? [1, 1, 1])) },
        uExposure: { value: brightness },
      },
      side: BackSide,
      depthWrite: false,
    });
    this.mesh = new Mesh(new BoxGeometry(2, 2, 2), material);
    this.mesh.frustumCulled = false;
    this.mesh.renderOrder = -10;
  }

  /** The sky of `scene`, or null when it has none or the GPU cannot read it. */
  static build(scene: MapScene, renderer: WebGLRenderer): RealSky | null {
    const sky = scene.sky;
    const source = sky === null ? undefined : scene.textures[sky.texture];
    if (sky === null || source === undefined) return null;
    const cube = toCubeTexture(source, renderer);
    // CS2 adapts its exposure to where the eye looks; the sky gets its own so it reads as bright day.
    const average = 0.2126 * sky.average[0] + 0.7152 * sky.average[1] + 0.0722 * sky.average[2];
    return cube === null ? null : new RealSky(cube, scene, Math.sqrt(sky.brightness) * (SKY_KEY / Math.max(0.05, average)));
  }

  /** `exposure` is the look's own factor, 1 by default. */
  setExposure(exposure: number): void {
    this.mesh.material.uniforms.uExposure.value = exposure * this.#brightness;
  }

  dispose(): void {
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
    this.#cube.dispose();
  }
}

/** Turns a z-up world direction, less the sky's yaw, into the cube's own axes. */
function skyBasis(yaw: number): Matrix3 {
  const c = Math.cos(-yaw);
  const s = Math.sin(-yaw);
  const turn = new Matrix3().set(c, -s, 0, s, c, 0, 0, 0, 1);
  // The cube keeps Source's axes: +X forward, +Y left, +Z up.
  return new Matrix3().set(1, 0, 0, 0, 0, 1, 0, -1, 0).multiply(turn);
}
