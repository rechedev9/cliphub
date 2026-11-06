import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../../api/tactical.ts';
import type { TacticalFrame } from '../../api/tactical.ts';
import { heightBand, mapPalette, srgbToLinear, surfaceColors, zRange } from './mesh.ts';
import {
  DUCKING_EYE,
  STANDING_EYE,
  defaultCut,
  eyeHeight,
  lookYaw,
  movementYaw,
  playBox,
  viewDirection,
} from './scene-math.ts';
import { sampleTrails3d } from './trails.ts';

const close = (actual: number, expected: number, epsilon = 1e-9): void => {
  assert.ok(Math.abs(actual - expected) <= epsilon, `${actual} is not ${expected}`);
};

const alive = TACTICAL_SAMPLE_FLAGS.alive;
const sample = (slot: number, x: number, z: number, flags = alive) => ({ slot, x, y: 0, z, yaw: 0, health: 100, flags });

test('view: yaw turns counter-clockwise from +X and positive pitch looks down', () => {
  const east = viewDirection(0);
  close(east.x, 1);
  close(east.z, 0);
  const north = viewDirection(90);
  close(north.x, 0);
  close(north.y, 1);
  const down = viewDirection(0, 90);
  close(down.z, -1);
});

test('view: without a recorded yaw the player looks where they move', () => {
  assert.equal(movementYaw({ x: 0, y: 0 }, { x: 0.2, y: 0 }), null);
  close(movementYaw({ x: 0, y: 0 }, { x: 0, y: -10 }) ?? 0, 270);
  close(lookYaw({ x: 0, y: 10, yaw: Number.NaN }, { x: 0, y: 0 }), 90);
  assert.equal(lookYaw({ x: 0, y: 0, yaw: 45 }), 45);
  assert.equal(eyeHeight({ flags: TACTICAL_SAMPLE_FLAGS.ducking }), DUCKING_EYE);
  assert.equal(eyeHeight({ flags: 0 }), STANDING_EYE);
});

test('mesh: floors take their height band, walls the band of their height a step darker', () => {
  const vertices = new Float32Array([0, 0, 0, 64, 0, 0, 64, 64, 256]);
  assert.deepEqual(zRange(vertices), [0, 256]);
  const palette = { low: [0.2, 0.2, 0.2] as const, high: [1, 0.5, 0.25] as const, wall: 0.5 };
  // One flat floor triangle at the top of the range and one vertical wall triangle.
  const surfaces = Float32Array.from([0, 0, 256, 10, 0, 256, 0, 10, 256, 0, 0, 0, 10, 0, 0, 0, 0, 256]);
  const colors = surfaceColors(surfaces, palette, [0, 256]);
  assert.equal(colors.length, surfaces.length);
  close(colors[0], srgbToLinear(1), 1e-6);
  close(colors[1], srgbToLinear(0.5), 1e-6);
  close(colors[9], srgbToLinear(0.2 * 0.5), 1e-6);
  assert.ok(colors[15] > colors[9]);
  assert.ok(mapPalette([0.05, 0.05, 0.06]).low[0] < mapPalette([0.98, 0.98, 0.98]).low[0]);
  assert.deepEqual(zRange(new Float32Array()), [0, 0]);
});

test('mesh: height bands step up with height and hold steady within a band', () => {
  assert.equal(heightBand(-50, 0, 600), 0);
  close(heightBand(900, 0, 600), 1);
  assert.ok(heightBand(300, 0, 600) > heightBand(100, 0, 600));
  const within = heightBand(5, 0, 600) - heightBand(1, 0, 600);
  const across = heightBand(20, 0, 600) - heightBand(15, 0, 600);
  assert.ok(across > within * 3, 'a band edge jumps more than the slope inside a band');
});

test('cut: opens just above the highest player and never leaves the mesh', () => {
  const frames: TacticalFrame[] = [{ tick: 0, samples: [sample(1, 0, 100), sample(2, 0, 300)] }];
  const cut = defaultCut({ minZ: -500, maxZ: 2000 }, frames);
  assert.ok(cut > 300 + 72);
  assert.ok(cut < 2000);
  assert.equal(defaultCut({ minZ: -500, maxZ: 400 }, frames), 400);
  assert.equal(defaultCut({ minZ: -500, maxZ: 2000 }, []), 2000);
});

test('frame: the play area wins over a collision mesh that reaches the skybox', () => {
  const mesh = { min: { x: -9000, y: -9000, z: -4000 }, max: { x: 9000, y: 9000, z: 4000 } };
  const frames: TacticalFrame[] = [{ tick: 0, samples: [sample(1, 0, -100), sample(2, 0, 200)] }];
  const framed = playBox(mesh, { min_x: -2000, min_y: -1500, max_x: 1800, max_y: 2500 }, frames);
  assert.deepEqual([framed.min.x, framed.max.y], [-2000, 2500]);
  assert.ok(framed.min.z >= -200 && framed.max.z <= 500);
  assert.equal(playBox(mesh, undefined).min.x, -9000);
});

test('trails: keep heights, stop at death and run oldest first', () => {
  const frames: TacticalFrame[] = [
    { tick: 0, samples: [sample(1, 0, 0), sample(2, 0, 0)] },
    { tick: 64, samples: [sample(1, 10, 16), sample(2, 0, 0, 0)] },
    { tick: 128, samples: [sample(1, 20, 32)] },
  ];
  const trails = sampleTrails3d(frames, { index: 2, alpha: 0 }, 64, 2);
  assert.deepEqual(trails.get(1)?.map((point) => point.z), [0, 16, 32]);
  assert.deepEqual(trails.get(2)?.length, 1);
  const short = sampleTrails3d(frames, { index: 2, alpha: 0 }, 64, 1);
  assert.deepEqual(short.get(1)?.map((point) => point.x), [10, 20]);
});
