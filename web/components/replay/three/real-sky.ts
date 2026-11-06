import { BackSide, BoxGeometry, Matrix3, Mesh, ShaderMaterial, Vector3 } from 'three';
import type { CompressedCubeTexture, WebGLRenderer } from 'three';
import type { MapScene } from '@/lib/api/map-scene';
import { CS2_POST } from './cs2-post';
import type { Cs2PostUniforms } from './cs2-post';
import { toCubeTexture } from './scene-textures';

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
uniform float uBrightness;
uniform float uMeter;
varying vec3 vDirection;

${CS2_POST}

void main() {
  vec3 sky = textureCube(uSky, uBasis * normalize(vDirection)).rgb * uTint;
  if (uMeter > 0.5) {
    gl_FragColor = vec4(vec3(dot(sky * uBrightness, vec3(0.2126, 0.7152, 0.0722))), 1.0);
    return;
  }
  gl_FragColor = vec4(cs2Grade(cs2Tone(sky * uExposure)), 1.0);
  #include <colorspace_fragment>
}
`;

/** The map's sky cube, drawn round the camera behind every surface. */
export class RealSky {
  readonly mesh: Mesh<BoxGeometry, ShaderMaterial>;
  readonly #cube: CompressedCubeTexture;
  readonly #brightness: number;
  /** What the cubemap fog samples: the cube, its turn and its side in texels. */
  readonly fogSource: { cube: CompressedCubeTexture; basis: Matrix3; size: number };

  private constructor(cube: CompressedCubeTexture, options: { scene: MapScene; post: Cs2PostUniforms; meter: { value: number }; size: number }) {
    const { scene, post, meter, size } = options;
    this.#cube = cube;
    const sky = scene.sky;
    this.#brightness = sky?.brightness ?? 1;
    const yaw = ((sky?.yaw ?? 0) * Math.PI) / 180;
    this.fogSource = { cube, basis: skyBasis(yaw), size };
    const material = new ShaderMaterial({
      vertexShader: SKY_VERTEX,
      fragmentShader: SKY_FRAGMENT,
      uniforms: {
        uSky: { value: cube },
        uBasis: { value: this.fogSource.basis },
        uTint: { value: new Vector3(...(sky?.tint ?? [1, 1, 1])) },
        uExposure: { value: this.#brightness },
        uBrightness: { value: this.#brightness },
        uMeter: meter,
        ...post,
      },
      side: BackSide,
      depthWrite: false,
    });
    this.mesh = new Mesh(new BoxGeometry(2, 2, 2), material);
    this.mesh.frustumCulled = false;
    this.mesh.renderOrder = -10;
  }

  /** The sky of `scene`, or null when it has none or the GPU cannot read it. */
  static build(options: { scene: MapScene; renderer: WebGLRenderer; post: Cs2PostUniforms; meter: { value: number } }): RealSky | null {
    const { scene, renderer, post, meter } = options;
    const sky = scene.sky;
    const source = sky === null ? undefined : scene.textures[sky.texture];
    if (sky === null || source === undefined) return null;
    const cube = toCubeTexture(source, renderer);
    return cube === null ? null : new RealSky(cube, { scene, post, meter, size: source.width });
  }

  /** `exposure` is the map's exposure times the look's own factor. */
  setExposure(exposure: number): void {
    this.mesh.material.uniforms.uExposure.value = exposure * this.#brightness;
  }

  dispose(): void {
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
    this.#cube.dispose();
  }
}

/** The world direction less the sky's yaw; the cube is sampled in Source's z-up axes, as CS2 does. */
function skyBasis(yaw: number): Matrix3 {
  const c = Math.cos(-yaw);
  const s = Math.sin(-yaw);
  return new Matrix3().set(c, -s, 0, s, c, 0, 0, 0, 1);
}
