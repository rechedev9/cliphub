// Unit tests for classifying a reel's failureReason into a typed result.
// Run: node --test failure-reason.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  DEMO_INCOMPATIBLE_PREFIX,
  FAILED_STRIP_LABEL,
  MISMATCH_REDRIVE_FAILURE_REASON,
  UNPLAYABLE_START_PREFIX,
  failedStripLabel,
  parseFailureReason,
} from './failure-reason.ts';

test('parseFailureReason classifies each reason into its kind and retry hint', () => {
  const cases: Array<{ name: string; reason: string; kind: string; retryCanHelp: boolean; message: RegExp }> = [
    {
      name: 'render mismatch latched by the reconcile loop breaker keeps its own message',
      reason: MISMATCH_REDRIVE_FAILURE_REASON,
      kind: 'render-mismatch',
      retryCanHelp: true,
      message: /no coincide con las jugadas/,
    },
    {
      name: 'a mismatch-looking raw string that is not the latched reason stays generic',
      reason: 'render does not match the intent',
      kind: 'generic',
      retryCanHelp: true,
      message: /No se pudo completar el vídeo/,
    },
  ];
  for (const { name, reason, kind, retryCanHelp, message } of cases) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, kind, name);
    assert.equal(result.retryCanHelp, retryCanHelp, name);
    assert.match(result.message, message, name);
    assert.equal(failedStripLabel(reason), FAILED_STRIP_LABEL.pipeline, name);
  }
});

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

test('playback-ended demo is demo-incompatible with its own message', () => {
  const reason =
    'demo_incompatible: cs2 cannot replay this demo to the end (playback stops before every protected segment completes); captured 3/16 segments before the failure';
  const result = parseFailureReason(reason);
  assert.equal(result.kind, 'demo-incompatible');
  assert.equal(result.retryCanHelp, false);
  assert.deepEqual(result.counts, { captured: 3, requested: 16 });
  assert.match(result.message, /La demo termina antes/);
  assert.doesNotMatch(result.message, /versión antigua de CS2/);
  assert.match(result.message, /Se capturaron 3 de 16 jugadas/);
});

test('a generic reason stays generic, retryable, and does not leak internals', () => {
  const reason = 'ffmpeg exited with code 1';
  const result = parseFailureReason(reason);
  assert.equal(result.kind, 'generic');
  assert.equal(result.retryCanHelp, true);
  assert.match(result.message, /No se pudo completar el vídeo/);
  assert.doesNotMatch(result.message, /ffmpeg/);
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

test('ordinary tool failures stay generic without surfacing raw tool output', () => {
  for (const reason of ['ffmpeg exited with code 1', 'compose failed', 'editor timed out']) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, 'generic', reason);
    assert.equal(result.retryCanHelp, true, reason);
    assert.match(result.message, /comparte el diagnóstico desde Ajustes/);
    assert.notEqual(result.message, reason);
  }
});

test('undefined and empty reasons fall back to a generic retryable message', () => {
  for (const reason of [undefined, '', '   ']) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, 'generic');
    assert.equal(result.retryCanHelp, true);
    assert.match(result.message, /No se pudo completar el vídeo/);
  }
});

test('observer-target failure explains how to regenerate without leaking the console path', () => {
  const reason =
    'recorder failed: capture POV verification failed: observer target remained unknown during seg-012; check C:\\cs2\\console.log';
  const result = parseFailureReason(reason, { fullDemo: true });
  assert.equal(result.kind, 'pov-verification');
  assert.equal(result.retryCanHelp, false);
  assert.match(result.message, /perdió el POV/);
  assert.match(result.message, /vuelve a preparar la demo/);
  assert.doesNotMatch(result.message, /console\.log|seg-012|C:\\/);
});

test('observer-target drift is a retryable capture flake, not a dead pipeline', () => {
  const reason =
    'recorder failed: capture POV verification failed: observer target 76561198307734468 drifted from 76561198386265483 during seg-001; check CS2 console.log';
  const result = parseFailureReason(reason, { fullDemo: true });
  assert.equal(result.kind, 'capture-flake');
  assert.equal(result.retryCanHelp, true);
  assert.match(result.message, /perdió el POV/);
  assert.match(result.message, /No es un error de pipeline/);
  assert.doesNotMatch(result.message, /console\.log|76561198307734468|seg-001/);
  assert.equal(failedStripLabel(reason, { fullDemo: true }), FAILED_STRIP_LABEL.capture);
});

test('observer mismatch before record-start is the same capture flake', () => {
  const reason =
    'recorder failed: capture POV verification failed: observer target 76561198000000001 does not match 76561198386265483 before record-start-seg-001';
  const result = parseFailureReason(reason);
  assert.equal(result.kind, 'capture-flake');
  assert.equal(result.retryCanHelp, true);
  assert.equal(failedStripLabel(reason), FAILED_STRIP_LABEL.capture);
});

test('generic failures keep the pipeline-dead strip label', () => {
  assert.equal(failedStripLabel('ffmpeg exited with code 1'), FAILED_STRIP_LABEL.pipeline);
});

test('other POV verification failures stay sanitized and retryable', () => {
  const reasons = [
    'capture POV verification failed: observer drifted before the protected kill',
    'capture POV verification failed: HLAE console stopped responding',
  ];

  for (const reason of reasons) {
    const result = parseFailureReason(reason);
    assert.equal(result.kind, 'generic', reason);
    assert.equal(result.retryCanHelp, true, reason);
    assert.match(result.message, /No se pudo completar el vídeo/);
    assert.doesNotMatch(result.message, /POV|HLAE|observer/);
  }
});

test('observer-target failure remains retryable outside Full Demo', () => {
  const reason =
    'recorder failed: capture POV verification failed: observer target remained unknown during seg-003';
  const result = parseFailureReason(reason);

  assert.equal(result.kind, 'generic');
  assert.equal(result.retryCanHelp, true);
  assert.doesNotMatch(result.message, /observer|seg-003/);
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
