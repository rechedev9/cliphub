import {
  BoxGeometry,
  Clock,
  Color,
  FogExp2,
  HalfFloatType,
  LoadingManager,
  MathUtils,
  Mesh,
  MeshBasicMaterial,
  MeshStandardMaterial,
  Object3D,
  PerspectiveCamera,
  Raycaster,
  SRGBColorSpace,
  Scene,
  ShaderMaterial,
  Texture,
  TextureLoader,
  Vector2,
  Vector3,
  VideoTexture,
  WebGLRenderTarget,
  WebGLRenderer,
} from "three";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { MeshoptDecoder } from "three/examples/jsm/libs/meshopt_decoder.module.js";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { OutputPass } from "three/examples/jsm/postprocessing/OutputPass.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { ShaderPass } from "three/examples/jsm/postprocessing/ShaderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { createCrtMaterial } from "./crt-material";
import { STATIONS, stationById } from "./stations";
import type { Shot, Station, StationId, Vec3 } from "./stations";

export type Quality = "high" | "low";

export interface AnchorPoint {
  x: number;
  y: number;
  visible: boolean;
}

export interface CiberSceneOptions {
  canvas: HTMLCanvasElement;
  quality: Quality;
  reducedMotion: boolean;
  posterVariant: number;
  onProgress: (ratio: number) => void;
  onHover: (id: StationId | null) => void;
  onSelect: (id: StationId) => void;
  onAnchors: (points: ReadonlyMap<StationId, AnchorPoint>) => void;
  onArrive: (id: StationId) => void;
  onError: (error: unknown) => void;
  /** Called once if the first seconds after ready run under 20 fps; null when the visitor asked for 3D. */
  onSlow: (() => void) | null;
}

/** Screen space, in CSS px, that the page covers on each side of the free region. */
export interface Insets {
  top: number;
  right: number;
  bottom: number;
}

// public/video/<file>.webp is the frame at `still`. A video that starts at its still swaps in seamlessly;
// the CRT play starts from the beginning, so its tube tunes in through static.
const FEEDS = {
  jugada: { file: "crt-jugada", still: 80.5, start: 0, loop: false },
  kill: { file: "killfeed-loop", still: 2.2, start: 2.2, loop: true },
  hero: { file: "hero-loop", still: 1.4, start: 1.4, loop: true },
} as const;
type Feed = keyof typeof FEEDS;
const FEED_IDS = ["jugada", "kill", "hero"] as const satisfies readonly Feed[];

interface Channel {
  feed: Feed;
  aspect: number;
  layout: number;
}

export interface CiberScene {
  ready: Promise<void>;
  goTo: (id: StationId, instant: boolean) => void;
  setInsets: (insets: Insets) => void;
  setVideosPaused: (paused: boolean) => void;
  dispose: () => void;
}

interface CameraState {
  position: Vector3;
  target: Vector3;
  fov: number;
}

const ATLASES = ["room", "desk", "desktop", "monitor", "far", "tower", "cd", "keyboard", "props", "paper"] as const;
const SCREEN_ASPECT = 0.318 / 0.24;
const SHORT_ASPECT = 540 / 960;
const UP = new Vector3(0, 1, 0);

function toThree(v: Vec3): Vector3 {
  return new Vector3(v[0], v[2], -v[1]);
}

function smoothstep(x: number, edges: { from: number; to: number }): number {
  const t = MathUtils.clamp((x - edges.from) / (edges.to - edges.from), 0, 1);
  return t * t * (3 - 2 * t);
}

function easeInOutCubic(t: number): number {
  return t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2;
}

function verticalFov(shot: Shot, aspect: number): number {
  const half = MathUtils.degToRad(shot.fov / 2);
  return MathUtils.radToDeg(2 * Math.atan(Math.tan(half) / aspect));
}

/** Frame a shot the way its still looks with object-fit: cover in a region of this aspect. */
function coverFrame(shot: Shot, aspect: number): CameraState {
  return {
    position: toThree(shot.position),
    target: toThree(shot.target),
    fov: verticalFov(shot, Math.max(aspect, shot.still ?? aspect)),
  };
}

/** Blend the portrait and landscape shots of a station for the visible region. */
function frameFor(station: Station, aspect: number): CameraState {
  const shot = aspect < 1 ? station.tall : station.wide;
  if (shot.still) return coverFrame(shot, aspect);
  const t = smoothstep(aspect, { from: 0.72, to: 1.3 });
  const position = toThree(station.tall.position).lerp(toThree(station.wide.position), t);
  const target = toThree(station.tall.target).lerp(toThree(station.wide.target), t);
  const fov = MathUtils.lerp(verticalFov(station.tall, aspect), verticalFov(station.wide, aspect), t);
  return { position, target, fov: MathUtils.clamp(fov, 18, 78) };
}

function pickVideoSource(name: string): string {
  const probe = document.createElement("video");
  return probe.canPlayType('video/webm; codecs="vp9"') ? `/video/${name}.webm` : `/video/${name}.mp4`;
}

function makeVideo(feed: Feed): HTMLVideoElement {
  const video = document.createElement("video");
  video.muted = true;
  video.loop = FEEDS[feed].loop;
  video.playsInline = true;
  video.preload = "none";
  return video;
}

function onFirstFrame(video: HTMLVideoElement, done: () => void) {
  if (typeof video.requestVideoFrameCallback === "function") video.requestVideoFrameCallback(() => done());
  else video.addEventListener("playing", done, { once: true });
}

function videoTexture(video: HTMLVideoElement): VideoTexture {
  const tex = new VideoTexture(video);
  tex.colorSpace = SRGBColorSpace;
  tex.flipY = false;
  return tex;
}

const GrainShader = {
  uniforms: {
    tDiffuse: { value: null },
    time: { value: 0 },
    amount: { value: 0.045 },
    vignette: { value: 0.32 },
  },
  vertexShader: /* glsl */ `
    varying vec2 vUv;
    void main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }
  `,
  fragmentShader: /* glsl */ `
    uniform sampler2D tDiffuse;
    uniform float time;
    uniform float amount;
    uniform float vignette;
    varying vec2 vUv;
    float hash(vec2 p) { p = fract(p * vec2(443.897, 441.423)); p += dot(p, p.yx + 19.19); return fract((p.x + p.y) * p.x); }
    void main() {
      vec4 c = texture2D(tDiffuse, vUv);
      float d = length((vUv - 0.5) * vec2(1.0, 0.85));
      c.rgb *= mix(1.0, smoothstep(0.92, 0.22, d), vignette);
      float g = hash(gl_FragCoord.xy + fract(time) * 97.0) - 0.5;
      c.rgb += g * amount * (0.35 + 0.65 * (1.0 - c.rgb));
      gl_FragColor = c;
    }
  `,
};

export function createCiberScene(options: CiberSceneOptions): CiberScene {
  const { canvas, quality, reducedMotion } = options;
  const renderer = new WebGLRenderer({ canvas, antialias: false, powerPreference: "high-performance" });
  renderer.outputColorSpace = SRGBColorSpace;
  const pixelRatio = Math.min(window.devicePixelRatio || 1, quality === "high" ? 1.75 : 1.5);
  renderer.setPixelRatio(pixelRatio);

  const scene = new Scene();
  scene.background = new Color(0x020304);
  scene.fog = new FogExp2(0x0b1013, 0.085);
  const camera = new PerspectiveCamera(45, 1, 0.02, 30);

  const target = new WebGLRenderTarget(1, 1, { type: HalfFloatType, samples: quality === "high" ? 4 : 0 });
  const composer = new EffectComposer(renderer, target);
  composer.addPass(new RenderPass(scene, camera));
  // Baked surfaces stay at or under 1, so only the neon, the LEDs and the tubes bloom.
  const bloom = new UnrealBloomPass(new Vector2(1, 1), 0.5, 0.55, 1.0);
  composer.addPass(bloom);
  composer.addPass(new OutputPass());
  const grain = new ShaderPass(GrainShader);
  composer.addPass(grain);

  const manager = new LoadingManager();
  manager.onProgress = (_url, loaded, total) => options.onProgress(total ? loaded / total : 0);
  const textures = new TextureLoader(manager);
  const gltfLoader = new GLTFLoader(manager);
  gltfLoader.setMeshoptDecoder(MeshoptDecoder);
  const suffix = quality === "high" ? "" : "-m";
  const anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy());

  function loadTexture(url: string): Promise<Texture> {
    return textures.loadAsync(url).then((tex) => {
      tex.flipY = false;
      tex.colorSpace = SRGBColorSpace;
      tex.anisotropy = anisotropy;
      return tex;
    });
  }

  const videos = {
    jugada: makeVideo("jugada"),
    kill: makeVideo("kill"),
    hero: makeVideo("hero"),
  } satisfies Record<Feed, HTMLVideoElement>;
  const videoTex = {
    jugada: videoTexture(videos.jugada),
    kill: videoTexture(videos.kill),
    hero: videoTexture(videos.hero),
  } satisfies Record<Feed, VideoTexture>;
  // A tube shows its feed's still until that video draws a frame, so the videos wait for the scene.
  const stills = new Map<Feed, Texture>();
  const live = new Set<Feed>();
  let videosOn = !reducedMotion;
  let sceneShown = false;
  // layout 0 is a vertical Short on the tube, 1 fills the tube.
  const channels = {
    jugada: { feed: "jugada", aspect: 4 / 3, layout: 1 },
    stream: { feed: "hero", aspect: SHORT_ASPECT, layout: 0 },
    killFill: { feed: "kill", aspect: SHORT_ASPECT, layout: 1 },
    heroFill: { feed: "hero", aspect: SHORT_ASPECT, layout: 1 },
  } satisfies Record<string, Channel>;
  const startChannel: Record<string, Channel> = {
    screen_main: channels.jugada,
    screen_far0: channels.heroFill,
    screen_far1: channels.killFill,
    screen_far2: channels.stream,
  };
  const showing = new Map<string, Channel>();
  const screens = new Map<string, ShaderMaterial>();
  const highlight = new Map<string, MeshBasicMaterial | ShaderMaterial>();
  const hands: { hour?: Object3D; min?: Object3D; sec?: Object3D } = {};
  let neon: MeshBasicMaterial | null = null;
  const neonColor = new Color(1.0, 0.42, 0.12).multiplyScalar(3.2);

  const hotspots: { mesh: Mesh; id: StationId }[] = [];
  const hitMaterial = new MeshBasicMaterial({ visible: false });
  for (const station of STATIONS) {
    if (!station.hotspot) continue;
    const { center, size } = station.hotspot;
    const box = new Mesh(new BoxGeometry(size[0], size[2], size[1]), hitMaterial);
    box.position.copy(toThree(center));
    scene.add(box);
    hotspots.push({ mesh: box, id: station.id });
  }

  const loaded = (async () => {
    const posterName = `poster-${MathUtils.clamp(Math.round(options.posterVariant), 1, 3)}`;
    const [gltf, ...maps] = await Promise.all([
      gltfLoader.loadAsync("/scene/puesto07.glb"),
      ...ATLASES.map((a) => loadTexture(`/scene/tex/${a}${suffix}.webp`)),
      loadTexture(`/scene/tex/${posterName}${suffix}.webp`),
      ...FEED_IDS.map((feed) => loadTexture(`/video/${FEEDS[feed].file}.webp`)),
    ]);
    const atlas = new Map<string, Texture>();
    ATLASES.forEach((a, i) => atlas.set(a, maps[i]));
    atlas.set("poster", maps[ATLASES.length]);
    FEED_IDS.forEach((feed, i) => stills.set(feed, maps[ATLASES.length + 1 + i]));

    gltf.scene.traverse((node) => {
      if (node.name.startsWith("clock_hour")) hands.hour = node;
      if (node.name.startsWith("clock_min")) hands.min = node;
      if (node.name.startsWith("clock_sec")) hands.sec = node;
      if (!(node instanceof Mesh)) return;
      const name = node.name;
      const source = Array.isArray(node.material) ? node.material[0] : node.material;
      const matName = source?.name ?? "";
      if (name.startsWith("screen_")) {
        const channel = startChannel[name] ?? channels.heroFill;
        const mat = createCrtMaterial({
          map: feedTexture(channel.feed),
          layout: channel.layout === 0 ? "short" : "fill",
          aspect: SCREEN_ASPECT,
          videoAspect: channel.aspect,
          boost: name === "screen_main" ? 1.4 : 1.1,
        });
        node.material = mat;
        screens.set(name, mat);
        showing.set(name, channel);
        highlight.set(name, mat);
        return;
      }
      if (matName.startsWith("atlas_")) {
        const map = atlas.get(matName.slice(6));
        const mat = new MeshBasicMaterial({ map: map ?? null });
        node.material = mat;
        highlight.set(name, mat);
        return;
      }
      if (name.startsWith("neon")) {
        neon = new MeshBasicMaterial({ color: neonColor.clone() });
        node.material = neon;
        return;
      }
      if (source instanceof MeshStandardMaterial) {
        const glow = source.emissive.clone().multiplyScalar(MathUtils.clamp(source.emissiveIntensity, 1, 3) * 1.2);
        const base =
          name.startsWith("clock_sec") || name.startsWith("clock_pin")
            ? new Color(0.32, 0.02, 0.01)
            : source.color.clone().multiplyScalar(0.05);
        node.material = new MeshBasicMaterial({ color: glow.r + glow.g + glow.b > 0.001 ? glow : base });
      }
    });
    scene.add(gltf.scene);
  })();

  // Ready after two drawn frames: the first one compiles every shader, so the fade never shows an empty canvas.
  let framesDrawn = 0;
  let markDrawn: () => void = () => undefined;
  const ready = loaded.then(
    () =>
      new Promise<void>((resolve) => {
        markDrawn = resolve;
      }),
  );
  ready.then(
    () => {
      sceneShown = true;
      if (videosOn && !disposed) playVideos();
    },
    () => undefined,
  );

  function feedTexture(feed: Feed): Texture {
    return (live.has(feed) ? undefined : stills.get(feed)) ?? videoTex[feed];
  }

  function playVideos() {
    for (const feed of FEED_IDS) {
      const video = videos[feed];
      // The CRT play runs once and then holds its last frame.
      if (video.ended) continue;
      if (!video.getAttribute("src")) {
        video.src = pickVideoSource(FEEDS[feed].file);
        video.currentTime = FEEDS[feed].start;
        onFirstFrame(video, () => showLive(feed));
      }
      void video.play().catch(() => undefined);
    }
  }

  function showLive(feed: Feed) {
    live.add(feed);
    const seamless = FEEDS[feed].still === FEEDS[feed].start;
    for (const [name, channel] of showing) {
      const mat = screens.get(name);
      if (!mat || channel.feed !== feed) continue;
      if (seamless) mat.uniforms.map.value = videoTex[feed];
      else tune(name, channel);
    }
  }

  let width = 1;
  let height = 1;
  const insets: Insets = { top: 0, right: 0, bottom: 0 };
  const shownInsets: Insets = { top: 0, right: 0, bottom: 0 };
  let current: StationId = "inicio";
  let from: CameraState | null = null;
  let progress = 1;
  let duration = 1.4;
  const state: CameraState = { position: new Vector3(), target: new Vector3(), fov: 50 };
  const pointer = new Vector2(0, 0);
  const smoothPointer = new Vector2(0, 0);
  let pointerInside = false;
  let hovered: StationId | null = null;
  const hoverLevel = new Map<string, number>();
  let neonDip = 0;
  let nextFlicker = 3;
  let nextChannel = 9;
  const tuning = new Map<string, { level: number; next: Channel }>();
  let paused = false;
  let disposed = false;
  let raf = 0;
  const clock = new Clock();
  const raycaster = new Raycaster();
  const anchors = new Map<StationId, AnchorPoint>();
  // Frame times from 1 s to 3 s after ready, once the fade-in and the first video frames are past.
  let speedCheckFrom = Number.POSITIVE_INFINITY;
  const frameTimes: number[] = [];

  function liveFrame(): CameraState {
    const w = Math.max(1, width - shownInsets.right);
    const h = Math.max(1, height - shownInsets.top - shownInsets.bottom);
    const frame = frameFor(stationById(current), w / h);
    const half = MathUtils.degToRad(frame.fov / 2);
    frame.fov = MathUtils.radToDeg(2 * Math.atan((Math.tan(half) * height) / h));
    return frame;
  }

  function goTo(id: StationId, instant: boolean) {
    if (id === current && progress >= 1) return;
    from = { position: state.position.clone(), target: state.target.clone(), fov: state.fov };
    const dist = from.position.distanceTo(frameFor(stationById(id), width / height).position);
    current = id;
    duration = MathUtils.clamp(0.9 + dist * 0.55, 1.1, 2.1);
    progress = instant || reducedMotion ? 1 : 0;
  }

  function applyChannel(name: string, channel: Channel) {
    const mat = screens.get(name);
    if (!mat) return;
    showing.set(name, channel);
    mat.uniforms.map.value = feedTexture(channel.feed);
    mat.uniforms.mode.value = channel.layout;
    mat.uniforms.aspects.value.set(SCREEN_ASPECT, channel.aspect);
  }

  /** Change what a tube shows, through a burst of static unless motion is reduced. */
  function tune(name: string, next: Channel) {
    if (!screens.has(name)) {
      startChannel[name] = next;
      return;
    }
    if (reducedMotion) applyChannel(name, next);
    else tuning.set(name, { level: 1, next });
  }

  function resize() {
    const rect = canvas.getBoundingClientRect();
    width = Math.max(1, Math.round(rect.width));
    height = Math.max(1, Math.round(rect.height));
    renderer.setSize(width, height, false);
    composer.setPixelRatio(pixelRatio);
    composer.setSize(width, height);
  }

  function hitTest(): StationId | null {
    if (!pointerInside) return null;
    raycaster.setFromCamera(pointer, camera);
    const hits = raycaster.intersectObjects(
      hotspots.map((h) => h.mesh),
      false,
    );
    const first = hits[0];
    if (!first) return null;
    const hit = hotspots.find((h) => h.mesh === first.object);
    return hit && hit.id !== current ? hit.id : null;
  }

  function updateHover(dt: number) {
    const next = hitTest();
    if (next !== hovered) {
      hovered = next;
      canvas.style.cursor = next ? "pointer" : "";
      options.onHover(next);
    }
    for (const station of STATIONS) {
      if (!station.mesh) continue;
      const mat = highlight.get(station.mesh);
      if (!mat) continue;
      const goal = hovered === station.id ? 1 : 0;
      const level = MathUtils.damp(hoverLevel.get(station.mesh) ?? 0, goal, 12, dt);
      hoverLevel.set(station.mesh, level);
      if (mat instanceof ShaderMaterial) mat.uniforms.hover.value = level;
      else mat.color.setScalar(1 + level * 0.28);
    }
  }

  function updateCamera(dt: number) {
    // With reduced motion the framing jumps to a new panel size instead of gliding.
    for (const side of ["top", "right", "bottom"] as const) {
      shownInsets[side] = reducedMotion ? insets[side] : MathUtils.damp(shownInsets[side], insets[side], 6, dt);
    }
    const live = liveFrame();
    if (progress < 1 && from) {
      progress = Math.min(1, progress + dt / duration);
      const e = easeInOutCubic(progress);
      state.position.lerpVectors(from.position, live.position, e);
      state.target.lerpVectors(from.target, live.target, e);
      state.fov = MathUtils.lerp(from.fov, live.fov, e);
      const lift = Math.sin(Math.PI * e) * Math.min(0.1, from.position.distanceTo(live.position) * 0.07);
      state.position.y += lift;
      if (progress >= 1) options.onArrive(current);
    } else {
      state.position.copy(live.position);
      state.target.copy(live.target);
      state.fov = live.fov;
    }
    camera.position.copy(state.position);
    if (!reducedMotion) {
      smoothPointer.x = MathUtils.damp(smoothPointer.x, pointerInside ? pointer.x : 0, 3, dt);
      smoothPointer.y = MathUtils.damp(smoothPointer.y, pointerInside ? pointer.y : 0, 3, dt);
      const forward = state.target.clone().sub(state.position).normalize();
      const right = forward.clone().cross(UP).normalize();
      const scale = current === "inicio" ? 1 : 0.45;
      camera.position.addScaledVector(right, smoothPointer.x * 0.018 * scale);
      camera.position.addScaledVector(UP, smoothPointer.y * 0.012 * scale);
    }
    camera.lookAt(state.target);
    camera.fov = state.fov;
    camera.aspect = width / height;
    const shiftY = (shownInsets.bottom - shownInsets.top) / 2;
    camera.setViewOffset(width, height, shownInsets.right / 2, shiftY, width, height);
    camera.updateProjectionMatrix();
  }

  function updateWorld(dt: number, t: number) {
    const now = new Date();
    const s = now.getSeconds();
    const m = now.getMinutes() + s / 60;
    const h = (now.getHours() % 12) + m / 60;
    if (hands.sec) hands.sec.rotation.z = -(s / 60) * Math.PI * 2;
    if (hands.min) hands.min.rotation.z = -(m / 60) * Math.PI * 2;
    if (hands.hour) hands.hour.rotation.z = -(h / 12) * Math.PI * 2;
    for (const mat of screens.values()) mat.uniforms.time.value = reducedMotion ? 0 : t;
    grain.uniforms.time.value = reducedMotion ? 0 : t;
    if (reducedMotion) return;
    if (neon) {
      if (t > nextFlicker) {
        neonDip = 1;
        nextFlicker = t + 4 + Math.random() * 9;
      }
      neonDip = Math.max(0, neonDip - dt * 3.5);
      const buzz = neonDip > 0 && Math.sin(t * 90) > 0 ? 0.35 : 1;
      neon.color.copy(neonColor).multiplyScalar(neonDip > 0 ? buzz : 1);
    }
    if (t > nextChannel) {
      nextChannel = t + 11 + Math.random() * 8;
      const pick = Math.random() < 0.5 ? "screen_far0" : "screen_far1";
      const now = showing.get(pick)?.feed;
      const options = [channels.killFill, channels.heroFill].filter((c) => c.feed !== now);
      tune(pick, options[Math.floor(Math.random() * options.length)] ?? channels.heroFill);
    }
    for (const [name, tv] of tuning) {
      const mat = screens.get(name);
      if (!mat) continue;
      const level = tv.level - dt * 2.6;
      if (tv.level > 0.5 && level <= 0.5) applyChannel(name, tv.next);
      mat.uniforms.noise.value = Math.max(0, Math.sin(Math.max(0, level) * Math.PI)) * 0.85;
      if (level <= 0) tuning.delete(name);
      else tuning.set(name, { level, next: tv.next });
    }
  }

  function updateAnchors() {
    const atDesk = current === "inicio" && progress >= 1;
    for (const station of STATIONS) {
      if (!station.anchor) continue;
      const p = toThree(station.anchor).project(camera);
      anchors.set(station.id, {
        x: ((p.x + 1) / 2) * width,
        y: ((1 - p.y) / 2) * height,
        visible: atDesk && p.z < 1 && Math.abs(p.x) < 1 && Math.abs(p.y) < 1,
      });
    }
    options.onAnchors(anchors);
  }

  function checkSpeed(dt: number, t: number) {
    if (t < speedCheckFrom) return;
    if (t < speedCheckFrom + 2) {
      frameTimes.push(dt);
      return;
    }
    speedCheckFrom = Number.POSITIVE_INFINITY;
    frameTimes.sort((a, b) => a - b);
    const median = frameTimes[Math.floor(frameTimes.length / 2)] ?? 0;
    if (median > 1 / 20) options.onSlow?.();
  }

  function tick() {
    if (disposed) return;
    raf = requestAnimationFrame(tick);
    if (paused) return;
    const dt = Math.min(clock.getDelta(), 0.1);
    const t = clock.elapsedTime;
    updateCamera(dt);
    updateHover(dt);
    updateWorld(dt, t);
    composer.render(dt);
    updateAnchors();
    checkSpeed(dt, t);
    framesDrawn += 1;
    if (framesDrawn !== 2) return;
    markDrawn();
    if (options.onSlow) speedCheckFrom = t + 1;
  }

  function onPointerMove(e: PointerEvent) {
    const rect = canvas.getBoundingClientRect();
    pointer.set(((e.clientX - rect.left) / rect.width) * 2 - 1, -((e.clientY - rect.top) / rect.height) * 2 + 1);
    pointerInside = e.pointerType === "mouse";
  }
  let down: { x: number; y: number } | null = null;
  function onPointerDown(e: PointerEvent) {
    down = { x: e.clientX, y: e.clientY };
    onPointerMove(e);
    pointerInside = true;
  }
  function onPointerUp(e: PointerEvent) {
    if (!down) return;
    const moved = Math.hypot(e.clientX - down.x, e.clientY - down.y);
    down = null;
    if (moved > 8) return;
    onPointerMove(e);
    pointerInside = true;
    const id = hitTest();
    if (e.pointerType !== "mouse") pointerInside = false;
    if (id) options.onSelect(id);
  }
  function onPointerLeave() {
    pointerInside = false;
  }
  function onVisibility() {
    paused = document.hidden;
    clock.getDelta();
  }
  function onContextLost(e: Event) {
    e.preventDefault();
    options.onError(new Error("webgl context lost"));
  }

  const observer = new ResizeObserver(resize);
  observer.observe(canvas);
  canvas.addEventListener("pointermove", onPointerMove);
  canvas.addEventListener("pointerdown", onPointerDown);
  canvas.addEventListener("pointerup", onPointerUp);
  canvas.addEventListener("pointerleave", onPointerLeave);
  canvas.addEventListener("webglcontextlost", onContextLost);
  document.addEventListener("visibilitychange", onVisibility);
  resize();
  Object.assign(state, liveFrame());
  loaded.then(
    () => tick(),
    (error: unknown) => options.onError(error),
  );

  return {
    ready,
    goTo,
    setInsets(next) {
      insets.top = next.top;
      insets.right = next.right;
      insets.bottom = next.bottom;
    },
    setVideosPaused(value) {
      videosOn = !value;
      if (!sceneShown) return;
      if (value) for (const v of Object.values(videos)) v.pause();
      else playVideos();
    },
    dispose() {
      disposed = true;
      cancelAnimationFrame(raf);
      observer.disconnect();
      canvas.removeEventListener("pointermove", onPointerMove);
      canvas.removeEventListener("pointerdown", onPointerDown);
      canvas.removeEventListener("pointerup", onPointerUp);
      canvas.removeEventListener("pointerleave", onPointerLeave);
      canvas.removeEventListener("webglcontextlost", onContextLost);
      document.removeEventListener("visibilitychange", onVisibility);
      for (const v of Object.values(videos)) {
        v.pause();
        v.removeAttribute("src");
        v.load();
      }
      scene.traverse((node) => {
        if (!(node instanceof Mesh)) return;
        node.geometry.dispose();
        const mats = Array.isArray(node.material) ? node.material : [node.material];
        for (const m of mats) {
          if (m instanceof MeshBasicMaterial) m.map?.dispose();
          m.dispose();
        }
      });
      for (const tex of [...Object.values(videoTex), ...stills.values()]) tex.dispose();
      composer.dispose();
      renderer.dispose();
    },
  };
}
