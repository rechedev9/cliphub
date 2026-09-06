// node scripts/bench-tactical-replay.mjs
// Synthetic 60 Hz playback over immutable 8 Hz samples; not an Electron FPS test.
import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { createSampleTrailReader, isAlive } from '../lib/tactical-replay.ts';

const frames = Array.from({ length: 720 }, (_, i) => ({
  tick: i * 8,
  samples: Array.from({ length: 10 }, (_, slot) => ({
    slot, x: i * 2 + slot, y: slot * 100, z: 0, yaw: i % 360, health: 100, flags: 1,
  })),
}));
const cursors = Array.from({ length: 600 }, (_, i) => ({ index: 40 + Math.floor(i * 8 / 60), alpha: (i * 8 / 60) % 1 }));

function legacyTrails(cursor) {
  const trails = new Map();
  const oldestTick = frames[cursor.index].tick - 2 * 64;
  for (let i = cursor.index; i >= 0; i--) {
    if (frames[i].tick < oldestTick) break;
    for (const sample of frames[i].samples) {
      if (!isAlive(sample)) continue;
      const points = trails.get(sample.slot);
      if (points === undefined) trails.set(sample.slot, [{ x: sample.x, y: sample.y }]);
      else points.unshift({ x: sample.x, y: sample.y });
    }
  }
  return trails;
}

const read = createSampleTrailReader(frames, 64, 2);
for (const cursor of cursors) assert.deepEqual([...read(cursor)], [...legacyTrails(cursor)]);
let checksum = 0;
function measure(reader) {
  const start = performance.now();
  for (let repeat = 0; repeat < 100; repeat++) {
    for (const cursor of cursors) checksum += reader(cursor).size;
  }
  return performance.now() - start;
}
measure(legacyTrails);
measure(read);
const before = [];
const after = [];
for (let run = 0; run < 5; run++) {
  before.push(measure(legacyTrails));
  after.push(measure(read));
}
const median = (values) => values.toSorted((a, b) => a - b)[Math.floor(values.length / 2)];
console.log(JSON.stringify({
  scenario: 'tactical-trails-60hz', evaluations: 60_000,
  baseline_median_ms: median(before), candidate_median_ms: median(after),
  speedup: median(before) / median(after), checksum,
}, null, 2));
