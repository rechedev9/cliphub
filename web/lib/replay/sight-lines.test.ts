import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalSample } from '../api/tactical.ts';
import { buildSightBvh } from './sight-bvh.ts';
import { blockingSmokes, inFieldOfView, sightPairs, sightPlayer, viewWedge } from './sight-lines.ts';

const ALIVE = TACTICAL_SAMPLE_FLAGS.alive;
const T = TACTICAL_SAMPLE_FLAGS.sideT;

function sample(slot: number, options: { x: number; y: number; yaw: number; flags: number }): TacticalSample {
  return { slot, x: options.x, y: options.y, z: 0, yaw: options.yaw, health: 100, flags: ALIVE | options.flags };
}

/** One wall in the plane x = 0, y -100..100, z 0..200. */
const bvh = buildSightBvh(Float32Array.from([0, -100, 0, 0, 100, 0, 0, 100, 200, 0, -100, 200]), Uint32Array.from([0, 1, 2, 0, 2, 3]));

test('sight lines: enemies across the wall do not see each other, enemies past its end do', () => {
  const players = [
    sample(0, { x: -300, y: 0, yaw: 0, flags: 0 }),
    sample(1, { x: 300, y: 0, yaw: 180, flags: T }),
    sample(2, { x: 300, y: 400, yaw: 90, flags: T }),
  ].map(sightPlayer);
  const pairs = sightPairs({ bvh, players, smokes: [] });
  assert.deepEqual(pairs, [{ a: 0, b: 2, aSees: true, bSees: false }]);
});

test('sight lines: teammates are never paired and a smoke cuts the line', () => {
  const players = [sample(0, { x: -300, y: 400, yaw: 0, flags: 0 }), sample(1, { x: 300, y: 400, yaw: 180, flags: T }), sample(2, { x: 300, y: 500, yaw: 0, flags: T })].map(sightPlayer);
  assert.equal(sightPairs({ bvh, players, smokes: [] }).length, 2);
  assert.deepEqual(sightPairs({ bvh, players, smokes: [{ x: 0, y: 400, z: 0 }, { x: 0, y: 450, z: 0 }] }), []);
});

test('sight lines: the field of view is about 106 degrees wide, and narrower scoped', () => {
  const viewer = sightPlayer(sample(0, { x: 0, y: 0, yaw: 0, flags: 0 }));
  const at = (degrees: number): { x: number; y: number; z: number } => ({ x: Math.cos((degrees * Math.PI) / 180) * 500, y: Math.sin((degrees * Math.PI) / 180) * 500, z: 40 });
  assert.equal(inFieldOfView(viewer, at(50)), true);
  assert.equal(inFieldOfView(viewer, at(-50)), true);
  assert.equal(inFieldOfView(viewer, at(60)), false);
  const scoped = sightPlayer(sample(0, { x: 0, y: 0, yaw: 0, flags: TACTICAL_SAMPLE_FLAGS.scoped }));
  assert.equal(inFieldOfView(scoped, at(20)), true);
  assert.equal(inFieldOfView(scoped, at(30)), false);
  const wrapped = sightPlayer(sample(0, { x: 0, y: 0, yaw: 350, flags: 0 }));
  assert.equal(inFieldOfView(wrapped, at(20)), true);
});

test('sight lines: the view wedge stops at the wall', () => {
  const player = sightPlayer(sample(0, { x: -300, y: 0, yaw: 0, flags: 0 }));
  const wedge = viewWedge({ bvh, player, reach: 1000, rays: 20 });
  assert.equal(wedge.length, 21);
  const middle = wedge[10];
  assert.ok(Math.abs(middle.x) < 1e-3, `middle ray ends at x ${middle.x}`);
  const edge = wedge[0];
  assert.ok(Math.hypot(edge.x + 300, edge.y) > 999, 'the outer ray passes the wall and runs to full reach');
});

test('sight lines: a smoke stops sight until it thins, and ends early when the demo says so', () => {
  const secondsOf = (tick: number): number => tick / 64;
  const smoke = { tick: 0, kind: 'smoke' as const, pos: [10, 20, 0] as [number, number, number], target_pos: [0, 0, 0] as [number, number, number] };
  assert.equal(blockingSmokes({ events: [smoke], nowSeconds: 15, secondsOf }).length, 1);
  assert.equal(blockingSmokes({ events: [smoke], nowSeconds: 21, secondsOf }).length, 0);
  const cleared = { ...smoke, end_tick: 8 * 64 };
  assert.equal(blockingSmokes({ events: [cleared], nowSeconds: 5, secondsOf }).length, 1);
  assert.equal(blockingSmokes({ events: [cleared], nowSeconds: 7, secondsOf }).length, 0);
});
