import { MapGeometryError } from './map-geometry.ts';

/**
 * The map's render scene (`internal/mapgeo/scenefile.go`): "CHMS", a uint32 version, a uint32
 * header length, the JSON header, zero padding to 16 bytes, then the binary arrays the header
 * points into by byte offset from the start of that section.
 */
export const MAP_SCENE_VERSION = 7;

export type SceneTextureFormat = 'bc1' | 'bc3' | 'bc4' | 'bc5' | 'bc6h' | 'bc7' | 'rgba8' | 'bgra8';

const TEXTURE_FORMATS: readonly SceneTextureFormat[] = ['bc1', 'bc3', 'bc4', 'bc5', 'bc6h', 'bc7', 'rgba8', 'bgra8'];

/** `levels` is the mip chain, largest first: each level halves the last, down to 1 x 1. A cube's level holds its 6 faces, +X -X +Y -Y +Z -Z. */
export type SceneTexture = { format: SceneTextureFormat; width: number; height: number; faces: 1 | 6; levels: Uint8Array[] };

export type SceneMaterial = {
  name: string;
  shader: string;
  /** Texture indices; -1 for none. */
  color: number;
  color2: number;
  blend: number;
  tint: [number, number, number];
  tint2: [number, number, number];
  uvScale: [number, number];
  uvOffset: [number, number];
  uv2Scale: [number, number];
  uv2Offset: [number, number];
  blendSoftness: number;
  /** F_FANCY_BLENDING: 0 vertex blend only, 1 softness from the modulation red, 2 and 3 a fixed softness. */
  blendMode: number;
  alphaTest: boolean;
  alphaRef: number;
  translucent: boolean;
  doubleSided: boolean;
  unlit: boolean;
  overlay: boolean;
  /** The albedo's average sRGB colour, for a draw without its texture. */
  average: [number, number, number];
};

/** One chunk of at most 65535 vertices of one material; positions are normalised to [min, max]. */
export type SceneChunk = {
  material: number;
  vertices: number;
  min: [number, number, number];
  max: [number, number, number];
  position: Uint16Array;
  normal: Int8Array;
  /** Float16 bits. */
  uv: Uint16Array;
  uv2: Uint16Array | null;
  color: Uint8Array | null;
  blend: Uint8Array | null;
  index: Uint16Array;
};

export type SceneSun = { direction: [number, number, number]; color: [number, number, number]; sky: [number, number, number]; shadowChannel: number };
export type SceneFog = { start: number; end: number; color: [number, number, number]; max: number };
/** The sky cube, its brightness and linear tint, and its turn about z in degrees. */
export type SceneSky = { texture: number; brightness: number; tint: [number, number, number]; yaw: number; average: [number, number, number] };
/** `average` is the irradiance's average linear colour: the ambient of surfaces without a lightmap. */
export type SceneLightmap = { irradiance: number; directional: number; shadows: number; uvScale: [number, number]; average: [number, number, number] };

export type MapScene = {
  map: string;
  build: string;
  min: [number, number, number];
  max: [number, number, number];
  sun: SceneSun | null;
  fog: SceneFog | null;
  lightmap: SceneLightmap | null;
  sky: SceneSky | null;
  textures: SceneTexture[];
  materials: SceneMaterial[];
  chunks: SceneChunk[];
};

type Range = { offset: number; length: number };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function num(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error('scene: number expected');
  return value;
}

function int(value: unknown): number {
  const n = num(value);
  if (!Number.isInteger(n)) throw new Error('scene: integer expected');
  return n;
}

function str(value: unknown): string {
  if (typeof value !== 'string') throw new Error('scene: string expected');
  return value;
}

function bool(value: unknown): boolean {
  if (typeof value !== 'boolean') throw new Error('scene: boolean expected');
  return value;
}

function vec2(value: unknown): [number, number] {
  if (!Array.isArray(value) || value.length !== 2) throw new Error('scene: vec2 expected');
  return [num(value[0]), num(value[1])];
}

function vec3(value: unknown): [number, number, number] {
  if (!Array.isArray(value) || value.length !== 3) throw new Error('scene: vec3 expected');
  return [num(value[0]), num(value[1]), num(value[2])];
}

function record(value: unknown): Record<string, unknown> {
  if (!isRecord(value)) throw new Error('scene: object expected');
  return value;
}

function list(value: unknown): unknown[] {
  if (!Array.isArray(value)) throw new Error('scene: array expected');
  return value;
}

function range(value: unknown, blob: number): Range {
  const r = record(value);
  const offset = int(r.offset);
  const length = int(r.length);
  if (offset < 0 || length < 0 || offset + length > blob || offset % 4 !== 0) throw new Error('scene: array outside the file');
  return { offset, length };
}

function textureFormat(value: unknown): SceneTextureFormat {
  const found = TEXTURE_FORMATS.find((format) => format === value);
  if (found === undefined) throw new Error('scene: unknown texture format');
  return found;
}

function textureIndex(value: unknown, count: number): number {
  const i = int(value);
  if (i < -1 || i >= count) throw new Error('scene: texture index out of range');
  return i;
}

function parseMaterial(raw: unknown, textures: number): SceneMaterial {
  const m = record(raw);
  return {
    name: str(m.name),
    shader: str(m.shader),
    color: textureIndex(m.color, textures),
    color2: textureIndex(m.color2, textures),
    blend: textureIndex(m.blend, textures),
    tint: vec3(m.tint),
    tint2: vec3(m.tint2),
    uvScale: vec2(m.uvScale),
    uvOffset: vec2(m.uvOffset),
    uv2Scale: vec2(m.uv2Scale),
    uv2Offset: vec2(m.uv2Offset),
    blendSoftness: num(m.blendSoftness),
    blendMode: int(m.blendMode),
    alphaTest: bool(m.alphaTest),
    alphaRef: num(m.alphaRef),
    translucent: bool(m.translucent),
    doubleSided: bool(m.doubleSided),
    unlit: bool(m.unlit),
    overlay: bool(m.overlay),
    average: vec3(m.average),
  };
}

function sun(raw: unknown): SceneSun | null {
  if (raw === null || raw === undefined) return null;
  const s = record(raw);
  return { direction: vec3(s.direction), color: vec3(s.color), sky: vec3(s.sky), shadowChannel: int(s.shadowChannel) };
}

function fog(raw: unknown): SceneFog | null {
  if (raw === null || raw === undefined) return null;
  const f = record(raw);
  return { start: num(f.start), end: num(f.end), color: vec3(f.color), max: num(f.max) };
}

function sky(raw: unknown, textures: SceneTexture[]): SceneSky | null {
  if (raw === null || raw === undefined) return null;
  const s = record(raw);
  const texture = textureIndex(s.texture, textures.length);
  if (texture < 0 || textures[texture]?.faces !== 6) throw new Error('scene: the sky is not a cube');
  return { texture, brightness: num(s.brightness), tint: vec3(s.tint), yaw: num(s.yaw), average: vec3(s.average) };
}

function faces(value: unknown): 1 | 6 {
  if (value === undefined || value === 1) return 1;
  if (value === 6) return 6;
  throw new Error('scene: a texture has 1 or 6 faces');
}

function lightmap(raw: unknown, textures: number): SceneLightmap | null {
  if (raw === null || raw === undefined) return null;
  const l = record(raw);
  return {
    irradiance: textureIndex(l.irradiance, textures),
    directional: textureIndex(l.directional, textures),
    shadows: textureIndex(l.shadows, textures),
    uvScale: vec2(l.uvScale),
    average: vec3(l.average),
  };
}

/** Decodes a scene file; throws when any part is malformed or points outside the file. */
export function parseMapScene(buffer: ArrayBuffer): MapScene {
  const view = new DataView(buffer);
  if (buffer.byteLength < 12 || new TextDecoder().decode(new Uint8Array(buffer, 0, 4)) !== 'CHMS') throw new Error('scene: not a scene file');
  const version = view.getUint32(4, true);
  if (version !== MAP_SCENE_VERSION) throw new Error(`scene: version ${version}`);
  const headerLength = view.getUint32(8, true);
  if (12 + headerLength > buffer.byteLength) throw new Error('scene: header outside the file');
  const header: unknown = JSON.parse(new TextDecoder().decode(new Uint8Array(buffer, 12, headerLength)));
  const blobStart = Math.ceil((12 + headerLength) / 16) * 16;
  const blob = buffer.byteLength - blobStart;
  const h = record(header);
  const bytes = (r: Range): Uint8Array => new Uint8Array(buffer, blobStart + r.offset, r.length);
  const u16 = (r: Range): Uint16Array => new Uint16Array(buffer, blobStart + r.offset, r.length / 2);
  const textures = list(h.textures).map((raw): SceneTexture => {
    const t = record(raw);
    const r = range(t, blob);
    const levels: Uint8Array[] = [];
    let at = r.offset;
    for (const length of list(t.levels)) {
      const n = int(length);
      if (n <= 0 || at + n > r.offset + r.length) throw new Error('scene: texture level outside its data');
      levels.push(bytes({ offset: at, length: n }));
      at += n;
    }
    if (levels.length === 0) throw new Error('scene: texture without levels');
    return { format: textureFormat(t.format), width: int(t.width), height: int(t.height), faces: faces(t.faces), levels };
  });
  const materials = list(h.materials).map((raw) => parseMaterial(raw, textures.length));
  const chunks = list(h.chunks).map((raw): SceneChunk => {
    const c = record(raw);
    const material = int(c.material);
    const vertices = int(c.vertices);
    if (material < 0 || material >= materials.length || vertices <= 0 || vertices > 65535) throw new Error('scene: bad chunk');
    const position = range(c.position, blob);
    const normal = range(c.normal, blob);
    const uv = range(c.uv, blob);
    const index = range(c.index, blob);
    if (position.length !== vertices * 6 || normal.length !== vertices * 3 || uv.length !== vertices * 4 || index.length % 6 !== 0) {
      throw new Error('scene: chunk arrays do not match its vertices');
    }
    const optional = (key: string, size: number): Range | null => {
      if (c[key] === undefined || c[key] === null) return null;
      const r = range(c[key], blob);
      if (r.length !== vertices * size) throw new Error(`scene: chunk ${key} does not match its vertices`);
      return r;
    };
    const uv2 = optional('uv2', 4);
    const color = optional('color', 4);
    const blend = optional('blend', 4);
    const indices = u16(index);
    for (const i of indices) if (i >= vertices) throw new Error('scene: index outside the chunk');
    return {
      material,
      vertices,
      min: vec3(c.min),
      max: vec3(c.max),
      position: u16(position),
      normal: new Int8Array(buffer, blobStart + normal.offset, normal.length),
      uv: u16(uv),
      uv2: uv2 === null ? null : u16(uv2),
      color: color === null ? null : bytes(color),
      blend: blend === null ? null : bytes(blend),
      index: indices,
    };
  });
  return {
    map: str(h.map),
    build: str(h.build),
    min: vec3(h.min),
    max: vec3(h.max),
    sun: sun(h.sun),
    fog: fog(h.fog),
    lightmap: lightmap(h.lightmap, textures.length),
    sky: sky(h.sky, textures),
    textures,
    materials,
    chunks,
  };
}

/** Fetches the map's scene from the local CS2 install; `onProgress` gets the share read, 0 to 1. */
export async function getMapScene(map: string, onProgress?: (share: number) => void): Promise<MapScene> {
  const res = await fetch(`/api/maps/${encodeURIComponent(map)}/scene`, { cache: 'no-store' });
  if (!res.ok) {
    let code: string | undefined;
    try {
      const body: unknown = await res.json();
      if (isRecord(body) && typeof body.code === 'string') code = body.code;
    } catch {
      code = undefined;
    }
    throw new MapGeometryError(`scene ${res.status}`, res.status, code);
  }
  const total = Number(res.headers.get('content-length') ?? '0');
  if (res.body === null || !(total > 0)) return parseMapScene(await res.arrayBuffer());
  const out = new Uint8Array(total);
  const reader = res.body.getReader();
  let at = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (at + value.length > total) throw new Error('scene: longer than announced');
    out.set(value, at);
    at += value.length;
    onProgress?.(at / total);
  }
  return parseMapScene(out.buffer);
}
