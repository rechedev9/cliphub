import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../../api/tactical.ts';
import { actionFocus, actionLevel, easeInOut, focusDistance, liveCut, smoothDamp, standingFeet } from './action-camera.ts';

const alive = TACTICAL_SAMPLE_FLAGS.alive;
const at = (x: number, y: number, z = 0, flags = alive) => ({ x, y, z, flags });

test('action camera: frames the living players and ignores the dead and a far lurker', () => {
  const focus = actionFocus([at(0, 0), at(200, 0), at(100, 300), at(5000, 5000), at(-900, -900, 0, 0)]);
  assert.ok(focus !== null);
  assert.equal(focus.centre.x, 100);
  assert.equal(focus.centre.y, 100);
  assert.ok(focus.radius >= 450);
  assert.equal(actionFocus([at(0, 0, 0, 0)]), null);
});

test('action camera: a wider spread pulls the camera back', () => {
  assert.ok(focusDistance(2000, 50) > focusDistance(500, 50));
});

test('action camera: the cut sits over the highest living player of the action, inside the mesh', () => {
  assert.equal(liveCut({ minZ: -500, maxZ: 2000 }, [at(0, 0, 100), at(0, 0, 900, 0), at(0, 0, -50), at(5000, 5000, 800)]), 100 + 72 + 60);
  assert.equal(liveCut({ minZ: -500, maxZ: 200 }, [at(0, 0, 100)]), 200);
  assert.equal(liveCut({ minZ: -500, maxZ: 2000 }, []), 2000);
});

test('action camera: the glide easing starts and ends where it should', () => {
  assert.equal(easeInOut(0), 0);
  assert.equal(easeInOut(1), 1);
  assert.equal(easeInOut(0.5), 0.5);
});

test('action camera: the spring settles on the target without overshooting', () => {
  let state = { value: 0, velocity: 0 };
  const seen: number[] = [];
  for (let i = 0; i < 300; i += 1) {
    state = smoothDamp(state, 1000, 0.9, 1 / 60);
    seen.push(state.value);
  }
  assert.ok(seen.every((value, i) => value <= 1000 && (i === 0 || value >= seen[i - 1])));
  assert.ok(seen[5] < 100, 'starts gently');
  assert.ok(1000 - state.value < 10, `settles, at ${state.value}`);
  assert.deepEqual(smoothDamp({ value: 3, velocity: 2 }, 10, 1, 0), { value: 3, velocity: 2 });
});

test('action camera: the level follows the followed player, else the median of the action group', () => {
  const slotted = [
    { ...at(0, 0, -700), slot: 1 },
    { ...at(100, 0, -690), slot: 2 },
    { ...at(200, 0, -400), slot: 3 },
    { ...at(9000, 9000, 500), slot: 4 },
    { ...at(0, 50, -400, 0), slot: 5 },
  ];
  assert.equal(actionLevel(slotted, null), -690, 'the far lurker and the dead are left out');
  assert.equal(actionLevel(slotted, 3), -400);
  assert.equal(actionLevel(slotted, 5), -690, 'a dead followed player falls back to the group');
  assert.equal(actionLevel([], null), null);
});

test('action camera: only the living stand on the map, capped', () => {
  const feet = standingFeet([at(1, 2, 3), at(4, 5, 6, 0), at(7, 8, 9), at(10, 11, 12)], 2);
  assert.deepEqual(feet, [{ x: 1, y: 2, z: 3 }, { x: 7, y: 8, z: 9 }]);
});
