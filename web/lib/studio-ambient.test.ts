import { strict as assert } from 'node:assert';
import test from 'node:test';
import {
  AMBIENT_FRAME_MS,
  AMBIENT_RENDER_SCALE,
  ambientBufferSize,
  ambientMode,
  ambientNoise,
  type AmbientSignals,
} from './studio-ambient.ts';

const IDLE: AmbientSignals = {
  reducedMotion: false,
  efficiency: false,
  captureActive: false,
  windowActive: true,
};

test('the field animates only when every degradation signal is clear', () => {
  assert.equal(ambientMode(IDLE), 'animated');
});

test('each degradation signal on its own freezes the field', () => {
  assert.equal(ambientMode({ ...IDLE, reducedMotion: true }), 'static');
  assert.equal(ambientMode({ ...IDLE, efficiency: true }), 'static');
  assert.equal(ambientMode({ ...IDLE, captureActive: true }), 'static');
  assert.equal(ambientMode({ ...IDLE, windowActive: false }), 'static');
});

test('a capture freezes the field even while Studio is focused and unthrottled', () => {
  // A capture shares the GPU with cs2.exe, so it outranks "the user is here".
  assert.equal(ambientMode({ ...IDLE, captureActive: true, windowActive: true }), 'static');
});

test('the drawing buffer is half the CSS box', () => {
  const size = ambientBufferSize(1920, 1080, 1);
  assert.deepEqual(size, { width: 1920 * AMBIENT_RENDER_SCALE, height: 1080 * AMBIENT_RENDER_SCALE });
});

test('device pixel ratio is clamped to 1 so a scaled 4K panel costs the same', () => {
  assert.deepEqual(ambientBufferSize(1920, 1080, 2), ambientBufferSize(1920, 1080, 1));
  assert.deepEqual(ambientBufferSize(1920, 1080, 1.5), ambientBufferSize(1920, 1080, 1));
});

test('a collapsed CSS box still yields a drawable buffer', () => {
  assert.deepEqual(ambientBufferSize(0, 0, 1), { width: 1, height: 1 });
});

test('the frame cap is 30fps', () => {
  assert.ok(Math.abs(AMBIENT_FRAME_MS - 1000 / 30) < 1e-9);
});

test('the noise texture is one deterministic byte per texel', () => {
  const first = ambientNoise(64);
  const second = ambientNoise(64);
  assert.equal(first.length, 64 * 64);
  assert.deepEqual(Array.from(first), Array.from(second));
  assert.ok(first.every((value) => value >= 0 && value <= 255));
});

test('the noise texture tiles without a seam', () => {
  // The lattice wraps, so column 0 continues column size-1 rather than jumping.
  const size = 128;
  const texels = ambientNoise(size);
  let maxSeamDelta = 0;
  for (let y = 0; y < size; y += 1) {
    const left = texels[y * size] ?? 0;
    const right = texels[y * size + size - 1] ?? 0;
    maxSeamDelta = Math.max(maxSeamDelta, Math.abs(left - right));
  }
  // One lattice step across a 128px texture is 4px, so neighbouring texels can
  // legitimately differ; a hard seam would show up as a full-range jump.
  assert.ok(maxSeamDelta < 64, `seam delta ${maxSeamDelta} is a visible edge`);
});

test('the noise texture actually varies', () => {
  const texels = ambientNoise(64);
  const distinct = new Set(texels);
  assert.ok(distinct.size > 32, `only ${distinct.size} distinct values`);
});
