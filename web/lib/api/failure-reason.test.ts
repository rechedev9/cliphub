// Unit tests for classifying a reel's failureReason into a typed result.
// Run: node --test failure-reason.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { DEMO_INCOMPATIBLE_PREFIX, UNPLAYABLE_START_PREFIX, parseFailureReason } from './failure-reason.ts';

test('demo-incompatible reason with a captured clause yields counts and no retry', () => {
  const reason =
    'demo_incompatible: cs2 cannot replay this demo (it was recorded on an older cs2 build); captured 1/16 segments before the failure';
  const result = parseFailureReason(reason);
  assert.equal(result.kind, 'demo-incompatible');
  assert.equal(result.retryCanHelp, false);
  assert.deepEqual(result.counts, { captured: 1, requested: 16 });
  assert.match(result.message, /versión antigua de CS2/);
  assert.match(result.message, /Se capturaron 1 de 16 jugadas/);
});

test('demo-incompatible reason without a captured clause has no counts', () => {
  const reason = 'demo_incompatible: cs2 cannot replay this demo (it was recorded on an older cs2 build)';
  const result = parseFailureReason(reason);
  assert.equal(result.kind, 'demo-incompatible');
  assert.equal(result.retryCanHelp, false);
  assert.equal(result.counts, undefined);
  assert.doesNotMatch(result.message, /Se capturaron/);
});

test('a generic reason stays generic and retryable', () => {
  const reason = 'ffmpeg exited with code 1';
  const result = parseFailureReason(reason);
  assert.equal(result.kind, 'generic');
  assert.equal(result.retryCanHelp, true);
  assert.equal(result.message, reason);
  assert.equal(result.counts, undefined);
});

test('a non-reusable capture reason is retryable with a Spanish re-record message', () => {
  for (const reason of [
    'recording result capture_mode must be "real"',
    'recording_not_reusable:recording result capture_mode must be "real"',
    'recording result lacks completed POV verification',
    'recording result capture input fingerprint does not match its plan',
    'legacy recording result contains fields from a newer capture contract',
    'recording result publication is pending',
  ]) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, 'recording-not-reusable', reason);
    assert.equal(result.retryCanHelp, true, reason);
    assert.match(result.message, /no es reutilizable/);
    assert.match(result.message, /volverá a grabar/i);
  }
});

test('ordinary tool failures stay generic and surface the raw English reason', () => {
  for (const reason of ['ffmpeg exited with code 1', 'compose failed', 'editor timed out']) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, 'generic', reason);
    assert.equal(result.retryCanHelp, true, reason);
    assert.equal(result.message, reason);
  }
});

test('undefined and empty reasons fall back to a generic retryable message', () => {
  for (const reason of [undefined, '', '   ']) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, 'generic');
    assert.equal(result.retryCanHelp, true);
    assert.equal(result.message, 'El reel falló en tu equipo.');
  }
});

test('the exported prefix is the exact orchestrator token', () => {
  assert.equal(DEMO_INCOMPATIBLE_PREFIX, 'demo_incompatible:');
  assert.equal(UNPLAYABLE_START_PREFIX, 'unplayable_start:');
});

test('unplayable-start is not retryable and tells the user not to relaunch CS2', () => {
  const result = parseFailureReason('unplayable_start: CS2 crashed rewinding playdemo to tick 0');
  assert.equal(result.kind, 'unplayable-start');
  assert.equal(result.retryCanHelp, false);
  assert.match(result.message, /No relances CS2/);
  assert.match(result.message, /tick 0/);
});
