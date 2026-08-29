import assert from 'node:assert/strict';
import test from 'node:test';
import { runSimulation, validateScenario } from './simulator.mjs';

const target = '76561198377256168';

function scenario(overrides = {}) {
  return {
    schema_version: 1,
    name: 'unit',
    target_steamid: target,
    start_tick: 10,
    max_frames: 100,
    expect: { outcome: 'verified', soft_quit: true },
    ...overrides,
  };
}

const markerScript = `
"use strict";
{
  const id = "zackvideo/generated-recorder";
  let frames = 0;
  mirv.events.clientFrameStageNotify.on(id, (event) => {
    if (event.isBefore) return;
    frames++;
    if (frames === 1) {
      mirv.message("[zackvideo] record-start-seg-001: mirv_streams record start");
      mirv.exec("mirv_streams record start");
    }
    if (frames === 2) {
      mirv.message("[zackvideo] record-end-seg-001: mirv_streams record end");
      mirv.exec("mirv_streams record end");
      mirv.message("ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1:test\\n");
    }
    if (frames === 3) mirv.exec("disconnect");
    if (frames === 5) mirv.exec("quit");
  });
  globalThis[id] = { unregister: () => mirv.events.clientFrameStageNotify.off(id) };
}
`;

test('runs JavaScript with deterministic MIRV events and delayed quit', async () => {
  const first = await runSimulation(markerScript, scenario());
  const second = await runSimulation(markerScript, scenario());
  assert.equal(first.ok, true);
  assert.equal(first.outcome, 'verified');
  assert.equal(first.disconnect_frame, 3);
  assert.equal(first.quit_frame, 5);
  assert.deepEqual(first, second);
});

test('rejects marker-only scripts that never execute balanced recording commands', async () => {
  const source = `
    mirv.events.clientFrameStageNotify.on("x", () => {
      mirv.message("[zackvideo] record-start-seg-fake: mirv_streams record start");
      mirv.message("[zackvideo] record-end-seg-fake: mirv_streams record end");
      mirv.message("ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1:test");
      mirv.exec("disconnect");
      mirv.exec("quit");
    });
  `;
  const result = await runSimulation(source, scenario({ expect: { outcome: 'verified' } }));
  assert.equal(result.ok, false);
  assert.ok(result.integrity_failures.some((failure) => failure.includes('do not match completed recordings')));
});

test('reports expectation mismatches instead of treating them as simulator crashes', async () => {
  const result = await runSimulation(markerScript, scenario({ expect: { outcome: 'failed' } }));
  assert.equal(result.ok, false);
  assert.match(result.expectation_failures[0], /outcome=verified/);
});

test('rejects malformed scenarios before executing generated script text', () => {
  assert.throws(() => validateScenario({ schema_version: 1 }), /scenario.name/);
  assert.throws(() => validateScenario({ schema_version: 99, name: 'x', target_steamid: target }), /schema_version/);
});

test('the VM limits accidental process access and dynamic code generation', async () => {
  const source = `
    if (typeof process !== "undefined") throw new Error("process leaked");
    let blocked = false;
    try { Function("return 1")(); } catch (_) { blocked = true; }
    if (!blocked) throw new Error("dynamic code generation leaked");
    mirv.events.clientFrameStageNotify.on("x", () => mirv.exec("quit"));
  `;
  const result = await runSimulation(source, scenario({ expect: { outcome: 'incomplete' } }));
  assert.equal(result.ok, true);
});
