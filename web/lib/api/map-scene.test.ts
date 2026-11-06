import assert from 'node:assert/strict';
import test from 'node:test';
import { MAP_SCENE_VERSION, parseMapScene } from './map-scene.ts';

type Blob = { offset: number; length: number };

/** A scene file with one triangle, one colour texture and a sky cube, as scenefile.go lays it out. */
function sceneFile(edit: (header: Record<string, unknown>) => void = () => {}, version = MAP_SCENE_VERSION): ArrayBuffer {
  const parts: Uint8Array[] = [];
  let size = 0;
  const add = (bytes: Uint8Array): Blob => {
    const offset = Math.ceil(size / 4) * 4;
    parts.push(new Uint8Array(offset - size), bytes);
    size = offset + bytes.length;
    return { offset, length: bytes.length };
  };
  const texture = add(new Uint8Array(8));
  const sky = add(new Uint8Array(96));
  const position = add(new Uint8Array(new Uint16Array([0, 0, 0, 65535, 0, 0, 0, 65535, 0]).buffer));
  const normal = add(new Uint8Array(9));
  const uv = add(new Uint8Array(12));
  const index = add(new Uint8Array(new Uint16Array([0, 1, 2]).buffer));
  const lut = add(new Uint8Array(2 ** 3 * 4).fill(128));
  const header: Record<string, unknown> = {
    map: 'de_test',
    build: '1',
    min: [0, 0, 0],
    max: [10, 10, 0],
    sun: null,
    fog: null,
    lightmap: null,
    sky: { texture: 1, brightness: 1, tint: [1, 1, 1], yaw: 0, average: [0.5, 0.6, 0.8] },
    post: { exposureMin: 0.8, exposureMax: 1, tonemap: { exposureBias: 0, shoulder: 0, linearStrength: 0.001, linearAngle: 0.001, toeStrength: 1, toeNum: 1, toeDenom: 1, whitePoint: 1.5 }, lutSize: 2, lut },
    textures: [
      { format: 'bc1', width: 4, height: 4, faces: 1, levels: [8], ...texture },
      { format: 'bc6h', width: 4, height: 4, faces: 6, levels: [96], ...sky },
    ],
    materials: [
      {
        name: 'm',
        shader: 'csgo_complex.vfx',
        color: 0,
        color2: -1,
        blend: -1,
        tint: [1, 1, 1],
        tint2: [1, 1, 1],
        uvScale: [1, 1],
        uvOffset: [0, 0],
        uv2Scale: [1, 1],
        uv2Offset: [0, 0],
        blendSoftness: 0.5,
        blendMode: 0,
        alphaTest: false,
        alphaRef: 0.5,
        translucent: false,
        doubleSided: false,
        unlit: false,
        overlay: false,
        average: [0.5, 0.5, 0.5],
      },
    ],
    chunks: [{ material: 0, vertices: 3, indices: 3, min: [0, 0, 0], max: [10, 10, 0], position, normal, uv, index }],
  };
  edit(header);
  const json = new TextEncoder().encode(JSON.stringify(header));
  const blobStart = Math.ceil((12 + json.length) / 16) * 16;
  const out = new Uint8Array(blobStart + size);
  out.set(new TextEncoder().encode('CHMS'));
  const view = new DataView(out.buffer);
  view.setUint32(4, version, true);
  view.setUint32(8, json.length, true);
  out.set(json, 12);
  let at = blobStart;
  for (const part of parts) {
    out.set(part, at);
    at += part.length;
  }
  return out.buffer;
}

test('parseMapScene reads the chunks, textures and sky of a scene file', () => {
  const scene = parseMapScene(sceneFile());
  assert.equal(scene.map, 'de_test');
  assert.equal(scene.chunks.length, 1);
  assert.deepEqual([...scene.chunks[0].index], [0, 1, 2]);
  assert.equal(scene.chunks[0].position[3], 65535);
  assert.equal(scene.textures[1].faces, 6);
  assert.equal(scene.sky?.texture, 1);
});

test('parseMapScene refuses another version and arrays outside the file', () => {
  assert.throws(() => parseMapScene(sceneFile(() => {}, MAP_SCENE_VERSION - 1)), /version/);
  assert.throws(
    () =>
      parseMapScene(
        sceneFile((h) => {
          h.chunks = [{ material: 0, vertices: 3, min: [0, 0, 0], max: [1, 1, 1], position: { offset: 1 << 20, length: 18 }, normal: { offset: 0, length: 9 }, uv: { offset: 0, length: 12 }, index: { offset: 0, length: 6 } }];
        }),
      ),
    /outside/,
  );
});

test('parseMapScene refuses a sky that is not a cube', () => {
  assert.throws(
    () =>
      parseMapScene(
        sceneFile((h) => {
          h.sky = { texture: 0, brightness: 1, tint: [1, 1, 1], yaw: 0, average: [1, 1, 1] };
        }),
      ),
    /cube/,
  );
});

test('parseMapScene reads the post processing and its colour LUT', () => {
  const scene = parseMapScene(sceneFile());
  assert.equal(scene.post?.exposureMin, 0.8);
  assert.equal(scene.post?.tonemap.whitePoint, 1.5);
  assert.equal(scene.post?.lutSize, 2);
  assert.equal(scene.post?.lut?.length, 32);
  assert.equal(scene.post?.lut?.[5], 128);
});

test('parseMapScene refuses a colour LUT that does not match its size', () => {
  assert.throws(
    () =>
      parseMapScene(
        sceneFile((h) => {
          h.post = { exposureMin: 1, exposureMax: 1, tonemap: {}, lutSize: 3, lut: { offset: 0, length: 32 } };
        }),
      ),
    /LUT/,
  );
});

test('parseMapScene reads both fog layers and leaves a missing one out', () => {
  const scene = parseMapScene(
    sceneFile((h) => {
      h.fog = {
        gradient: null,
        cube: { distance: { start: 128, end: 7000, exponent: 1.1 }, height: { start: 2413, end: -3327, exponent: 1.5 }, max: 0.9, lodBias: 1 },
      };
    }),
  );
  assert.equal(scene.fog?.gradient, null);
  assert.equal(scene.fog?.cube?.distance.end, 7000);
  assert.equal(scene.fog?.cube?.height.exponent, 1.5);
  assert.equal(scene.fog?.cube?.lodBias, 1);
});
