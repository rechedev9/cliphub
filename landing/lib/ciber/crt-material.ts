import { ShaderMaterial, Texture, Vector2 } from "three";

export type ScreenLayout = "short" | "fill";

export interface CrtOptions {
  map: Texture;
  layout: ScreenLayout;
  /** Width over height of the screen mesh. */
  aspect: number;
  /** Width over height of the video. */
  videoAspect: number;
  boost: number;
}

const vertexShader = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

const fragmentShader = /* glsl */ `
uniform sampler2D map;
uniform float time;
uniform float boost;
uniform float hover;
uniform float noise;
uniform float mode;
uniform vec2 aspects;
varying vec2 vUv;

float hash(vec2 p) {
  p = fract(p * vec2(123.34, 456.21));
  p += dot(p, p + 45.32);
  return fract(p.x * p.y);
}

vec3 video(vec2 uv) {
  return texture2D(map, clamp(uv, 0.002, 0.998)).rgb;
}

vec3 blurred(vec2 uv) {
  vec3 c = vec3(0.0);
  for (int x = -1; x <= 1; x++) {
    for (int y = -1; y <= 1; y++) {
      c += video(uv + vec2(float(x), float(y)) * 0.045);
    }
  }
  return c / 9.0;
}

void main() {
  vec2 uv = vUv;
  vec3 color;
  if (mode < 0.5) {
    // A vertical Short centred on a 4:3 tube, with a dark blurred copy behind it.
    float w = aspects.y / aspects.x;
    float u = (uv.x - (0.5 - w * 0.5)) / w;
    vec3 back = blurred(vec2(0.5 + (uv.x - 0.5) * 0.35, 0.5 + (uv.y - 0.5) * 0.35)) * 0.16;
    float inside = step(0.0, u) * step(u, 1.0);
    float edge = inside * (1.0 - smoothstep(0.0, 0.012, min(u, 1.0 - u)));
    color = mix(back, video(vec2(u, uv.y)), inside);
    color = mix(color, vec3(1.0, 0.45, 0.12), edge * 0.85);
  } else {
    // Fill the tube: crop the video to the screen aspect.
    vec2 crop = vec2(min(1.0, aspects.x / aspects.y), min(1.0, aspects.y / aspects.x));
    color = video(0.5 + (uv - 0.5) * crop);
  }

  float lines = vUv.y * 260.0;
  float fw = fwidth(lines);
  float scan = mix(0.72 + 0.28 * pow(abs(sin(lines * 3.14159)), 0.6), 1.0, smoothstep(0.35, 0.9, fw));
  float mask = 0.94 + 0.06 * sin(gl_FragCoord.x * 2.1);
  vec2 d = vUv - 0.5;
  float vig = smoothstep(0.78, 0.18, length(d * vec2(1.0, 1.15)));
  float flicker = 0.985 + 0.015 * sin(time * 47.0);
  color *= scan * mask * vig * flicker;

  float n = hash(vUv * 480.0 + fract(time * 13.0));
  color = mix(color, vec3(n * 0.9), noise);

  vec3 glass = vec3(0.05, 0.06, 0.06) * smoothstep(0.9, 0.0, length(d - vec2(-0.25, 0.3)));
  gl_FragColor = vec4(color * boost * (1.0 + hover * 0.18) + glass, 1.0);
}
`;

export function createCrtMaterial(options: CrtOptions): ShaderMaterial {
  return new ShaderMaterial({
    uniforms: {
      map: { value: options.map },
      time: { value: 0 },
      boost: { value: options.boost },
      hover: { value: 0 },
      noise: { value: 0 },
      mode: { value: options.layout === "short" ? 0 : 1 },
      aspects: { value: new Vector2(options.aspect, options.videoAspect) },
    },
    vertexShader,
    fragmentShader,
    toneMapped: false,
  });
}
